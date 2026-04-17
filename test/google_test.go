package test

import (
	"testing"

	"zmud/api"
	"zmud/lib"
)

func getGoogle(t *testing.T) *api.Google {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
		return nil
	}
	if cfg.Google.APIKey == "" {
		t.Skip("Google API Key 未配置")
		return nil
	}
	return api.NewGoogle(cfg.Google.APIKey, cfg.Google.Proxy)
}

func TestGoogleTranslate(t *testing.T) {
	g := getGoogle(t)
	if g == nil {
		return
	}
	result, err := g.Translate("hello")
	if err != nil {
		t.Skipf("API 调用失败: %v", err)
	}
	if result == "" {
		t.Error("Translate returned empty string")
	}
	t.Logf("hello -> %s", result)
}

func TestGoogleTranslateSpecialChars(t *testing.T) {
	g := getGoogle(t)
	if g == nil {
		return
	}
	testCases := []string{
		"Your name?",
		"Hello world!",
		"测试中文翻译",
	}
	for _, src := range testCases {
		result, err := g.Translate(src)
		if err != nil {
			t.Skipf("翻译失败: %v (%s)", err, src)
			continue
		}
		if result == "" {
			t.Errorf("Translate returned empty string (%s)", src)
			continue
		}
		t.Logf("%s -> %s", src, result)
	}
}

func TestGoogleTranslateBatch(t *testing.T) {
	g := getGoogle(t)
	if g == nil {
		return
	}
	results, err := g.TranslateBatch([]string{"hello", "world"})
	if err != nil {
		t.Skipf("API 调用失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	t.Logf("%v", results)
}

func TestGoogleTranslateBatchExceedLimit(t *testing.T) {
	g := getGoogle(t)
	if g == nil {
		t.Skip("Google 未配置")
		return
	}
	_, err := g.TranslateBatch(make([]string, 21))
	if err == nil {
		t.Error("expected error when exceeding limit")
	}
	t.Logf("%v", err)
}
