package lib

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 线性指令步进引擎
// 语法：cmd1:keyword1; cmd2; cmd3:keyword2
//   - 有冒号：发送后阻塞等待，直到服务器返回包含关键字的文本或超时（1秒）
//   - 无冒号：直接发送，不等待
//
// 示例一
// look:正厅; e:东厢房; sleep:醒来; w; n
// 逻辑： 发送 look -> 阻塞直到收到"正厅" -> 发送 e -> 阻塞直到收到"东厢房" -> 发送 sleep -> 阻塞直到收到"醒来" -> 顺序发送 w 和 n。
//
// 支持内置延迟指令
// look; #wa 1s; get all
//
// 支持捕获变量（通配符 * 匹配并捕获，{name} 命名捕获）：
// hp:气血*/*;dazuo $1          → * 捕获第1段，引用 $1
// hp:气血{hp}/*;dazuo $hp      → {hp} 命名为 hp，引用 $hp
// hp:气血{hp}/*;dazuo $hp-20   → 支持四则运算（$hp-20=80）
type Script struct {
	wc      chan string       // 命令发送管道
	waitCh  chan string       // 服务器文本管道
	stopCh  chan struct{}     // 中断信号
	timeout time.Duration     // 命令执行超时时间
	running bool              // 标记是否正在运行
	vars    map[string]string // 最近一次 waitKeyword 的捕获变量
}

// 创建新的脚本引擎
func NewScript(wc chan string) *Script {
	return &Script{
		wc:      wc,
		waitCh:  make(chan string, 100),
		stopCh:  make(chan struct{}),
		timeout: 300 * time.Second,
		vars:    make(map[string]string),
	}
}

var repeatRe = regexp.MustCompile(`^#([0-9]+)\s*(.*)`)

// 匹配并提取重复指令: 例如输入 "#3 s", 返回(3, "s")
func matchRepeat(text string) (int, string) {
	// 预编译正则提高效率
	subs := repeatRe.FindStringSubmatch(text)
	if len(subs) < 3 {
		return 1, text
	}

	// 将字符串类型的数字转换为 int
	num, err := strconv.Atoi(subs[1])
	if err != nil {
		return 1, text
	}

	return num, subs[2]
}

// 运行脚本，解析并执行指令
func (s *Script) Run(input string) {
	s.running = true
	defer func() {
		s.running = false
		close(s.waitCh)
	}()
	cmds := strings.Split(input, ";")
	for i := 0; i < len(cmds); i++ {
		//fmt.Printf("[script:%d] %s\n", i, cmds[i])
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		cmd := strings.TrimSpace(cmds[i])
		if cmd == "" {
			s.wc <- s.subst(cmd)
			continue
		}

		// #loop 指令：从头重新执行（安全检查：前面 #wa 总时间需 >= 1s）
		if cmd == "#loop" {
			if totalWaitDuration(cmds[:i]) < time.Second {
				fmt.Println("#loop 被拒绝: 前面 #wa 总时间需 >= 1s")
				return
			}
			i = -1 // 重置索引，下次循环从 0 开始
			continue
		}
		// #jmp N：跳转到第 N 条命令
		if cmd, ok := strings.CutPrefix(cmd, "#jmp"); ok {
			n, _ := strconv.Atoi(strings.TrimSpace(cmd))
			if n <= 0 {
				n = 1
			}
			i = n - 2 // -2 因为 for 循环有 i++
			continue
		}
		// #wa 指令：可中断的等待
		if cmd, ok := strings.CutPrefix(cmd, "#wa"); ok {
			duration := parseDuration(cmd)
			if !s.wait(duration) {
				return
			}
			continue
		}
		// %N 指令: 概率执行（如 %20 drink）
		if strings.HasPrefix(cmd, "%") {
			probRe := regexp.MustCompile(`^%(\d+)(.+)$`)
			match := probRe.FindStringSubmatch(cmd)
			if match == nil {
				// 格式无效，跳过
				continue
			}
			prob, _ := strconv.Atoi(match[1])
			if prob < 0 {
				prob = 0
			}
			if prob > 100 {
				prob = 100
			}
			// 命中概率则执行实际命令，否则跳过
			if rand.N(100) < prob {
				s.executeCmd(match[2])
			}
			continue
		}
		// #N 指令: 重复数次执行
		repeat, cmd := matchRepeat(cmd)
		for k := 0; k < repeat; k++ {
			// 关键字等待
			if i := strings.Index(cmd, ":"); i > 0 {
				keyword := strings.TrimSpace(cmd[i+1:])
				cmd = strings.TrimSpace(cmd[:i])
				s.wc <- s.subst(cmd)
				if !s.waitKeyword(keyword) {
					return
				}
			} else {
				s.wc <- s.subst(cmd)
			}
		}
	}
}

