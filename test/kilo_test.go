package test

import (
	"testing"

	"zmud/api"
	"zmud/lib"
)

func TestKiloAsk(t *testing.T) {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
	}
	if cfg.Kilo.APIKey == "" {
		t.Skip("Kilo API 密钥未配置")
	}

	k := api.NewKilo(cfg.Kilo.APIKey, cfg.Kilo.Model, cfg.Kilo.Proxy)
	result, err := k.Ask("hello")
	if err != nil {
		t.Errorf("API 调用失败: %v", err)
	}
	t.Logf("hello -> %s", result)
}

func TestKiloTranslate(t *testing.T) {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
	}
	if cfg.Kilo.APIKey == "" {
		t.Skip("Kilo API 密钥未配置")
	}

	k := api.NewKilo(cfg.Kilo.APIKey, cfg.Kilo.Model, cfg.Kilo.Proxy)
	result, err := k.Translate("hello")
	if err != nil {
		t.Errorf("API 调用失败: %v", err)
	}
	t.Logf("hello -> %s", result)
}

func TestKiloTranslateBatch(t *testing.T) {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
	}
	if cfg.Kilo.APIKey == "" {
		t.Skip("Kilo API 密钥未配置")
	}

	k := api.NewKilo(cfg.Kilo.APIKey, cfg.Kilo.Model, cfg.Kilo.Proxy)
	results, err := k.TranslateBatch([]string{"hello", "world"})
	if err != nil {
		t.Errorf("API 调用失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	t.Logf("%v", results)
}

func TestKiloTranslateBatchExceedLimit(t *testing.T) {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
	}
	if cfg.Kilo.APIKey == "" {
		t.Skip("Kilo API 密钥未配置")
	}

	k := api.NewKilo(cfg.Kilo.APIKey, cfg.Kilo.Model, cfg.Kilo.Proxy)
	_, err = k.TranslateBatch(make([]string, 21))
	if err == nil {
		t.Error("expected error when exceeding limit")
	}
	t.Logf("%v", err)
}