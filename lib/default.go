package lib

// DefaultConfig 返回默认配置
// 注意：API 密钥等敏感信息不包含默认值，必须由用户配置
func DefaultConfig() *Config {
	return &Config{
		Language: "cn",
		Engine:   "baidu",
		DeepSeek: DeepSeekConfig{
			APIKey: "", // 必须由用户配置
		},
		Baidu: BaiduConfig{
			AppID:  "", // 必须由用户配置
			Secret: "", // 必须由用户配置
		},
		Kilo: KiloConfig{
			APIKey: "", // 必须由用户配置
			Model:  "kilo-auto/free",
		},
		Servers: []Server{
			{
				Name: "T2TMUD",
				Host: "t2tmud.org",
				Port: "8080",
			},
			{
				Name: "Aardwolf",
				Host: "aardmud.org",
				Port: "4000",
			},
		},
	}
}
