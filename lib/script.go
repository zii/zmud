package lib

import (
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
	for i, cmd := range cmds {
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		// 检查是否有关键字等待
		if i := strings.Index(cmd, ":"); i > 0 {
			keyword := strings.TrimSpace(cmd[i+1:])
			cmd = strings.TrimSpace(cmd[:i])
			s.wc <- cmd
			if !s.wait(keyword) {
				return // 超时或中断
			}
		} else {
			s.wc <- cmd
		}
	}
}

// 等待服务器返回包含关键字的文本
func (s *Script) wait(keyword string) bool {
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
