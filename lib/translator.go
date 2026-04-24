package lib

import (
	"fmt"
	"os"
	"path/filepath"

	"zmud/api"
	"zmud/lib/lmdb"

	"github.com/vkudryk/rapidhash-go"
)

// 翻译器，将文本翻译为中文，支持累积多行后统一翻译
type Translator struct {
	db       *lmdb.DB
	Config   *Config
	baidu    *api.Baidu
	deepseek *api.DeepSeek
	kilo     *api.Kilo
	google   *api.Google
}

// NewTranslator 创建翻译器实例，缓存文件放在 ~/.zmud/cache.db
// cfg: 翻译配置，包含引擎和 API 密钥，不能为 nil
func NewTranslator(cfg *Config) *Translator {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	zmudDir := filepath.Join(home, ".zmud")
	os.MkdirAll(zmudDir, 0755)
	dbPath := filepath.Join(zmudDir, "cache.db")
	db, err := lmdb.Open(dbPath)
	if err != nil {
		panic(err)
	}
	return &Translator{
		db:       db,
		Config:   cfg,
		baidu:    api.NewBaidu(cfg.Baidu.AppID, cfg.Baidu.Secret),
		deepseek: api.NewDeepSeek(cfg.DeepSeek.APIKey),
		kilo:     api.NewKilo(cfg.Kilo.APIKey, cfg.Kilo.Model, cfg.Kilo.Proxy),
		google:   api.NewGoogle(cfg.Google.APIKey, cfg.Google.Proxy),
	}
}

// SetEngine 切换翻译引擎
func (t *Translator) SetEngine(engine string) {
	t.Config.Engine = engine
	SaveConfig(t.Config)
}

// GetEngine 返回当前引擎名称
func (t *Translator) GetEngine() string {
	return t.Config.Engine
}

// 将 src 翻译为中文，返回翻译结果，优先使用缓存
func (t *Translator) Translate(src string) (string, error) {
	if len(src) == 0 {
		return "", nil
	}
	// 生成缓存 key (rapidhash)
	key := fmt.Sprintf("%x", rapidhash.String(src))

	// 查缓存
	var result string
	t.db.View(func(tx *lmdb.Tx) error {
		result, _ = tx.Get(key)
		return nil
	})
	if result != "" {
		return result, nil
	}

	// 调用 API
	var err error
	engine := t.Config.Engine
	if engine == "" {
		// 空值表示禁用翻译，直接返回原文
		return src, nil
	}
	if engine == "baidu" {
		result, err = t.baidu.Translate(src)
	} else if engine == "deepseek" {
		result, err = t.deepseek.Translate(src)
	} else if engine == "kilo" {
		result, err = t.kilo.Translate(src)
	} else if engine == "google" {
		result, err = t.google.Translate(src)
	} else {
		return "", fmt.Errorf("未知翻译引擎：%s", engine)
	}
	if err != nil {
		return "", err
	}

	// 存缓存
	t.db.Update(func(tx *lmdb.Tx) error {
		tx.Set(key, result, nil)
		return nil
	})
	return result, nil
}

// TranslateBatch 批量翻译，最多20条
func (t *Translator) TranslateBatch(srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	if len(srcs) > api.MaxBatchSize {
		return nil, fmt.Errorf("超过最大翻译数量限制20条")
	}

	engine := t.Config.Engine
	if engine == "" {
		return srcs, nil
	}

	// 先查缓存
	cachedResults := make([]string, len(srcs))
	var missed []int
	for i, src := range srcs {
		key := fmt.Sprintf("%x", rapidhash.String(src))
		var cached string
		t.db.View(func(tx *lmdb.Tx) error {
			cached, _ = tx.Get(key)
			return nil
		})
		if cached != "" {
			cachedResults[i] = cached
		} else {
			missed = append(missed, i)
		}
	}

	// 全部命中缓存
	if len(missed) == 0 {
		return cachedResults, nil
	}

	// 批量翻译未命中的
	var srcsMissed []string
	for _, i := range missed {
		srcsMissed = append(srcsMissed, srcs[i])
	}

	var err error
	var apiResults []string
	if engine == "baidu" {
		apiResults, err = t.baidu.TranslateBatch(srcsMissed)
	} else if engine == "deepseek" {
		apiResults, err = t.deepseek.TranslateBatch(srcsMissed)
	} else if engine == "kilo" {
		apiResults, err = t.kilo.TranslateBatch(srcsMissed)
	} else if engine == "google" {
		apiResults, err = t.google.TranslateBatch(srcsMissed)
	} else {
		return nil, fmt.Errorf("未知翻译引擎：%s", engine)
	}

	// 存缓存
	if err == nil {
		t.db.Update(func(tx *lmdb.Tx) error {
			for j, i := range missed {
				if j < len(apiResults) {
					key := fmt.Sprintf("%x", rapidhash.String(srcs[i]))
					tx.Set(key, apiResults[j], nil)
				}
			}
			return nil
		})
	}

	// 合并结果
	var finalResults []string
	for i := range srcs {
		// 先看是否命中缓存
		if cachedResults[i] != "" {
			finalResults = append(finalResults, cachedResults[i])
			continue
		}
		// 找映射
		found := false
		for j, mi := range missed {
			if mi == i {
				if apiResults != nil && j < len(apiResults) {
					finalResults = append(finalResults, apiResults[j])
				}
				found = true
				break
			}
		}
		if !found {
			finalResults = append(finalResults, "")
		}
	}

	return finalResults, err
}

// IsCached 检测 src 是否已被缓存
func (t *Translator) IsCached(src string) bool {
	if len(src) == 0 {
		return false
	}
	key := fmt.Sprintf("%x", rapidhash.String(src))
	var result string
	t.db.View(func(tx *lmdb.Tx) error {
		result, _ = tx.Get(key)
		return nil
	})
	return result != ""
}

// SetConfig 更新翻译器配置
func (t *Translator) SetConfig(cfg *Config) {
	t.Config = cfg
}

// Ask 向 AI 发送问答请求，返回 AI 的回答
// 优先使用 Kilo，如果失败则尝试 DeepSeek
func (t *Translator) Ask(src string) (string, error) {
	// 优先使用 Kilo
	if t.Config.Kilo.APIKey != "" {
		ans, err := t.kilo.Ask(src)
		if err == nil {
			return ans, nil
		} else {
			return "", fmt.Errorf("Kilo: %w", err)
		}
	}
	// Kilo 失败，尝试 DeepSeek
	if t.Config.DeepSeek.APIKey != "" {
		ans, err := t.deepseek.Ask(src)
		if err == nil {
			return ans, nil
		} else {
			return "", fmt.Errorf("Deepseek: %w", err)
		}
	}
	return "", fmt.Errorf("请先配置 Kilo 或 DeepSeek API Key")
}
