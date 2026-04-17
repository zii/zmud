package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Kilo 配置
const (
	kiloAPIURL = "https://api.kilo.ai/api/gateway/chat/completions"
)

// Kilo 翻译客户端
type Kilo struct {
	apiKey string // Kilo API 密钥
	model  string // 模型名称
	hc     *http.Client
}

// NewKilo 创建 Kilo 翻译客户端
// apiKey: Kilo API 密钥，必须由用户提供
// model: 模型名称，默认使用 "kilo-auto/free"
// proxy: 代理地址，如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080
func NewKilo(apiKey, model, proxy string) *Kilo {
	if model == "" {
		model = "kilo-auto/free"
	}
	var proxyURL *url.URL
	if proxy != "" {
		proxyURL, _ = url.Parse(proxy)
	}
	hc := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConnsPerHost: 100,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return &Kilo{
		apiKey: apiKey,
		model:  model,
		hc:     hc,
	}
}

// Translate 翻译文本为中文，返回翻译结果
func (k *Kilo) Translate(src string) (string, error) {
	reqBody := map[string]any{
		"model": k.model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个Mud翻译助手，请将用户输入的文本翻译成中文，直接输出翻译结果，不要解释。"},
			{"role": "user", "content": src},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", kiloAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.apiKey)

	resp, err := k.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败：%w, body: %s", err, string(respBody[:min(100, len(respBody))]))
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("API 错误：%s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无翻译结果")
	}

	return result.Choices[0].Message.Content, nil
}

// TranslateBatch 批量翻译文本，最多20条，返回 JSON 数组
func (k *Kilo) TranslateBatch(srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	if len(srcs) > MaxBatchSize {
		return nil, fmt.Errorf("超过最大翻译数量限制20条")
	}

	srcJSON, _ := json.Marshal(srcs)
	prompt := fmt.Sprintf("将以下原文翻译成中文，直接返回 JSON 数组，不要其他内容。%s", srcJSON)

	reqBody := map[string]any{
		"model": k.model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个Mud翻译助手，请将用户输入的文本翻译成中文，直接输出翻译结果，不要解释。"},
			{"role": "user", "content": prompt},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", kiloAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.apiKey)

	resp, err := k.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败：%w", err)
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("API 错误：%s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("无翻译结果")
	}

	// 解析 JSON 数组
	var translations []string
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &translations); err != nil {
		return nil, fmt.Errorf("解析翻译结果失败：%w", err)
	}

	return translations, nil
}

// Ask 向 Kilo AI 发送问答请求，返回 AI 的回答
// src: 用户的问题
// 返回 AI 的回答内容
func (k *Kilo) Ask(src string) (string, error) {
	reqBody := map[string]any{
		"model": k.model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个 Mud 游戏助手，帮助玩家解答游戏相关问题"},
			{"role": "user", "content": src},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", kiloAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.apiKey)

	resp, err := k.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败：%w, body: %s", err, string(respBody[:min(100, len(respBody))]))
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("API 错误：%s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无问答结果")
	}

	return result.Choices[0].Message.Content, nil
}
