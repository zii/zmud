package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"zmud/ansi"
	"zmud/api"
	"zmud/lib"
	"zmud/lib/lmdb"

	//"github.com/peterh/liner"
	"zmud/lib/liner"

	"github.com/tidwall/match"
	"golang.org/x/term"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// 服务器一次性响应的文本
type batch struct {
	lines []string
	ln    int // Translate后实际输出的行数
}

// MUD 客户端，管理连接、输入输出和历史命令
type Client struct {
	conn        net.Conn          // TCP 连接，与 MUD 服务器的通信管道
	exit        chan struct{}     // 退出信号通道，服务器断开时触发
	once        sync.Once         // 确保退出通道只关闭一次
	tr          *lib.Translator   // 翻译器，将服务器消息翻译为中文
	server      *lib.Server       // 当前连接的服务器
	ring        [10]string        // 服务器原始文本流(用于调试翻译)
	ri          int               // 最新一条原始文本
	mode        lib.Mode          // 显示模式: LSRC=原文, lib.LTRN=译文, LMIX=双语
	liner       *liner.State      // 行编辑器，支持历史和编辑
	historyFile string            // 历史记录文件路径
	cmdHistory  map[string]int    // 命令使用次数，用于补全排序
	batchs      []*batch          // 服务器最近响应历史
	wc          chan string       // 命令管道，后台发送goroutine从此读取
	rc          chan string       // 读取的命令管道
	script      *lib.Script       // 当前运行的脚本
	db          *lmdb.DB          // 别名数据库
	triggers    map[string]string // 触发器缓存（包括 SKIP）
	muTrigger   sync.Mutex
	aliases     map[string]string // 别名缓存，写操作受 muAlias 保护
	muAlias     sync.RWMutex
	encoder     transform.Transformer // 编码器，缓存以提升性能
	scriptPend  bool                  // 脚本中断待确认
	pendAt      time.Time             // pending 开始时间
}

// 创建新的客户端实例，初始化所有通道和默认值
// cfg: 翻译配置，不能为 nil
// server: 当前连接的服务器
func NewClient(cfg *lib.Config, server *lib.Server, mode lib.Mode) (*Client, error) {
	home, _ := os.UserHomeDir()
	f := filepath.Join(home, ".zmud", "history")
	os.MkdirAll(filepath.Dir(f), 0700)
	dbPath := filepath.Join(home, ".zmud", server.Host+":"+server.Port+".db")
	db, err := lmdb.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败 %s: %v", dbPath, err)
	}
	c := &Client{
		exit:        make(chan struct{}),
		tr:          lib.NewTranslator(cfg),
		server:      server,
		mode:        mode,
		historyFile: f,
		cmdHistory:  make(map[string]int),
		wc:          make(chan string, 10),
		rc:          make(chan string, 10),
		db:          db,
		triggers:    make(map[string]string),
	}
	c.loadTriggers()
	c.loadAliases()
	c.encoder = c.initEncoder()
	return c, nil
}

// 发送退出信号确保只关闭一次
func (c *Client) quit() {
	c.once.Do(func() { close(c.exit) })
}

