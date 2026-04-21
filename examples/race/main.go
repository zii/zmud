// 终端竞争示例：使用 liner 库模拟 readInput 和 inputLoop 分离
package main

import (
	"fmt"
	"time"

	"zmud/lib/liner"
)

func main() {
	s := liner.NewLiner()
	defer s.Close()
	s.SetCtrlCAborts(true)
	inputCh := make(chan string, 10)

	// 模拟：inputLoop goroutine - 处理输入
	go func() {
		for input := range inputCh {
			fmt.Println("输入完成:", input)
		}
	}()

	// 模拟：readInput goroutine - 只读取输入
	go func() {
		for {
			input, err := s.Prompt("❯ ")
			if err != nil {
				return
			}
			inputCh <- input
		}
	}()

	// 模拟：服务器goroutine (readServer)
	time.Sleep(50 * time.Millisecond)
	go func() {
		for i := 0; i < 50; i++ {
			fmt.Println("服务器响应第", i+1, "行")
			time.Sleep(2 * time.Second)
		}
	}()

	select {}
}
