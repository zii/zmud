package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode"

	"zmud/ansi"
	"zmud/api"
	"zmud/lib"

	//"github.com/peterh/liner"
	"zmud/lib/liner"

	"golang.org/x/term"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// 服务器一次性响应的文本
type batch struct {
	text string
	ln   int // Translate后实际输出的行数
}

// MUD 客户端，管理连接、输入输出和历史命令
type Client struct {
	conn        net.Conn        // TCP 连接，与 MUD 服务器的通信管道
	exit        chan struct{}   // 退出信号通道，服务器断开时触发
	once        sync.Once       // 确保退出通道只关闭一次
	tr          *lib.Translator // 翻译器，将服务器消息翻译为中文
	server      *lib.Server     // 当前连接的服务器
	ring        [10]string      // 服务器原始文本流(用于调试)
	ri          int             // 最新一条原始文本
	lang        lib.Lang        // 语言显示模式: LSRC=原文, lib.LTRN=译文, LMIX=双语
	liner       *liner.State    // 行编辑器，支持历史和编辑
	historyFile string          // 历史记录文件路径
	cmdHistory  map[string]int  // 命令使用次数，用于补全排序
	batchs      []*batch        // 服务器最近响应历史
}

// 创建新的客户端实例，初始化所有通道和默认值
// cfg: 翻译配置，不能为 nil
// server: 当前连接的服务器
func NewClient(cfg *lib.Config, server *lib.Server) *Client {
	home, _ := os.UserHomeDir()
	f := filepath.Join(home, ".zmud", "history")
	os.MkdirAll(filepath.Dir(f), 0700)
	return &Client{
		exit:        make(chan struct{}),
		tr:          lib.NewTranslator(cfg),
		server:      server,
		lang:        lib.LMIX, // 默认双语
		liner:       liner.NewLiner(),
		historyFile: f,
		cmdHistory:  make(map[string]int),
	}
}

// 发送退出信号，确保只关闭一次
func (c *Client) quit() {
	c.once.Do(func() { close(c.exit) })
}

// 连接到指定服务器地址，建立 TCP 连接
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.server.Host+":"+c.server.Port)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// 根据配置获取解码器，charset 为空则返回 nil（不转换）
func (c *Client) getDecoder() transform.Transformer {
	charset := strings.ToLower(c.server.Charset)
	if charset == "gb" || charset == "gbk" {
		return simplifiedchinese.GBK.NewDecoder()
	}
	if charset == "big5" {
		return traditionalchinese.Big5.NewDecoder()
	}
	return nil
}

// 根据配置获取编码器，charset 为空则返回 nil（不转换）
func (c *Client) getEncoder() transform.Transformer {
	charset := strings.ToLower(c.server.Charset)
	if charset == "gb" || charset == "gbk" {
		return simplifiedchinese.GBK.NewEncoder()
	}
	if charset == "big5" {
		return traditionalchinese.Big5.NewEncoder()
	}
	return nil
}

// 执行系统命令, 返回
func (c *Client) doSystemCmd(input string) {
	if input == "/r" {
		i := (c.ri - 0 + len(c.ring)) % len(c.ring)
		fmt.Printf("%q\n", c.ring[i])
	} else if m, ok := strings.CutPrefix(input, "/r "); ok {
		d, _ := strconv.Atoi(m)
		i := (c.ri - d + len(c.ring)) % len(c.ring)
		fmt.Printf("%q\n", c.ring[i])
	} else if input == "/e" {
		fmt.Printf("当前引擎: %s\n", c.tr.GetEngine())
	} else if m, ok := strings.CutPrefix(input, "/e "); ok {
		switch m {
		case "baidu", "deepseek", "kilo", "google":
			c.tr.SetEngine(m)
			fmt.Printf("更换引擎 %s 成功!\n", m)
		case "update":
			lib.InputEngine(c.tr.Config)
		default:
			fmt.Println("切换引擎: baidu, deepseek, kilo, google")
			fmt.Println("/e update - 更新引擎参数")
		}
	} else if m, ok := strings.CutPrefix(input, "/ask "); ok {
		fmt.Println("正在询问 AI...")
		prompt := fmt.Sprintf("我在玩[%s] Mud. %s", c.server.Name, m)
		ans, err := c.tr.Ask(prompt)
		if err != nil {
			fmt.Printf("提问失败: %v\n", err)
		} else {
			fmt.Printf("AI: %s\n", ans)
		}
	} else if input == "/hint" {
		// 获取最近3条batch
		n := len(c.batchs)
		start := 0
		if n > 3 {
			start = n - 3
		}
		var context string
		for i := start; i < n; i++ {
			context += c.batchs[i].text
		}
		prompt := fmt.Sprintf("我在玩[%s] Mud，当前屏幕显示内容如下：\n%s\n请问下一步应该输入什么命令？", c.server.Name, context)
		ans, err := c.tr.Ask(prompt)
		if err != nil {
			fmt.Printf("获取提示失败: %v\n", err)
		} else {
			fmt.Printf("建议: %s\n", ans)
		}
	} else if input == "/quit" {
		fmt.Println("退出游戏")
		c.quit()
	}
	return
}

