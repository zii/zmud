package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"zmud/lib"
)

// chooseServer 解析命令行参数，无参数时显示菜单选择服务器
// cfg: 完整的配置结构体，用于保存新添加的服务器
// 返回选中的服务器指针
func chooseServer(cfg *lib.Config) *lib.Server {
	args := os.Args[1:]

	// 有参数时，直接使用参数
	if len(args) >= 1 {
		host, port := args[0], "8080"
		if len(args) >= 2 {
			port = args[1]
		}
		// 构造临时 Server 返回
		return &lib.Server{Host: host, Port: port}
	}

	for {
		// 显示菜单（包含添加选项）
		fmt.Println("可用服务器:")
		for i, s := range cfg.Servers {
			fmt.Printf("  %d. %s (%s:%s)\n", i+1, s.Name, s.Host, s.Port)
		}
		fmt.Println("  +/- 添加删除服务器")

		var choice string
		fmt.Printf("请选择 [1-%d]: ", len(cfg.Servers))
		if _, err := fmt.Scanln(&choice); err != nil {
			continue
		}

		// 添加新服务器
		if choice == "+" {
			addNewServer(cfg)
			continue // 重新显示菜单
		}

		// 删除服务器
		if choice == "-" {
			deleteServer(cfg)
			continue // 重新显示菜单
		}

		// 选择现有服务器
		if n, err := strconv.Atoi(choice); err == nil && n >= 1 && n <= len(cfg.Servers) {
			return &cfg.Servers[n-1]
		}
	}
}

// addNewServer 提示用户输入新的服务器信息，保存到配置文件
func addNewServer(cfg *lib.Config) {
	fmt.Print("请输入服务器名称: ")
	var name string
	fmt.Scanln(&name)

	fmt.Print("请输入服务器IP或域名: ")
	var host string
	fmt.Scanln(&host)

	fmt.Print("请输入服务器端口: ")
	var port string
	fmt.Scanln(&port)

	fmt.Print("请输入服务器编码 (gb/big5，回车跳过): ")
	var charset string
	fmt.Scanln(&charset)

	server := lib.Server{Name: name, Host: host, Port: port, Charset: charset}
	cfg.Servers = append(cfg.Servers, server)

	if err := lib.SaveConfig(cfg); err != nil {
		fmt.Printf("保存失败：%v\n", err)
	} else {
		fmt.Println("服务器已添加并保存。")
	}
}

// deleteServer 删除服务器
func deleteServer(cfg *lib.Config) {
	if len(cfg.Servers) == 0 {
		fmt.Println("没有服务器可以删除。")
		return
	}

	fmt.Println("请选择要删除的服务器编号:")
	for i, s := range cfg.Servers {
		fmt.Printf("  %d. %s\n", i+1, s.Name)
	}

	var idx int
	fmt.Scanln(&idx)
	if idx < 1 || idx > len(cfg.Servers) {
		fmt.Println("无效选择。")
		return
	}

	name := cfg.Servers[idx-1].Name
	cfg.Servers = append(cfg.Servers[:idx-1], cfg.Servers[idx:]...)

	if err := lib.SaveConfig(cfg); err != nil {
		fmt.Printf("保存失败：%v\n", err)
	} else {
		fmt.Printf("服务器 %s 已删除。\n", name)
	}
}

func main() {
	// 加载配置
	cfg, err := lib.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败：%v\n", err)
		os.Exit(1)
	}

	// 确保配置完整（如果缺失 API 密钥，启动交互式向导）
	if !lib.EnsureConfig(cfg) {
		fmt.Fprintf(os.Stderr, "配置未完成，翻译功能将被禁用。\n")
	}

	for {
		// 使用配置中的服务器列表
		s := chooseServer(cfg)

		// 判断语言模式：gb/gbk/big5 则用原文
		mode := lib.LMIX
		charset := strings.ToLower(s.Charset)
		if charset == "gb" || charset == "gbk" || charset == "big5" {
			mode = lib.LSRC
		}
		c := NewClient(cfg, s, mode)

		if err := c.Connect(); err != nil {
			fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
			continue
		}

		// 设置终端窗口标题
		fmt.Print("\x1b]1;" + s.Name + "\x07")
		fmt.Printf("Connected to %s:%s\n", s.Host, s.Port)
		c.Run()
		break
	}
}