// 连接到指定服务器地址，建立 TCP 连接
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.server.Host+":"+c.server.Port)
	if err != nil {
		return err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// 强制立即发送，不等待缓冲区
		tcpConn.SetNoDelay(true)
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

// 根据配置初始化编码器，charset 为空则返回 nil（不转换）
func (c *Client) initEncoder() transform.Transformer {
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
			for _, line := range c.batchs[i].lines {
				context += line + "\n"
			}
		}
		prompt := fmt.Sprintf("我在玩[%s] Mud，当前屏幕显示内容如下：\n%s\n请问下一步应该输入什么命令？", c.server.Name, context)
		ans, err := c.tr.Ask(prompt)
		if err != nil {
			fmt.Printf("获取提示失败: %v\n", err)
		} else {
			fmt.Printf("建议: %s\n", ans)
		}
	} else if m, ok := strings.CutPrefix(input, "/run "); ok {
		c.script = lib.NewScript(c.wc, c.aliases)
		go c.script.Run(m)
	} else if input == "/stop" {
		if c.script != nil {
			c.script.Stop()
			c.script = nil
			fmt.Println("脚本已停止")
		}
	} else if input == "/debug" {
		lib.DEBUG = !lib.DEBUG
		fmt.Printf("调试模式: %v\n", lib.DEBUG)
	} else if input == "/alias" {
		var n int
		c.db.View(func(tx *lmdb.Tx) error {
			tx.AscendKeys("alias:*", func(key, value string) bool {
				name := key[6:]
				fmt.Printf("/alias %s %s\n", name, value)
				n++
				return true
			})
			return nil
		})
		if n == 0 {
			fmt.Println("暂无别名")
		}
	} else if m, ok := strings.CutPrefix(input, "/alias "); ok {
		parts := strings.SplitN(m, " ", 2)
		name := parts[0]
		key := "alias:" + name
		if len(parts) == 1 {
			var val string
			c.db.View(func(tx *lmdb.Tx) error {
				val, _ = tx.Get(key)
				return nil
			})
			if val != "" {
				fmt.Printf("/alias %s %s\n", name, val)
			} else {
				fmt.Println("别名不存在:", name)
			}
		} else if parts[1] == "DELETE" {
			c.db.Update(func(tx *lmdb.Tx) error {
				tx.Delete(key)
				return nil
			})
			c.muAlias.Lock()
			delete(c.aliases, name)
			c.muAlias.Unlock()
			fmt.Println("别名已删除:", name)
		} else {
			c.db.Update(func(tx *lmdb.Tx) error {
				tx.Set(key, parts[1], nil)
				return nil
			})
			c.muAlias.Lock()
			c.aliases[name] = parts[1]
			c.muAlias.Unlock()
			fmt.Println("别名已设置:", name)
		}
	} else if input == "/trigger" {
		var n int
		c.db.View(func(tx *lmdb.Tx) error {
			tx.AscendKeys("trigger:*", func(key, value string) bool {
				pattern := key[8:]
				fmt.Printf("  %s -> %s\n", pattern, value)
				n++
				return true
			})
			return nil
		})
		if n == 0 {
			fmt.Println("暂无触发器")
		}
	} else if m, ok := strings.CutPrefix(input, "/trigger "); ok {
		var pattern, command string
		if strings.HasPrefix(m, `"`) {
			// 带引号的 pattern
			idx := strings.Index(m[1:], `"`)
			if idx == -1 {
				fmt.Println("触发器格式错误，需要引号闭合")
				return
			}
			pattern = m[1 : idx+1]
			if idx+2 < len(m) {
				command = m[idx+2:]
			}
		} else {
			// 不带引号，用第一个空格分隔
			parts := strings.SplitN(m, " ", 2)
			pattern = parts[0]
			if len(parts) > 1 {
				command = parts[1]
			}
		}
		command = strings.TrimSpace(command)
		key := "trigger:" + pattern
		if command == "" {
			// 查询或删除
			var val string
			c.db.View(func(tx *lmdb.Tx) error {
				val, _ = tx.Get(key)
				return nil
			})
			if val != "" {
				fmt.Printf("  %s -> %s\n", pattern, val)
			} else {
				fmt.Println("触发器不存在:", pattern)
			}
		} else if command == "DELETE" {
			c.db.Update(func(tx *lmdb.Tx) error {
				tx.Delete(key)
				return nil
			})
			fmt.Println("触发器已删除:", pattern)
			c.muTrigger.Lock()
			delete(c.triggers, pattern)
			c.muTrigger.Unlock()
		} else {
			c.db.Update(func(tx *lmdb.Tx) error {
				tx.Set(key, command, nil)
				return nil
			})
			c.muTrigger.Lock()
			c.triggers[pattern] = command
			c.muTrigger.Unlock()
			fmt.Println("触发器已设置:", pattern)
		}
	} else if m, ok := strings.CutPrefix(input, "/back "); ok {
		rev, err := lib.ReversePath(m)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(rev)
			c.rc <- rev
		}
	} else if input == "/quit" {
		fmt.Println("退出游戏")
		c.quit()
	}
	return
}

// 设置语言模式
func (c *Client) setMode(n lib.Mode) {
	newl := lib.Mode(n)
	if c.mode != newl {
		c.mode = newl
		c.redraw()
	}
}

// 补全函数：系统命令 + 历史命令 Tab 补全
func (c *Client) completer(line string) []string {
	// 系统命令
	commands := []string{
		"/e", "/r", "/ask", "/hint", "/alias", "/trigger", "/back",
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
	// 按使用次数降序排序（稳定排序：次数相同保持插入顺序）
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})
	for _, p := range pairs {
		if !seen[p.cmd] {
			results = append(results, p.cmd)
			seen[p.cmd] = true
		}
	}
	return results
}