// 设置语言模式
func (c *Client) setLang(n lib.Lang) {
	newl := lib.Lang(n)
	if c.lang != newl {
		c.lang = newl
		c.redraw()
	}
}

// 补全函数：系统命令 + 历史命令 Tab 补全
func (c *Client) completer(line string) []string {
	// 系统命令
	commands := []string{
		"/e", "/e baidu", "/e deepseek", "/e kilo", "/e update", "/r", "/ask", "/hint",
	}
	var results []string
	seen := make(map[string]bool)

	// 先添加系统命令匹配
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, line) {
			results = append(results, cmd)
			seen[cmd] = true
		}
	}
	// 添加历史命令匹配，按使用次数排序
	type pair struct {
		cmd   string
		count int
	}
	var pairs []pair
	for cmd, count := range c.cmdHistory {
		if strings.HasPrefix(cmd, line) {
			pairs = append(pairs, pair{cmd, count})
		}
	}
	// 按使用次数降序排序
	for i := range pairs {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].count > pairs[i].count {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	for _, p := range pairs {
		if !seen[p.cmd] {
			results = append(results, p.cmd)
			seen[p.cmd] = true
		}
	}
	return results
}

// 从 stdin 读取输入行并发送到服务器
func (c *Client) readInput() {
	c.liner.SetCtrlCAborts(true)
	c.liner.SetCompleter(func(line string) []string { return c.completer(line) })
	c.liner.SetKeyBinding("f1", func(s *liner.State) {
		if c.lang == lib.LSRC {
			c.setLang(lib.LTRN)
		} else {
			c.setLang(lib.LSRC)
		}
	})
	c.liner.SetKeyBinding("f2", func(s *liner.State) { c.setLang(lib.LMIX) })

	// 加载历史记录
	if f, err := os.Open(c.historyFile); err == nil {
		c.liner.ReadHistory(f)
		f.Close()
	}

	for {
		input, err := c.liner.Prompt("❯ ")
		if err != nil { // EOF 或 Ctrl+C
			break
		}
		// 跳过空行
		if strings.TrimSpace(input) != "" {
			// 添加到历史
			c.liner.AppendHistory(input)
			// 添加到历史（过滤短命令）
			if len(input) > 2 {
				c.cmdHistory[input]++
			}
			// 处理命令
			if strings.HasPrefix(input, "/") {
				c.doSystemCmd(input)
				continue
			}
			// 发送到服务器
			if encoder := c.getEncoder(); encoder != nil {
				out, _, err := transform.Bytes(encoder, []byte(input+"\r\n"))
				if err == nil {
					c.conn.Write(out)
				} else {
					fmt.Fprint(c.conn, input, "\r\n")
				}
			} else {
				fmt.Fprint(c.conn, input, "\r\n")
			}
		}
	}

	// 保存历史记录
	if f, err := os.Create(c.historyFile); err == nil {
		c.liner.WriteHistory(f)
		f.Close()
	}
	c.quit()
}

// 运行客户端主循环，启动读写 goroutine 并使用 scanner 读取用户输入
func (c *Client) Run() {
	defer c.liner.Close()

	go c.readServer()
	go c.readInput()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sig)

	for {
		select {
		case <-sig:
			c.quit()
			return
		case <-c.exit:
			fmt.Println("\r")
			return
		}
	}
}

// 此行文本是否不需要翻译
// 字母开头, 帮助信息*/-开头, 英文超过一半
func (c *Client) dontTranslate(pure string) bool {
	if len(pure) > 0 && unicode.IsLetter(rune(pure[0])) ||
		strings.HasPrefix(pure, "* ") || strings.HasPrefix(pure, "- ") ||
		lib.IsEnglishDominant(pure) {
		return false
	}
	return true
}

