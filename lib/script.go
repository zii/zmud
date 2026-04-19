package lib

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 线性指令步进引擎
// 语法：cmd1:keyword1; cmd2; cmd3:keyword2
//   - 有冒号：发送后阻塞等待，直到服务器返回包含关键字的文本或超时（1秒）
//   - 无冒号：直接发送，不等待
//
// 示例
// look:正厅; e:东厢房; sleep:醒来; w; n
// 逻辑： 发送 look -> 阻塞直到收到“正厅” -> 发送 e -> 阻塞直到收到“东厢房” -> 发送 sleep -> 阻塞直到收到“醒来” -> 顺序发送 w 和 n。
// 支持内置延迟指令
// look; #wa 1s; get all
type Script struct {
	wc      chan string   // 命令发送管道
	waitCh  chan string   // 服务器文本管道
	stopCh  chan struct{} // 中断信号
	timeout time.Duration
	running bool // 标记是否正在运行
}

// 创建新的脚本引擎
func NewScript(wc chan string) *Script {
	return &Script{
		wc:      wc,
		waitCh:  make(chan string, 100),
		stopCh:  make(chan struct{}),
		timeout: time.Second,
	}
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
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		cmd := strings.TrimSpace(cmds[i])
		if cmd == "" {
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
		if strings.HasPrefix(cmd, "#jmp ") {
			n, err := strconv.Atoi(strings.TrimPrefix(cmd, "#jmp "))
			if err != nil || n < 1 {
				fmt.Println("#jmp 无效")
				return
			}
			i = n - 2 // -2 因为 for 循环有 i++
			continue
		}
		// #wa 指令：可中断的等待
		if strings.HasPrefix(cmd, "#wa ") {
			duration := parseDuration(strings.TrimPrefix(cmd, "#wa "))
			if !s.wait(duration) {
				return
			}
			continue
		}
		// 关键字等待
		if i := strings.Index(cmd, ":"); i > 0 {
			keyword := strings.TrimSpace(cmd[i+1:])
			cmd = strings.TrimSpace(cmd[:i])
			s.wc <- cmd
			if !s.waitKeyword(keyword) {
				return
			}
		} else {
			s.wc <- cmd
		}
	}
}

// 计算前面所有 #wa 指令的总延迟时间
func totalWaitDuration(cmds []string) time.Duration {
	var total time.Duration
	for _, c := range cmds {
		if s, ok := strings.CutPrefix(c, "#wa "); ok {
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
	// 默认认为是毫秒
	v, _ := strconv.Atoi(s)
	return time.Duration(v) * time.Millisecond
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

// 等待服务器返回包含关键字的文本
func (s *Script) waitKeyword(keyword string) bool {
	deadline := time.Now().Add(s.timeout)
	for time.Now().Before(deadline) {
		select {
		case text := <-s.waitCh:
			if strings.Contains(text, keyword) {
				return true
			}
		case <-s.stopCh:
			return false
		}
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
