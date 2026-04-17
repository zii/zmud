package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Google 翻译客户端，负责调用 Google Cloud Translation API 进行文本翻译
type Google struct {
	apiKey string     // Google API 密钥
	from   string     // 源语言代码，如 auto(自动识别)、en(英语) 等
	to     string     // 目标语言代码，如 zh(中文)
	hc     *http.Client
}

// NewGoogle 创建 Google 翻译客户端，需要提供 API Key 和代理地址
// apiKey: Google Cloud API 密钥，必须由用户提供
// proxy: 代理地址，如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080，可为空
func NewGoogle(apiKey, proxy string) *Google {
	var proxyURL *url.URL
	if proxy != "" {
		proxyURL, _ = url.Parse(proxy)
	}
	hc := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return &Google{
		apiKey: apiKey,
		from:   "auto",
		to:     "zh",
		hc:     hc,
	}
}

// GoogleRequest Google Translation API 请求结构
type GoogleRequest struct {
	Q      string `json:"q"`      // 待翻译的文本
	Source string `json:"source"` // 源语言
	Target string `json:"target"` // 目标语言
}

// GoogleResponse Google Translation API 响应结构
type GoogleResponse struct {
	Data struct {
		Translations []struct {
			TranslatedText string `json:"translatedText"`
		} `json:"translations"`
	} `json:"data"`
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Translate 翻译文本为中文，返回翻译结果
// src: 待翻译的源文本
// 返回值：翻译后的中文文本和可能的错误信息
func (g *Google) Translate(src string) (string, error) {
	if strings.TrimSpace(src) == "" {
		return "", fmt.Errorf("输入文本不能为空")
	}

	reqBody := GoogleRequest{
		Q:      src,
		Source: g.from,
		Target: g.to,
	}
	body, _ := json.Marshal(reqBody)

	reqURL := fmt.Sprintf("https://translation.googleapis.com/language/translate/v2?key=%s", g.apiKey)
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%w", err)
	}

	// 如果响应为空或非 JSON，直接返回原始内容帮助调试
	if len(respBody) == 0 {
		return "", fmt.Errorf("服务器返回空响应")
	}
	if !strings.HasPrefix(string(respBody), "{") {
		return "", fmt.Errorf("非 JSON 响应 (%d): %s", resp.StatusCode, string(respBody))
	}

	var result GoogleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析 JSON 失败：%w, body=%s", err, string(respBody))
	}

	// 检查 API 返回的错误
	if result.Error.Message != "" {
		return "", fmt.Errorf("API 错误 (%d): %s", result.Error.Code, result.Error.Message)
	}

	if len(result.Data.Translations) == 0 {
		return "", fmt.Errorf("无翻译结果: %s", string(respBody))
	}

	return result.Data.Translations[0].TranslatedText, nil
}

// TranslateBatch 批量翻译文本，最多20条
func (g *Google) TranslateBatch(srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	if len(srcs) > MaxBatchSize {
		return nil, fmt.Errorf("超过最大翻译数量限制20条")
	}

	// Google API 支持批量翻译，但需要将文本放入 q 字段（换行分隔）
	var sb strings.Builder
	for i, src := range srcs {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(src)
	}

	reqBody := GoogleRequest{
		Q:      sb.String(),
		Source: g.from,
		Target: g.to,
	}
	body, _ := json.Marshal(reqBody)

	reqURL := fmt.Sprintf("https://translation.googleapis.com/language/translate/v2?key=%s", g.apiKey)
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	var result GoogleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败：%w, body=%s", err, string(respBody))
	}

	// 检查 API 返回的错误
	if result.Error.Message != "" {
		return nil, fmt.Errorf("API 错误 (%d): %s", result.Error.Code, result.Error.Message)
	}

	if len(result.Data.Translations) == 0 {
		return nil, fmt.Errorf("无翻译结果: %s", string(respBody))
	}

	// 解析换行分隔的翻译结果
	translations := strings.Split(result.Data.Translations[0].TranslatedText, "\n")
	return translations, nil
}