// 发送单命令到服务器（实际写入）
func (c *Client) sendImpl(cmd string) {
	if c.conn == nil {
		return
	}
	if c.encoder != nil {
		out, _, err := transform.Bytes(c.encoder, []byte(cmd+"\r\n"))
		if err == nil {
			c.conn.Write(out)
			return
		}
	}
	fmt.Fprint(c.conn, cmd, "\r\n")
}

// 发送命令到管道，由后台goroutine实际发送
func (c *Client) send(cmd string) {
	select {
	case c.wc <- cmd:
	case <-c.exit:
	}
}

// 从 stdin 读取输入行并发送到服务器
func (c *Client) readInput() {
	c.liner.SetCtrlCAborts(true)
	c.liner.SetCompleter(func(line string) []string { return c.completer(line) })
	c.liner.SetKeyBinding("f1", func(s *liner.State) {
		if c.mode == lib.LSRC {
			c.setMode(lib.LTRN)
		} else {
			c.setMode(lib.LSRC)
		}
	})
	c.liner.SetKeyBinding("f2", func(s *liner.State) { c.setMode(lib.LMIX) })

	// 加载历史记录
	if f, err := os.Open(c.historyFile); err == nil {
		c.liner.ReadHistory(f)
		f.Close()
	}

	for c.liner != nil {
		input, err := c.liner.Prompt("❯ ")
		if err != nil { // EOF 或 Ctrl+C
			close(c.rc)
			break
		}
		input = strings.TrimSpace(input)
		// 中断确认：返回 true 表示跳过此输入
		if c.handleScriptInterrupt(input) {
			continue
		}
		// 添加到历史
		// Prompt无限循环, 完全抢占了锁, 所以AppendHistory如果放在inputLoop会永远阻塞. 造成输入后屏幕不渲染.
		if input != "" {
			c.liner.AppendHistory(input)
		}
		c.rc <- input
	}

	// 保存历史记录
	if f, err := os.Create(c.historyFile); err == nil {
		c.liner.WriteHistory(f)
		f.Close()
	}
	close(c.wc) // 关闭命令管道，通知 sendLoop 退出
	c.quit()
}

// handleScriptInterrupt 处理脚本中断确认逻辑
// 返回 true 表示应跳过此次输入（pending 状态），false 表示正常处理
func (c *Client) handleScriptInterrupt(input string) bool {
	// 系统命令直接放行
	if strings.HasPrefix(input, "/") {
		return false
	}
	if c.script == nil || !c.script.Running() {
		return false
	}

	now := time.Now()
	if c.scriptPend {
		if now.Sub(c.pendAt) > 3*time.Second {
			// 超时自动取消
			c.scriptPend = false
			return false
		}
		// 确认中断
		c.script.Stop()
		c.script = nil
		c.scriptPend = false
		fmt.Println("(中断了当前脚本)")
		return true
	}

	// 首次输入：进入 pending
	c.scriptPend = true
	c.pendAt = now
	fmt.Println("(脚本运行中，确认中断?)")
	return true
}

// 从管道读取命令并发送到服务器
func (c *Client) sendLoop() {
	for cmd := range c.wc {
		c.sendImpl(cmd)
	}
}

// 处理输入循环，从 rc channel 读取并处理命令
func (c *Client) inputLoop() {
	for input := range c.rc {
		// 添加到历史（过滤短命令）
		if len(input) > 2 {
			c.cmdHistory[input]++
		}
		// 处理命令
		if strings.HasPrefix(input, "/") {
			c.doSystemCmd(input)
			continue
		}
		// 检查别名（支持 $A1-$A9 位置参数）
		if len(c.aliases) > 0 && input != "" {
			if expanded, ok := lib.ExpandAlias(c.aliases, input); ok {
				fmt.Printf("❯ %s -> %s\n", input, expanded)
				input = expanded
			}
		}
		// 发送到服务器
		if c.script != nil {
			c.script.Stop()
			if c.script.Running() {
				fmt.Println("(中断了当前脚本)")
			}
			c.script = nil
		}
		if input == "" {
			c.send("")
		} else {
			c.script = lib.NewScript(c.wc, c.aliases)
			go c.script.Run(input)
		}
	}
}