// 计算前面所有 #wa 指令的总延迟时间
func totalWaitDuration(cmds []string) time.Duration {
	var total time.Duration
	for _, c := range cmds {
		if s, ok := strings.CutPrefix(c, "#wa"); ok {
			total += parseDuration(s)
		}
	}
	return total
}

// 解析时间字符串，支持 500ms, 1s, 2.5s 等格式
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	// 处理 "500ms", "1s", "2.5s" 格式
	if strings.HasSuffix(s, "ms") {
		v, _ := strconv.Atoi(strings.TrimSuffix(s, "ms"))
		return time.Duration(v) * time.Millisecond
	}
	if strings.HasSuffix(s, "s") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "s"), 64)
		return time.Duration(v * float64(time.Second))
	}
	// 默认认为是秒
	v, _ := strconv.ParseFloat(s, 64)
	return time.Duration(v * float64(time.Second))
}

// 等待指定时间，可中断
func (s *Script) wait(d time.Duration) bool {
	select {
	case <-s.stopCh:
		return false
	case <-time.After(d):
		return true
	}
}

// 等待服务器返回包含关键字的文本，支持正则、通配符捕获和命名捕获
func (s *Script) waitKeyword(keyword string) bool {
	timeout := time.After(s.timeout)
	re, names := makePattern(keyword)

	for {
		select {
		case text := <-s.waitCh:
			if re != nil {
				subs := re.FindStringSubmatch(text)
				if subs == nil {
					continue
				}
				// 位置捕获：$1, $2 ...
				for i := 1; i < len(subs); i++ {
					s.vars[strconv.Itoa(i)] = strings.TrimSpace(subs[i])
				}
				// 命名捕获：$name
				for i, name := range names {
					if i+1 < len(subs) {
						s.vars[name] = strings.TrimSpace(subs[i+1])
					}
				}
				return true
			}
			// 纯文本匹配
			if strings.Contains(text, keyword) {
				return true
			}
		case <-s.stopCh:
			return false
		case <-timeout:
			fmt.Printf("等待关键字 [%s] 超时\n", keyword)
			return false
		}
	}
}

// 检测字符串是否包含正则表达式元字符
func containsRegexMeta(s string) bool {
	for _, c := range []byte("()|*+.") {
		if strings.Contains(s, string(c)) {
			return true
		}
	}
	return false
}

// 检查 rune 切片 i 位置之后是否还有字面字符（非通配符、非命名捕获）
func hasNextLiteral(runes []rune, i int) bool {
	for j := i + 1; j < len(runes); j++ {
		r := runes[j]
		switch r {
		case '*', '?', '}':
			continue
		case '{':
			// 跳过 {name}
			for j++; j < len(runes) && runes[j] != '}'; j++ {
			}
		default:
			return true
		}
	}
	return false
}

// 将包含通配符和命名捕获的关键字转为正则表达式
//
//	{name} → 命名捕获组 (.*)
//	*      → 位置捕获组 (.*)
//	?      → 单字符捕获组 (.)
//	其余 regex 元字符自动转义
//
// 返回编译好的 regex 和命名列表（names[i] 对应 group i+1）
func makePattern(keyword string) (*regexp.Regexp, []string) {
	hasGlob := strings.ContainsAny(keyword, "*?")
	hasNamed := strings.Contains(keyword, "{")

	if !hasGlob && !hasNamed {
		// 纯文本关键字，也可能是有 regex 元字符但无通配符（如 \s+(\d+)）
		if containsRegexMeta(keyword) {
			re, _ := regexp.Compile(keyword)
			return re, nil
		}
		return nil, nil
	}

	if hasGlob || hasNamed {
		// 有 * 或 ? 时永远按 glob 语义处理，不能先尝试 regexp.Compile
		// 否则 "气血*/*" 会被 Go 编译为合法 regex（血*=零或多个血），
		// 而非用户期望的 glob（*=匹配任意字符）
	}

	// 转换通配符和命名捕获为 regex
	// 必须用 rune 迭代处理多字节 UTF-8 中文，byte 迭代会拆散字符
	var names []string
	var buf strings.Builder
	runes := []rune(keyword)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '{':
			// 在剩余的 rune 中找 }
			j := -1
			for k := i + 1; k < len(runes); k++ {
				if runes[k] == '}' {
					j = k
					break
				}
			}
			if j < 0 {
				// 没有闭合的 }，按字面处理
				buf.WriteString(regexp.QuoteMeta(string(r)))
				continue
			}
			names = append(names, string(runes[i+1:j]))
			if hasNextLiteral(runes, i) {
				buf.WriteString("(.*?)")
			} else {
				buf.WriteString("(.*)")
			}
			i = j // for 循环有 i++
		case r == '*':
			if hasNextLiteral(runes, i) {
				buf.WriteString("(.*?)")
			} else {
				buf.WriteString("(.*)")
			}
		case r == '?':
			buf.WriteString("(.)")
		default:
			buf.WriteString(regexp.QuoteMeta(string(r)))
		}
	}

	re, err := regexp.Compile(buf.String())
	if err != nil {
		return nil, nil
	}
	return re, names
}

