package test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"zmud/lib"
)

func getTranslator(t *testing.T) *lib.Translator {
	cfg, err := lib.LoadConfig()
	if err != nil {
		t.Skipf("加载配置失败: %v", err)
		return nil
	}
	return lib.NewTranslator(cfg)
}

func TestTranslatorTranslate(t *testing.T) {
	tr := getTranslator(t)
	if tr == nil {
		return
	}
	result, err := tr.Translate("hello")
	if err != nil {
		t.Errorf("翻译失败: %v", err)
		return
	}
	t.Logf("hello -> %s", result)
}

func TestTranslatorTranslateBatch(t *testing.T) {
	tr := getTranslator(t)
	if tr == nil {
		return
	}
	results, err := tr.TranslateBatch([]string{"hello", "world"})
	if err != nil {
		t.Errorf("批量翻译失败: %v", err)
		return
	}
	t.Logf("%v", results)
}

func TestTranslatorTranslateBatchExceedLimit(t *testing.T) {
	tr := getTranslator(t)
	if tr == nil {
		return
	}
	_, err := tr.TranslateBatch(make([]string, 21))
	if err == nil {
		t.Error("expected error when exceeding limit")
	}
	t.Logf("%v", err)
}

func TestTranslatorAsk(t *testing.T) {
	tr := getTranslator(t)
	if tr == nil {
		return
	}
	result, err := tr.Ask("hello")
	if err != nil {
		t.Errorf("提问失败: %v", err)
		return
	}
	t.Logf("hello -> %s", result)
}

func TestTranslatorTranslateBatchCache(t *testing.T) {
	tr := getTranslator(t)
	if tr == nil {
		return
	}

	rand.Seed(time.Now().UnixNano())
	samples := make([]string, 5)
	for i := range samples {
		samples[i] = fmt.Sprintf("test %d", rand.Intn(10000))
	}

	start1 := time.Now()
	results1, err := tr.TranslateBatch(samples)
	time1 := time.Since(start1)
	if err != nil {
		t.Fatalf("第一次翻译失败: %v", err)
	}

	start2 := time.Now()
	results2, err := tr.TranslateBatch(samples)
	time2 := time.Since(start2)
	if err != nil {
		t.Fatalf("第二次翻译失败: %v", err)
	}

	for i := range results1 {
		if results1[i] != results2[i] {
			t.Errorf("结果不一致: %s vs %s", results1[i], results2[i])
		}
	}

	t.Logf("第一次: %v, 第二次: %v", time1, time2)
	if time2 < time1 {
		t.Logf("缓存生效，第二次更快")
	}
}
