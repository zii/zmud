package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"zmud/lib"
)

// 读取整行输入，支持空格
func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// chooseServer 解析命令行参数，无参数时显示菜单选择服务器
// cfg: 完整的配置结构体，用于保存新添加的服务器
// 返回选中的服务器指针
func chooseServer(cfg *lib.Config) *lib.Server {
	args := os.Args[1:]
	// 有参数时，直接使用参数
	if len(args) >= 1 {
		// 单个数字参数：按序号选择服务器
		if n, err := strconv.Atoi(args[0]); err == nil {
			if n >= 1 && n <= len(cfg.Servers) {
				os.Args = os.Args[:1] // 清掉参数，连接失败后下次调用显示菜单
				return &cfg.Servers[n-1]
			}
			// 序号超出范围，显示菜单
			fmt.Printf("没有 %d 号服务器，可选 1-%d\n", n, len(cfg.Servers))
			os.Args = os.Args[:1]
		} else {
			host, port := args[0], "8080"
			if len(args) >= 2 {
				port = args[1]
			}
			charset := ""
			if len(args) >= 3 {
				charset = args[2]
			}
			// 构造临时 Server 返回
			return &lib.Server{Host: host, Port: port, Charset: charset}
		}
	}

	for {
		// 显示菜单（包含添加选项）
		fmt.Println("可用服务器:")
		for i, s := range cfg.Servers {
			fmt.Printf("  %d. %s (%s:%s)\n", i+1, s.Name, s.Host, s.Port)
		}
		fmt.Println("  +/- 添加删除服务器")

		var choice string
		fmt.Print("请选择: ")
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
	name := readLine()

	fmt.Print("请输入服务器IP或域名: ")
	host := readLine()

	fmt.Print("请输入服务器端口: ")
	port := readLine()

	fmt.Print("请输入服务器编码 (gb/big5，回车跳过): ")
	charset := readLine()

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

// chooseAccount 选择或添加游戏角色
func chooseAccount(s *lib.Server, cfg *lib.Config) *lib.Account {
	for {
		// 无账号时直接输入角色名
		if len(s.Accounts) == 0 {
			fmt.Print("请输入角色名: ")
			name := readLine()
			if name == "" {
				continue
			}
			a := &lib.Account{Username: name}
			fmt.Print("自动登录命令 (可选，回车跳过): ")
			if cmd := readLine(); cmd != "" {
				a.Cmd = cmd
			}
			s.Accounts = append(s.Accounts, a)
			if err := lib.SaveConfig(cfg); err != nil {
				fmt.Printf("保存失败：%v\n", err)
			}
			return a
		}

		fmt.Println("可用角色:")
		for i, a := range s.Accounts {
			cmd := ""
			if a.Cmd != "" {
				cmd = " [自动登录]"
			}
			fmt.Printf("  %d. %s%s\n", i+1, a.Username, cmd)
		}
		fmt.Println("  +/- 添加/删除角色")
		fmt.Println("  e  编辑角色")
		fmt.Print("请选择: ")

		choice := readLine()
		if choice == "+" {
			fmt.Print("请输入角色名: ")
			name := readLine()
			if name == "" {
				continue
			}
			a := &lib.Account{Username: name}
			fmt.Print("自动登录命令 (可选，回车跳过): ")
			if cmd := readLine(); cmd != "" {
				a.Cmd = cmd
			}
			s.Accounts = append(s.Accounts, a)
			if err := lib.SaveConfig(cfg); err != nil {
				fmt.Printf("保存失败：%v\n", err)
			}
			return a
		}

		if choice == "-" {
			fmt.Print("请输入要删除的角色编号: ")
			idx, err := strconv.Atoi(readLine())
			if err == nil && idx >= 1 && idx <= len(s.Accounts) {
				s.Accounts = append(s.Accounts[:idx-1], s.Accounts[idx:]...)
				lib.SaveConfig(cfg)
				fmt.Println("已删除")
			}
			continue
		}

		if choice == "e" {
			fmt.Print("请输入要编辑的角色编号: ")
			idx, err := strconv.Atoi(readLine())
			if err == nil && idx >= 1 && idx <= len(s.Accounts) {
				a := s.Accounts[idx-1]
				fmt.Printf("自动登录命令 (%s): ", a.Cmd)
				if cmd := readLine(); cmd != "" {
					a.Cmd = cmd
				}
				lib.SaveConfig(cfg)
				fmt.Println("已更新")
			}
			continue
		}

		if n, err := strconv.Atoi(choice); err == nil && n >= 1 && n <= len(s.Accounts) {
			return s.Accounts[n-1]
		}
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
		account := chooseAccount(s, cfg)

		c, err := lib.NewClient(cfg, s, account, lib.LSRC)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

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