// 替换命令中的 $ 变量引用，支持四则运算
//
//	$$    → 字面 $
//	$name → vars[name]
//	$N    → vars[N]（N 为 1 位数字）
//	$name+N → 算术运算（支持 + - * /，结果取整）
func (s *Script) subst(cmd string) string {
	var buf strings.Builder
	for i := 0; i < len(cmd); i++ {
		if cmd[i] != '$' {
			buf.WriteByte(cmd[i])
			continue
		}
		if i+1 >= len(cmd) {
			buf.WriteByte('$')
			continue
		}
		next := cmd[i+1]
		switch {
		case next == '$':
			// $$ → 字面 $
			buf.WriteByte('$')
			i++
		case isLetter(next):
			// $name → 读取完整变量名
			j := i + 1
			for ; j < len(cmd) && isAlphaNum(cmd[j]); j++ {
			}
			name := cmd[i+1 : j]
			val, ok := s.vars[name]
			if ok {
				varNum, j2 := tryArithmetic(cmd, j, val)
				if j2 > j {
					i = j2 - 1
				} else {
					i = j - 1
				}
				buf.WriteString(varNum)
			} else {
				buf.WriteString(cmd[i:j])
				i = j - 1
			}
		case next >= '0' && next <= '9':
			// $N → 位置捕获
			name := string(next)
			val, ok := s.vars[name]
			if ok {
				varNum, j2 := tryArithmetic(cmd, i+2, val)
				if j2 > i+2 {
					i = j2 - 1
				} else {
					i = i + 1
				}
				buf.WriteString(varNum)
			} else {
				buf.WriteByte('$')
				buf.WriteByte(next)
				i++
			}
		default:
			buf.WriteByte('$')
		}
	}
	return buf.String()
}

// 尝试解析变量值后的算术运算，如 $hp-20 → 100-20=80
// 返回计算后的字符串和新的下标，无可运算时原值返回
func tryArithmetic(cmd string, start int, val string) (string, int) {
	if start >= len(cmd) {
		return val, start
	}
	op := cmd[start]
	if op != '+' && op != '-' && op != '*' && op != '/' {
		return val, start
	}
	// 读取运算符后的数字（支持小数）
	j := start + 1
	if j >= len(cmd) || !isDigit(cmd[j]) && cmd[j] != '.' {
		return val, start
	}
	for ; j < len(cmd) && (isDigit(cmd[j]) || cmd[j] == '.'); j++ {
	}
	operand, err := strconv.ParseFloat(cmd[start+1:j], 64)
	if err != nil {
		return val, start
	}
	varVal, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return val, start
	}
	var result float64
	switch op {
	case '+':
		result = varVal + operand
	case '-':
		result = varVal - operand
	case '*':
		result = varVal * operand
	case '/':
		if operand == 0 {
			return val, start
		}
		result = varVal / operand
	}
	return strconv.Itoa(int(result)), j
}

// 判断字节是否为字母
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// 判断字节是否为字母数字或下划线
func isAlphaNum(b byte) bool {
	return isLetter(b) || (b >= '0' && b <= '9')
}

// 判断字节是否为数字
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// 投喂服务器文本
func (s *Script) Feed(lines []string) {
	for _, line := range lines {
		if !s.running {
			return
		}
		select {
		case s.waitCh <- line:
		default:
		}
	}
}

// 停止脚本
func (s *Script) Stop() {
	close(s.stopCh)
}

func (s *Script) Running() bool {
	return s.running
}

// 执行单条命令，支持关键字等待格式 cmd:keyword
func (s *Script) executeCmd(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	// 关键字等待
	if i := strings.Index(cmd, ":"); i > 0 {
		keyword := strings.TrimSpace(cmd[i+1:])
		cmd = strings.TrimSpace(cmd[:i])
		s.wc <- s.subst(cmd)
		s.waitKeyword(keyword)
	} else {
		s.wc <- s.subst(cmd)
	}
}
