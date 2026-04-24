package lib

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// 移除 Telnet IAC 序列，保留实际数据
func FilterIAC(buf []byte) []byte {
	const IAC = 255
	const WILL = 251
	const WONT = 252
	const DO = 253
	const DONT = 254
	const SB = 250
	const SE = 240

	out := make([]byte, 0, len(buf))
	i := 0
	for i < len(buf) {
		if buf[i] == IAC {
			if i+1 >= len(buf) {
				break
			}
			next := buf[i+1]
			switch next {
			case WILL, WONT, DO, DONT:
				// IAC + cmd + option = 3 bytes
				i += 3
				if i > len(buf) {
					i = len(buf)
				}
			case SB:
				// IAC SB ... IAC SE，跳到 SE
				i += 2
				for i < len(buf) {
					if buf[i] == IAC && i+1 < len(buf) && buf[i+1] == SE {
						i += 2
						break
					}
					i++
				}
			case IAC:
				// 转义的 255，保留一个
				out = append(out, IAC)
				i += 2
			default:
				// 未知命令，跳过 2 字节
				i += 2
			}
		} else {
			out = append(out, buf[i])
			i++
		}
	}
	return out
}

// 移除所有控制字符
func StripCtrl(s string) string {
	clean := ""
	for _, r := range s {
		if (r >= 32 && r <= 126) || r == '\r' || r == '\n' || r == '\t' {
			clean += string(r)
		}
	}
	return clean
}

// 移除行尾一组\r\n
func TrimRN(s string) string {
	if strings.HasSuffix(s, "\r\n") {
		return s[:len(s)-2]
	}
	return s
}

var rnRe = regexp.MustCompile(`[\r\n]`)

func RemoveRN(s string) string {
	return rnRe.ReplaceAllString(s, "")
}

var newlineFixer = strings.NewReplacer("\r", "")

// 移除\r
func RemoveCr(input string) string {
	return newlineFixer.Replace(input)
}

var (
	// 匹配 ANSI 颜色码
	reANSI = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	// 修复换行的正则：现在它可以大胆地匹配 [^标点\s] 了
	//reWrap = regexp.MustCompile(`([^.,!?;:。，！？；：\s\x01])[\t ]{0,2}\r\n[\t ]{0,2}([a-zA-Z])`)
	reWrap = regexp.MustCompile(`([^.!?;:。！？；：\-\s\x01])[\t ]{0,2}[\r\n][\t ]{0,10}([a-zA-Z])`)
)

// 移除硬回车
// 提取颜色(\x1b) -> 放入隔离位(\x01) -> 清理折行(\r\n) -> 还原颜色(\x1b)
func CleanWrap(s string) string {
	// 1. 提取并保护所有的 ANSI 颜色码
	var codes []string
	// 使用一个特殊的占位符，例如 "【COLOR_TOKEN】"
	// 或者是更保险的不可见字符序列
	tmp := reANSI.ReplaceAllStringFunc(s, func(m string) string {
		codes = append(codes, m)
		return "\x01" // 临时占位符
	})

	// 2. 执行修复换行的逻辑
	// 此时文本中没有颜色码干扰，"/ \r\n\r\nThe" 这种双换行会因为第一个 \r\n 后面没字母而失败
	tmp = reWrap.ReplaceAllString(tmp, "$1 $2")

	// 3. 把颜色码还原回去
	for _, code := range codes {
		tmp = strings.Replace(tmp, "\x01", code, 1)
	}

	return tmp
}

// 清除颜色代码
func CleanColor(s string) string {
	return reANSI.ReplaceAllString(s, "")
}

// StripANSI 提取：前缀颜色, 前缀缩进, 后缀(颜色+换行), 中间纯文本
func StripANSI(raw string) (preColor string, indent string, suffix string, content string) {
	// 1. 定义匹配 [ANSI序列] 或 [空白符] 的模式
	// 注意：这里我们分开捕获，或者先拿到整体再拆分
	fullPrefixRegex := regexp.MustCompile(`^(\s|\x1b\[[0-9;]*[a-zA-Z])*`)
	fullPrefix := fullPrefixRegex.FindString(raw)

	// 从 fullPrefix 中分离颜色和缩进
	// 缩进通常指【最后一段】连续的空格，或者所有的空白符
	// 这里我们采取：提取所有颜色码作为 preColor，剩下的空白符作为 indent
	preColor = strings.Join(reANSI.FindAllString(fullPrefix, -1), "")
	indent = reANSI.ReplaceAllString(fullPrefix, "")

	// 2. 提取后缀 (颜色 + 换行/空格)
	fullSuffixRegex := regexp.MustCompile(`(\s|\x1b\[[0-9;]*[a-zA-Z])*$`)
	suffix = fullSuffixRegex.FindString(raw)

	// 3. 提取中间纯文本
	if len(fullPrefix) < len(raw) {
		// 截取中间部分
		rawContent := raw[len(fullPrefix) : len(raw)-len(suffix)]
		// 移除中间夹杂的颜色码
		content = reANSI.ReplaceAllString(rawContent, "")
	}

	return preColor, indent, suffix, content
}

// 方向到反方向的映射
var revDir = map[string]string{
	"n": "s", "s": "n",
	"e": "w", "w": "e",
	"ne": "sw", "sw": "ne",
	"nw": "se", "se": "nw",
	"u": "d", "d": "u",
	"eu": "wd", "wd": "eu",
	"wu": "ed", "ed": "wu",
	"nu": "sd", "sd": "nu",
	"su": "nd", "nd": "su",
	"enter": "out", "out": "enter",
}

// ReversePath 反转 MUD 路径字符串, 例如 "e;#2s;ne" 返回 "sw;#2n;w"
// 支持 #2s 和 #2 s 两种带次数格式
func ReversePath(path string) (string, error) {
	segs := strings.Split(path, ";")
	for i, seg := range segs {
		var dir string
		var cnt int
		if strings.HasPrefix(seg, "#") {
			n, err := fmt.Sscanf(seg, "#%d%s", &cnt, &dir)
			if err != nil || n != 2 {
				return "", fmt.Errorf("无法解析路径段: %s", seg)
			}
		} else {
			dir = seg
			cnt = 1
		}
		rev, ok := revDir[dir]
		if !ok {
			return "", fmt.Errorf("无法识别的方向: %s", dir)
		}
		if cnt > 1 {
			segs[i] = fmt.Sprintf("#%d%s", cnt, rev)
		} else {
			segs[i] = rev
		}
	}
	// 翻转段顺序
	for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
		segs[i], segs[j] = segs[j], segs[i]
	}
	return strings.Join(segs, ";"), nil
}

// IsEnglishDominant 判断英文字符（A-Z, a-z）的个数是否超过总字符数的一半
func IsEnglishDominant(s string) bool {
	if len(s) == 0 {
		return false
	}

	enCount := 0
	totalCount := 0

	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		// 统计总字符数（Rune count）
		totalCount++

		// 判断是否为英文字母
		// 如果你想把数字和常用标点也算作英文范畴，可以修改这里的逻辑
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			enCount++
		}
	}

	// 判断比例是否超过 50%
	return float64(enCount) > float64(totalCount)*0.5
}