// 当服务器返回超多条帮助信息时, 逐条翻译太慢, 需要先批量翻译进行预热
func (c *Client) warmup(lines []string) {
	if c.lang == lib.LSRC || len(lines) <= 5 || c.tr.GetEngine() == "" {
		return
	}
	var srcs []string
	for i, line := range lines {
		if line == "" {
			continue
		}
		lang := c.lang
		// 去掉前后的颜色码+空格, 翻译的时候再拼接上
		_, _, _, pure := lib.StripANSI(line)
		// 提示符不翻译
		if strings.HasSuffix(pure, ">") && i >= len(lines)-1 {
			lang = lib.LSRC
		} else if c.dontTranslate(pure) {
			lang = lib.LSRC
		}

		if lang != lib.LSRC {
			srcs = append(srcs, pure)
		}
		if len(srcs) == api.MaxBatchSize {
			_, err := c.tr.TranslateBatch(srcs)
			if err != nil {
				fmt.Println("\x1b[31m" + "\n批量翻译失败: " + err.Error() + "\x1b[0m")
			}
			srcs = srcs[:0]
		}
	}
	_, err := c.tr.TranslateBatch(srcs)
	if err != nil {
		fmt.Println("\x1b[31m" + "\n批量翻译失败: " + err.Error() + "\x1b[0m")
	}
}

// 翻译一段文本, 返回输出的实际行数
func (c *Client) Translate(text string) int {
	lines := strings.Split(text, "\n")
	go c.warmup(lines)
	outn := len(lines)
	preColor := ""
	for i, line := range lines {
		if line == "" {
			fmt.Println()
			continue
		}
		lang := c.lang
		if c.tr.GetEngine() == "" {
			lang = lib.LSRC
		}
		// 去掉前后的颜色码+空格, 翻译的时候再拼接上
		pre, indent, suf, pure := lib.StripANSI(line)
		// 提示符不翻译
		if strings.HasSuffix(pure, ">") && i >= len(lines)-1 {
			lang = lib.LSRC
		} else if c.dontTranslate(pure) {
			lang = lib.LSRC
		}
		fmt.Print(pre)

		// lang=LSRC/LMIX: 显示原文
		if lang != lib.LTRN {
			fmt.Print(indent + pure)
		}

		// 这些情况启用翻译: 字母开头, 帮助信息*/-开头, 英文超过一半
		if lang != lib.LSRC {
			if pre != "" {
				preColor = pre
			}
			if suf == ansi.Reset || pre == ansi.Reset {
				preColor = ""
			}
			s, err := c.tr.Translate(pure)
			if err == nil {
				// lang=lib.LTRN: 只显示译文, lang=LMIX: 双语
				if lang == lib.LTRN {
					fmt.Print(indent, s)
				} else {
					indent = lib.RemoveRN(indent)
					fmt.Print("\n", indent, ansi.Straw, s, ansi.Reset, preColor)
					outn++
				}
			} else {
				fmt.Println("\x1b[31m" + "\n翻译失败: " + err.Error() + "\x1b[0m")
				fmt.Println("\x1b[31m" + "\n" + pure + "\x1b[0m")
				outn += 2
			}
		}
		fmt.Print(suf)
		// 提示输入的时候不需要换行
		if i < len(lines)-1 {
			fmt.Println()
		}
	}
	return outn
}

// 重绘最近batch
func (c *Client) redraw() {
	// 清屏
	fmt.Print("\x1b[2J\x1b[H")
	// 重新输出
	for _, b := range c.batchs {
		ln := c.Translate(b.text)
		b.ln = ln
	}
}

// 从服务器读取消息，过滤 IAC 后输出，断开时触发退出
func (c *Client) readServer() {
	var tmp string
	buf := make([]byte, 4096)
	// 根据配置选择解码器
	decoder := c.getDecoder()
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			c.quit()
			return
		}
		data := lib.FilterIAC(buf[:n])
		text := string(data)
		// 如果配置了 charset，则转换编码
		if decoder != nil {
			out, _, err := transform.Bytes(decoder, data)
			if err == nil {
				text = string(out)
			}
		}
		// 服务器会返回各式各样的换行, 比如"\r\n", "\n\r", "\r\r\n\n", "\r\n\r\n", 要把\r删了统一成\n
		text = lib.RemoveCr(text)
		// 处理拼接: 如果行尾以小写字母结尾, 就暂存
		if !strings.HasSuffix(text, "[0m") && len(text) > 0 && unicode.IsLower(rune(text[len(text)-1])) {
			tmp += text
			continue
		}
		text = tmp + text
		tmp = ""
		c.ri = (c.ri + 1) % len(c.ring)
		c.ring[c.ri] = text
		text = lib.CleanWrap(text)
		ln := c.Translate(text)

		// 添加并截断batch历史
		b := &batch{text, ln}
		c.batchs = append(c.batchs, b)
		total := 0
		for _, b := range c.batchs {
			total += b.ln
		}
		_, height, _ := term.GetSize(int(os.Stdout.Fd()))
		if height <= 0 {
			height = 24
		}
		if total > height && len(c.batchs) > 1 {
			c.batchs = c.batchs[1:]
		}
	}
}
