package lib

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 表示整个应用的配置
type Config struct {
	Language string         `yaml:"language"` // 界面语言，如 "cn" 表示中文
	Engine   string         `yaml:"engine"`   // 翻译引擎："baidu"、"deepseek"、"kilo" 或 ""（空值表示禁用翻译）
	DeepSeek DeepSeekConfig `yaml:"deepseek"` // DeepSeek 翻译配置
	Baidu    BaiduConfig    `yaml:"baidu"`    // 百度翻译配置
	Kilo     KiloConfig     `yaml:"kilo"`     // Kilo 翻译配置
	Servers  []Server       `yaml:"servers"`  // 服务器列表
}

// DeepSeekConfig 包含 DeepSeek 翻译 API 的配置
type DeepSeekConfig struct {
	APIKey string `yaml:"api_key"` // DeepSeek API 密钥，必须配置
}

// KiloConfig 包含 Kilo AI Gateway 的配置
type KiloConfig struct {
	APIKey string `yaml:"api_key"` // Kilo API 密钥，必须配置
	Model  string `yaml:"model"`   // 模型名称，可选
	Proxy  string `yaml:"proxy"`   // 代理地址，如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080
}

// BaiduConfig 包含百度翻译 API 的配置
type BaiduConfig struct {
	AppID  string `yaml:"app_id"` // 百度翻译应用 ID，必须配置
	Secret string `yaml:"secret"` // 百度翻译密钥，必须配置
}

// Server 表示一个 MUD 服务器
type Server struct {
	Name string `yaml:"name"` // 服务器显示名称
	Host string `yaml:"host"` // 服务器主机地址
	Port string `yaml:"port"` // 服务器端口号
}

// LoadConfig 从 ~/.zmud/setting.yaml 加载配置，如果文件不存在则返回默认配置
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录失败：%w", err)
	}

	configPath := filepath.Join(home, ".zmud", "setting.yaml")
	config := DefaultConfig()

	// 如果配置文件不存在，直接返回默认配置（后续会由 EnsureConfig 处理）
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败：%w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析 YAML 配置失败：%w", err)
	}

	return config, nil
}

// SaveConfig 将配置保存到 ~/.zmud/setting.yaml
func SaveConfig(config *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户主目录失败：%w", err)
	}

	zmudDir := filepath.Join(home, ".zmud")
	if err := os.MkdirAll(zmudDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	configPath := filepath.Join(zmudDir, "setting.yaml")
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化 YAML 配置失败：%w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

// EnsureConfig 确保配置完整，如果不完整则启动交互式配置向导
// 返回 true 表示配置已完整（或用户完成了配置），false 表示用户取消配置
func EnsureConfig(config *Config) bool {
	// 检查当前引擎配置是否完整
	if !isConfigComplete(config) {
		engine := InputEngine(config)
		config.Engine = engine // 设置为空值表示禁用翻译
		SaveConfig(config)
	}
	return true
}

// isConfigComplete 检查当前引擎的配置是否完整
func isConfigComplete(config *Config) bool {
	if config.Engine == "" {
		return true // 空值表示禁用翻译，不需要配置
	}

	switch config.Engine {
	case "baidu":
		return config.Baidu.AppID != "" && config.Baidu.Secret != ""
	case "deepseek":
		return config.DeepSeek.APIKey != ""
	case "kilo":
		return config.Kilo.APIKey != ""
	default:
		return false // 未知引擎视为不完整
	}
}

// promptEngine 选择翻译引擎，返回用户选择的引擎名称
// 返回值：引擎名称 ("baidu", "deepseek", "kilo", "") 和是否取消
func promptEngine() (string, bool) {
	fmt.Println("请选择翻译引擎：")
	fmt.Println("1. 百度翻译 (需要 AppID 和 Secret)")
	fmt.Println("2. DeepSeek (需要 API Key)")
	fmt.Println("3. Kilo (需要 API Key)")
	fmt.Println("4. 不用翻译")

	var choice int
	for {
		fmt.Printf("选择 [1/4]: ")
		if _, err := fmt.Scanln(&choice); err != nil {
			fmt.Println("请输入有效的数字")
			continue
		}
		if choice < 1 || choice > 4 {
			fmt.Println("请选择 1-4")
			continue
		}
		break
	}

	switch choice {
	case 1:
		return "baidu", false
	case 2:
		return "deepseek", false
	case 3:
		return "kilo", false
	default:
		return "", false
	}
}

// promptBaiduCredentials 提示输入百度翻译凭据
func promptBaiduCredentials(config *Config) {
	fmt.Print("请输入百度翻译 AppID: ")
	fmt.Scanln(&config.Baidu.AppID)

	fmt.Print("请输入百度翻译 Secret: ")
	fmt.Scanln(&config.Baidu.Secret)
}

// promptDeepSeekKey 提示输入 DeepSeek API Key
func promptDeepSeekKey(config *Config) {
	fmt.Print("请输入 DeepSeek API Key: ")
	fmt.Scanln(&config.DeepSeek.APIKey)
}

// promptKiloKey 提示输入 Kilo API Key
func promptKiloKey(config *Config) {
	fmt.Print("请输入 Kilo API Key: ")
	fmt.Scanln(&config.Kilo.APIKey)

	fmt.Print("请输入模型名称 (直接回车使用默认 kilo-auto/free): ")
	fmt.Scanln(&config.Kilo.Model)

	fmt.Print("请输入代理地址 (直接回车不使用代理，如 http://127.0.0.1:7890): ")
	fmt.Scanln(&config.Kilo.Proxy)
}

// 运行交互式配置向导，引导用户配置翻译引擎
// 返回 引擎名称
func InputEngine(config *Config) string {
	engine, _ := promptEngine()
	if engine == "" {
		return ""
	}

	switch engine {
	case "baidu":
		promptBaiduCredentials(config)
	case "deepseek":
		promptDeepSeekKey(config)
	case "kilo":
		promptKiloKey(config)
	}

	// 保存配置
	if err := SaveConfig(config); err != nil {
		fmt.Printf("保存配置失败：%v\n", err)
		return ""
	}

	fmt.Println("配置已保存。")
	return engine
}
