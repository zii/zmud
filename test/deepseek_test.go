package test

import (
	"testing"

	"zmud/api"
	"zmud/lib"
)

func getDeepSeek(t *testing.T) *api.DeepSeek {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
		return nil
	}
	if cfg.DeepSeek.APIKey == "" {
		t.Skip("DeepSeek APIKey 未配置")
		return nil
	}
	return api.NewDeepSeek(cfg.DeepSeek.APIKey)
}

func TestDeepSeekTranslate(t *testing.T) {
	d := getDeepSeek(t)
	if d == nil {
		return
	}
	result, err := d.Translate("hello")
	if err != nil {
		t.Skipf("API 调用失败: %v", err)
	}
	if result == "" {
		t.Error("Translate returned empty string")
	}
	t.Logf("hello -> %s", result)
}

func TestDeepSeekTranslateSpecialChars(t *testing.T) {
	d := getDeepSeek(t)
	if d == nil {
		return
	}
	testCases := []string{
		"Your name?",
		"Hello world!",
		"测试中文翻译",
	}
	for _, src := range testCases {
		result, err := d.Translate(src)
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

func TestDeepSeekTranslateBatch(t *testing.T) {
	d := getDeepSeek(t)
	if d == nil {
		return
	}
	results, err := d.TranslateBatch([]string{"hello", "world"})
	if err != nil {
		t.Skipf("API 调用失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	t.Logf("%v", results)
}

func TestDeepSeekTranslateBatchExceedLimit(t *testing.T) {
	d := getDeepSeek(t)
	if d == nil {
		t.Skip("DeepSeek 未配置")
		return
	}
	_, err := d.TranslateBatch(make([]string, 21))
	if err == nil {
		t.Error("expected error when exceeding limit")
	}
	t.Logf("%v", err)
}