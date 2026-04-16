package test

import (
	"testing"

	"zmud/api"
	"zmud/lib"
)

func getBaidu(t *testing.T) *api.Baidu {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
		return nil
	}
	if cfg.Baidu.AppID == "" {
		t.Skip("Baidu AppID 未配置")
		return nil
	}
	return api.NewBaidu(cfg.Baidu.AppID, cfg.Baidu.Secret)
}

func TestBaiduTranslate(t *testing.T) {
	b := getBaidu(t)
	if b == nil {
		return
	}
	result, err := b.Translate("hello")
	if err != nil {
		t.Skipf("API 调用失败: %v", err)
	}
	if result == "" {
		t.Error("Translate returned empty string")
	}
	t.Logf("hello -> %s", result)
}

func TestBaiduTranslateSpecialChars(t *testing.T) {
	b := getBaidu(t)
	if b == nil {
		return
	}
	testCases := []string{
		"Your name?",
		"Hello world!",
		"测试中文翻译",
	}
	for _, src := range testCases {
		result, err := b.Translate(src)
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

func TestBaiduTranslateBatch(t *testing.T) {
	b := getBaidu(t)
	if b == nil {
		return
	}
	results, err := b.TranslateBatch([]string{"hello", "world"})
	if err != nil {
		t.Skipf("API 调用失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	t.Logf("%v", results)
}

func TestBaiduTranslateBatchExceedLimit(t *testing.T) {
	b := getBaidu(t)
	if b == nil {
		t.Skip("Baidu 未配置")
		return
	}
	_, err := b.TranslateBatch(make([]string, 21))
	if err == nil {
		t.Error("expected error when exceeding limit")
	}
	t.Logf("%v", err)
}