// 运行客户端主循环，启动读写 goroutine 并使用 scanner 读取用户输入
func (c *Client) Run() {
	defer func() {
		c.liner.Close()
		if r := recover(); r != nil {
			fmt.Printf("panic: %v\n", r)
			debug.PrintStack()
		}
	}()

	c.liner = liner.NewLiner()
	go c.readServer()
	go c.sendLoop()
	go c.readInput()
	go c.inputLoop()

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
	if c.mode == lib.LSRC || len(lines) <= 5 || c.tr.GetEngine() == "" {
		return
	}
	var srcs []string
	for i, line := range lines {
		if line == "" {
			continue
		}
		lang := c.mode
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
func (c *Client) Translate(lines []string) int {
	go c.warmup(lines)
	outn := len(lines)
	preColor := ""
	for i, line := range lines {
		if line == "" {
			fmt.Println()
			continue
		}
		lang := c.mode
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
		ln := c.Translate(b.lines)
		b.ln = ln
	}
}

// 从服务器读取消息，过滤 IAC 后输出，断开时触发退出
func (c *Client) readServer() {
	var tmp string
	buf := make([]byte, 8192)
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
		if c.mode != lib.LSRC && !strings.HasSuffix(text, "[0m") && len(text) > 0 && unicode.IsLower(rune(text[len(text)-1])) {
			tmp += text
			continue
		}
		text = tmp + text
		tmp = ""
		c.ri = (c.ri + 1) % len(c.ring)
		c.ring[c.ri] = text
		// 中文游戏不处理折行
		if c.mode != lib.LSRC {
			text = lib.CleanWrap(text)
		}
		lines, pures := c.checkSkip(text)
		var rn int // 渲染出来的行数
		if c.mode != lib.LSRC {
			rn = c.Translate(lines)
		} else {
			// 中文游戏直接打印
			for i, line := range lines {
				if i > 0 {
					fmt.Println()
				}
				fmt.Print(line)
			}
		}

		// 添加并截断batch历史
		b := &batch{lines, rn}
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

		puretext := strings.Join(pures, "\n")
		// 投喂脚本
		if c.script != nil {
			c.script.Feed(puretext)
		}

		// 检查触发器
		c.checkTrigger(pures)
	}
}

// 加载触发器到缓存
func (c *Client) loadTriggers() {
	if c.db == nil {
		return
	}
	c.triggers = make(map[string]string)
	c.db.View(func(tx *lmdb.Tx) error {
		tx.AscendKeys("trigger:*", func(key, command string) bool {
			c.triggers[key[8:]] = command
			return true
		})
		return nil
	})
}

// 从 DB 加载别名到内存缓存
func (c *Client) loadAliases() {
	if c.db == nil {
		return
	}
	c.muAlias.Lock()
	defer c.muAlias.Unlock()
	c.aliases = make(map[string]string)
	c.db.View(func(tx *lmdb.Tx) error {
		tx.AscendKeys("alias:*", func(key, command string) bool {
			c.aliases[key[6:]] = command
			return true
		})
		return nil
	})
}

// 检查 SKIP 触发器，返回(原文的分行, 去除颜色的分行)
func (c *Client) checkSkip(text string) ([]string, []string) {
	var result []string
	var pures []string
	lines := strings.Split(text, "\n")
	c.muTrigger.Lock()
	defer c.muTrigger.Unlock()
	for _, line := range lines {
		pure := lib.CleanColor(line)
		skip := false
		for pattern, command := range c.triggers {
			if command == "SKIP" && match.Match(pure, pattern) {
				skip = true
				break
			}
		}
		if !skip {
			result = append(result, line)
			pures = append(pures, pure)
		}
	}
	return result, pures
}

// 检查文本是否匹配触发器
func (c *Client) checkTrigger(lines []string) {
	if c.db == nil || len(c.triggers) == 0 {
		return
	}
	c.muTrigger.Lock()
	defer c.muTrigger.Unlock()
	for _, line := range lines {
		for pattern, command := range c.triggers {
			if command != "SKIP" && match.Match(line, "*"+pattern+"*") {
				c.runTrigger(command)
				break
			}
		}
	}
}

// 执行触发器命令
func (c *Client) runTrigger(command string) {
	if c.script != nil && c.script.Running() {
		fmt.Println("(触发器中断了当前脚本)")
		c.script.Stop()
	}
	c.script = lib.NewScript(c.wc, c.aliases)
	go c.script.Run(command)
	fmt.Printf("⚡ 触发器触发: %q\n", command)
}
