package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeepSeek 配置
const deepseekAPIURL = "https://api.deepseek.com/v1/chat/completions"
const deepseekModel = "deepseek-chat"

// DeepSeek 翻译客户端，负责调用 DeepSeek API 进行文本翻译
type DeepSeek struct {
	apiKey string // DeepSeek API 密钥
	model  string // 模型名称
	hc     *http.Client
}

// NewDeepSeek 创建 DeepSeek 翻译客户端，需要提供 API 密钥
// apiKey: DeepSeek API 密钥，必须由用户提供
func NewDeepSeek(apiKey string) *DeepSeek {
	hc := &http.Client{
		Timeout: 10 * time.Second, // 记得加超时，默认是没有超时的！
		Transport: &http.Transport{
			// 允许每个 Host 保留更多空闲连接，大幅提升高并发下的复用率
			MaxIdleConnsPerHost: 100,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	return &DeepSeek{
		apiKey: apiKey,
		model:  deepseekModel,
		hc:     hc,
	}
}

// Translate 翻译文本为中文，返回翻译结果
func (d *DeepSeek) Translate(src string) (string, error) {
	reqBody := map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个Mud翻译助手，请将用户输入的文本翻译成中文，直接输出翻译结果，不要解释。"},
			{"role": "user", "content": src},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", deepseekAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.hc.Do(req)
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
		return "", fmt.Errorf("解析响应失败：%w", err)
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
func (d *DeepSeek) TranslateBatch(srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	if len(srcs) > MaxBatchSize {
		return nil, fmt.Errorf("超过最大翻译数量限制20条")
	}

	srcJSON, _ := json.Marshal(srcs)
	prompt := fmt.Sprintf("将以下原文翻译成中文，直接返回 JSON 数组，不要其他内容。%s", srcJSON)

	reqBody := map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个Mud翻译助手，请将用户输入的文本翻译成中文，直接输出翻译结果，不要解释。"},
			{"role": "user", "content": prompt},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", deepseekAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.hc.Do(req)
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

// Ask 向 DeepSeek AI 发送问答请求，返回 AI 的回答
// src: 用户的问题
// 返回 AI 的回答内容
func (d *DeepSeek) Ask(src string) (string, error) {
	reqBody := map[string]any{
		"model": d.model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个 Mud 游戏助手，帮助玩家解答游戏相关问题"},
			{"role": "user", "content": src},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", deepseekAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.hc.Do(req)
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
		return "", fmt.Errorf("解析响应失败：%w", err)
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("API 错误：%s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无问答结果")
	}

	return result.Choices[0].Message.Content, nil
}
