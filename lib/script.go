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
// 全局变量，跨脚本共享捕获的变量
var VARS = map[string]string{}

// 是否打印调试信息
var DEBUG bool

type Script struct {
	wc      chan string       // 命令发送管道
	waitCh  chan string       // 服务器文本管道
	stopCh  chan struct{}     // 中断信号
	timeout time.Duration     // 命令执行超时时间
	running bool              // 标记是否正在运行
	aliases map[string]string // 别名快照（NewScript 时从 client 复制）
	gap     time.Duration     // 命令之间强行停顿一下(秒)
}

// 创建新的脚本引擎，aliases 为别名快照（内部会复制一份）
func NewScript(wc chan string, aliases map[string]string) *Script {
	a := make(map[string]string, len(aliases))
	for k, v := range aliases {
		a[k] = v
	}
	return &Script{
		wc:      wc,
		waitCh:  make(chan string, 100),
		stopCh:  make(chan struct{}),
		timeout: 300 * time.Second,
		aliases: a,
		gap:     200 * time.Millisecond,
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
	s.processCmds(strings.Split(input, ";"))
}

// 处理命令序列，支持 #loop/#jmp/#wa/%N/#N/cmd:keyword 及别名展开
func (s *Script) processCmds(cmds []string) {
	for i := 0; i < len(cmds); i++ {
		if s.stopped() {
			return
		}
		if DEBUG {
			fmt.Printf("[script:%d] %s\n", i, cmds[i])
		}
		if i > 0 {
			d := s.gap / 2
			if !s.wait(s.gap - d/2 + rand.N(d)) {
				return
			}
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
		// #jmp N 或 #jmp +N / #jmp -N：相对跳转
		if cmd, ok := strings.CutPrefix(cmd, "#jmp"); ok {
			offsetStr := strings.TrimSpace(cmd)
			isRelative := false
			if len(offsetStr) > 0 && (offsetStr[0] == '+' || offsetStr[0] == '-') {
				isRelative = true
			}
			if !isRelative {
				// 绝对跳转：原逻辑（1-based 行号）
				n, _ := strconv.Atoi(offsetStr)
				if n <= 0 {
					n = 1
				}
				i = n - 2
			} else {
				// 相对跳转：从 #jmp 当前位置偏移
				// -N: 往左跳 N 步, +N: 往右跳 N 步
				offset, _ := strconv.Atoi(offsetStr)
				targetIdx := i + offset
				if targetIdx < 0 {
					targetIdx = 0
				}
				i = targetIdx - 1 // -1 补偿 for 循环的 i++
			}
			continue
		}
		// #if <expr> <action>: 条件跳转或执行命令
		if cmd, ok := strings.CutPrefix(cmd, "#if"); ok {
			rest := strings.TrimSpace(cmd)
			expanded := s.subst(rest)

			// 从操作符右侧找到 action 的起始位置
			splitPos := findActionSplitPos(expanded)

			if splitPos > 0 {
				expr := strings.TrimSpace(expanded[:splitPos])
				action := strings.TrimSpace(expanded[splitPos+1:])
				if evalCompare(expr) {
					if n, err := strconv.Atoi(action); err == nil {
						if n <= 0 {
							n = 1
						}
						i = n - 2
					} else if action == "break" {
						return
					} else {
						s.executeCmd(action)
					}
				}
			}
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
		// #gap 指令：设置停顿时长
		if cmd, ok := strings.CutPrefix(cmd, "#gap"); ok {
			duration := parseDuration(cmd)
			s.gap = duration
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
			if idx := strings.Index(cmd, ":"); idx > 0 {
				keyword := strings.TrimSpace(cmd[idx+1:])
				raw := strings.TrimSpace(cmd[:idx])
				if expanded, ok := ExpandAlias(s.aliases, raw); ok {
					s.processCmds(strings.Split(expanded, ";"))
				} else {
					s.sendCmd(raw)
				}
				if !s.waitKeyword(keyword) {
					return
				}
			} else {
				s.executeCmd(cmd)
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
	re := makePattern(keyword)
	// 预编译 OR 子条件
	parts := splitOr(keyword)
	var subRes []*regexp.Regexp
	if len(parts) > 1 {
		subRes = make([]*regexp.Regexp, len(parts))
		for i, part := range parts {
			subRes[i] = makePattern(part)
		}
	}

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
					VARS[strconv.Itoa(i)] = strings.TrimSpace(subs[i])
				}
				// 命名捕获：$name（用 SubexpNames 获取正确组索引）
				for i, name := range re.SubexpNames() {
					if name != "" && i < len(subs) {
						VARS[name] = strings.TrimSpace(subs[i])
					}
				}
				// OR 条件编号
				for ci, subRe := range subRes {
					if subRe != nil && subRe.MatchString(text) {
						VARS["C"] = strconv.Itoa(ci + 1)
						break
					} else if subRe == nil && strings.Contains(text, parts[ci]) {
						VARS["C"] = strconv.Itoa(ci + 1)
						break
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
//	{name} → 命名捕获组 (?P<name>...)
//	*      → 位置捕获组 (.*?)
//	?      → 单字符捕获组 (.)
//	|      → 多条件或（在 {} 外时）
//
// 返回编译好的 regex（{name} 内嵌为 (?P<name>...) 以便 SubexpNames 正确映射）
func makePattern(keyword string) *regexp.Regexp {
	parts := splitOr(keyword)
	if len(parts) > 1 {
		if re := makeOrPattern(parts); re != nil {
			return re
		}
		// 编译失败则 fall through 让 | 作为字面字符处理
	}

	hasGlob := strings.ContainsAny(keyword, "*?")
	hasNamed := strings.Contains(keyword, "{")

	if !hasGlob && !hasNamed {
		// 纯文本关键字，也可能是有 regex 元字符但无通配符（如 \s+(\d+)）
		if containsRegexMeta(keyword) {
			re, _ := regexp.Compile(keyword)
			return re
		}
		return nil
	}

	if hasGlob || hasNamed {
		// 有 * 或 ? 时永远按 glob 语义处理，不能先尝试 regexp.Compile
		// 否则 "气血*/*" 会被 Go 编译为合法 regex（血*=零或多个血），
		// 而非用户期望的 glob（*=匹配任意字符）
	}

	// 转换通配符和命名捕获为 regex
	// 必须用 rune 迭代处理多字节 UTF-8 中文，byte 迭代会拆散字符
	var buf strings.Builder
	buf.WriteString("(?s)") // DOTALL: . 匹配 \n，支持跨行匹配
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
			name := string(runes[i+1 : j])
			if hasNextLiteral(runes, j) {
				buf.WriteString("(?P<" + name + ">.*?)")
			} else {
				buf.WriteString("(?P<" + name + ">.*)")
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
		return nil
	}
	return re
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
			val, ok := VARS[name]
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
			val, ok := VARS[name]
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

// findActionSplitPos 从操作符后找到 action 的起始位置
// expanded: 展开后的完整字符串，如"东 边="东 边""或"250>100 drink"
func findActionSplitPos(expanded string) int {
	// 找到最后一个运算符（优先级从高到低）
	var opIdx, opLen int
	for _, candidate := range []string{">=", "<=", "!=", "=", ">", "<"} {
		if idx := strings.Index(expanded, candidate); idx >= 0 && idx > opIdx {
			opIdx = idx
			opLen = len(candidate)
		}
	}

	start := opIdx + opLen
	for start < len(expanded) && expanded[start] == ' ' {
		start++
	}

	if start < len(expanded) && expanded[start] == '"' {
		for i := start + 1; i < len(expanded); i++ {
			if expanded[i] == '"' {
				return i + 1
			}
		}
	}

	return strings.IndexByte(expanded[start:], ' ') + start
}

// 解析并计算比较表达式，如 "250>100" 返回 true
func evalCompare(expr string) bool {
	// 先尝试双字符运算符 >=, <=, !=
	for _, op := range []string{">=", "<=", "!="} {
		if parts := strings.SplitN(expr, op, 2); len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])

			leftQuoted := isQuoted(left)
			rightQuoted := isQuoted(right)

			// 如果任意一边是双引号包裹，优先进行字符串比较
			if leftQuoted || rightQuoted {
				var l, r string
				if leftQuoted {
					l = strings.Trim(left, `"`)
				} else {
					l = left
				}
				if rightQuoted {
					r = strings.Trim(right, `"`)
				} else {
					r = right
				}
				return compareStrings(l, r, op)
			}

			// 尝试数字比较
			l, err1 := strconv.ParseFloat(left, 64)
			r, err2 := strconv.ParseFloat(right, 64)

			// 两边都能解析为数字 → 数字比较
			if err1 == nil && err2 == nil {
				switch op {
				case ">=":
					return l >= r
				case "<=":
					return l <= r
				case "!=":
					return l != r
				}
			} else if err1 != nil && err2 != nil {
				// 两边都不是数字且是 = 或!= → 字符串比较（带 trim）
				if op == "=" || op == "!=" {
					return compareStrings(strings.TrimSpace(left), strings.TrimSpace(right), op)
				}
				// > < <= >= 对非数字仍 fallback 到数字比较 (0)
			}

			// 数字比较（失败则为 0）
			switch op {
			case ">=":
				return l >= r
			case "<=":
				return l <= r
			case "!=":
				return l != r
			}
		}
	}
	// 再试单字符运算符 =, >, <
	for _, op := range []string{"=", ">", "<"} {
		if parts := strings.SplitN(expr, op, 2); len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])

			leftQuoted := isQuoted(left)
			rightQuoted := isQuoted(right)

			// 如果任意一边是双引号包裹，优先进行字符串比较
			if leftQuoted || rightQuoted {
				var l, r string
				if leftQuoted {
					l = strings.Trim(left, `"`)
				} else {
					l = left
				}
				if rightQuoted {
					r = strings.Trim(right, `"`)
				} else {
					r = right
				}
				return compareStrings(l, r, op)
			}

			// 尝试数字比较
			l, err1 := strconv.ParseFloat(left, 64)
			r, err2 := strconv.ParseFloat(right, 64)

			// 两边都能解析为数字 → 数字比较
			if err1 == nil && err2 == nil {
				switch op {
				case "=":
					return l == r
				case ">":
					return l > r
				case "<":
					return l < r
				}
			} else if err1 != nil && err2 != nil {
				// 两边都不是数字 → 字符串相等比较（带 trim）
				if op == "=" {
					return strings.TrimSpace(left) == strings.TrimSpace(right)
				}
				// > < 对非数字 fallback 到数字比较 (0)
			}

			// 数字比较（失败则为 0）
			switch op {
			case "=":
				return l == r
			case ">":
				return l > r
			case "<":
				return l < r
			}
		}
	}
	return false
}

// 判断字符串是否被双引号包裹
func isQuoted(s string) bool {
	return len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"'
}

// 字符串比较
func compareStrings(left, right, op string) bool {
	switch op {
	case "=":
		return left == right
	case "!=":
		return left != right
	}
	return false
}

// 投喂服务器文本
func (s *Script) Feed(text string) {
	if !s.running {
		return
	}
	select {
	case s.waitCh <- text:
	default:
	}
}

// 停止脚本
func (s *Script) Stop() {
	close(s.stopCh)
}

func (s *Script) Running() bool {
	return s.running
}

// 检查是否收到中断信号
func (s *Script) stopped() bool {
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}

// 发送命令到服务器（经过变量替换）
func (s *Script) sendCmd(raw string) {
	s.wc <- s.subst(raw)
}

// 执行单条命令，支持关键字等待格式 cmd:keyword 和别名展开
func (s *Script) executeCmd(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	// 关键字等待
	if idx := strings.Index(cmd, ":"); idx > 0 {
		keyword := strings.TrimSpace(cmd[idx+1:])
		cmd = strings.TrimSpace(cmd[:idx])
		if expanded, ok := ExpandAlias(s.aliases, cmd); ok {
			s.processCmds(strings.Split(expanded, ";"))
		} else {
			s.sendCmd(cmd)
		}
		if !s.waitKeyword(keyword) {
			return
		}
	} else {
		if expanded, ok := ExpandAlias(s.aliases, cmd); ok {
			s.processCmds(strings.Split(expanded, ";"))
			return
		}
		s.sendCmd(cmd)
	}
}

// ExpandAlias 查找并展开别名，替换 $A1-$A9 位置参数
// 取 cmd 第一个空格前的单词作为别名名称，剩余部分按空格拆分为参数
// 返回展开后的字符串和是否找到别名
func ExpandAlias(aliases map[string]string, cmd string) (string, bool) {
	idx := strings.IndexByte(cmd, ' ')
	var name, args string
	if idx > 0 {
		name = cmd[:idx]
		args = strings.TrimSpace(cmd[idx+1:])
	} else {
		name = cmd
	}
	template, ok := aliases[name]
	if !ok {
		return cmd, false
	}
	argParts := strings.Fields(args)
	result := template
	for i := 9; i >= 1; i-- {
		key := fmt.Sprintf("$A%d", i)
		if strings.Contains(result, key) {
			if i-1 < len(argParts) {
				result = strings.ReplaceAll(result, key, argParts[i-1])
			} else {
				result = strings.ReplaceAll(result, key, "")
			}
		}
	}
	return result, true
}

// splitOr 按 | 分割关键字，跳过 {} 内的 |
func splitOr(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
		case '|':
			if depth == 0 {
				if i > start {
					parts = append(parts, s[start:i])
				}
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	// 过滤空字符串
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// makeOrPattern 将多个匹配条件编译为带 alternation 的单个 regex
// 各条件的捕获组按顺序编号：条件1 的 *→$1，条件2 的 *→$2
func makeOrPattern(parts []string) *regexp.Regexp {
	var buf strings.Builder
	buf.WriteString("(?s)")
	for i, part := range parts {
		if i > 0 {
			buf.WriteString("|")
		}
		sub := makePattern(part)
		if sub != nil {
			s := strings.TrimPrefix(sub.String(), "(?s)")
			buf.WriteString("(?:")
			buf.WriteString(s)
			buf.WriteString(")")
		} else {
			buf.WriteString("(?:")
			buf.WriteString(regexp.QuoteMeta(part))
			buf.WriteString(")")
		}
	}
	re, err := regexp.Compile(buf.String())
	if err != nil {
		return nil
	}
	return re
}
