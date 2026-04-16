package api

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
)

// 批量翻译最大数量
const MaxBatchSize = 20

// 百度翻译配置
// 注意：AppID 和 Secret 必须由用户通过配置文件提供

// 百度翻译客户端，负责调用百度翻译 API 进行文本翻译
type Baidu struct {
	appID  string // 百度翻译 AppID，用于标识应用身份
	secret string // 百度翻译密钥，用于生成请求签名
	from   string // 源语言代码，如 auto(自动识别)、en(英语) 等
	to     string // 目标语言代码，如 zh(中文)
}

// NewBaidu 创建百度翻译客户端，需要提供 AppID 和 Secret
// appID: 百度翻译应用 ID，必须由用户提供
// secret: 百度翻译密钥，必须由用户提供
func NewBaidu(appID, secret string) *Baidu {
	return &Baidu{
		appID:  appID,
		secret: secret,
		from:   "auto",
		to:     "zh",
	}
}

// 生成请求签名，使用 md5(appid+q+salt+key)
func (b *Baidu) sign(q string, salt string) string {
	str := b.appID + q + salt + b.secret
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

// TranslationResponse 百度翻译 API 响应结构
type TranslationResponse struct {
	FROM   string `json:"from"`
	TO     string `json:"to"`
	LogID  int64  `json:"log_id"`
	Result []struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"trans_result"`
	ErrorMsg  string `json:"error_msg"`
	ErrorCode string `json:"error_code"`
}

// Translate 翻译文本为中文，返回翻译结果
// src: 待翻译的源文本
// 返回值：翻译后的中文文本和可能的错误信息
func (b *Baidu) Translate(src string) (string, error) {
	if strings.TrimSpace(src) == "" {
		return "", fmt.Errorf("输入文本不能为空")
	}

	salt := rand.Intn(1000000000)
	sign := b.sign(src, fmt.Sprint(salt))

	reqURL := "https://fanyi-api.baidu.com/api/trans/vip/translate"
	formData := fmt.Sprintf("q=%s&from=%s&to=%s&appid=%s&salt=%d&sign=%s",
		url.QueryEscape(src), b.from, b.to, b.appID, salt, sign)

	resp, err := http.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%w", err)
	}

	// 如果响应为空或非 JSON，直接返回原始内容帮助调试
	if len(body) == 0 {
		return "", fmt.Errorf("服务器返回空响应")
	}
	if !strings.HasPrefix(string(body), "{") {
		return "", fmt.Errorf("非 JSON 响应 (%d): %s", resp.StatusCode, string(body))
	}

	var result TranslationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析 JSON 失败：%w, body=%s", err, string(body))
	}

	if result.ErrorMsg != "" {
		return "", fmt.Errorf("API 错误：%s", result.ErrorMsg)
	}

	var sb strings.Builder
	for i, t := range result.Result {
		if i > 0 {
			sb.WriteString("\r\n")
		}
		sb.WriteString(t.Dst)
	}

	return sb.String(), nil
}

// TranslateBatch 批量翻译文本，最多20条
func (b *Baidu) TranslateBatch(srcs []string) ([]string, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	if len(srcs) > MaxBatchSize {
		return nil, fmt.Errorf("超过最大翻译数量限制20条")
	}

	var sb strings.Builder
	for i, src := range srcs {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(src)
	}

	salt := rand.Intn(1000000000)
	sign := b.sign(sb.String(), fmt.Sprint(salt))

	reqURL := "https://fanyi-api.baidu.com/api/trans/vip/translate"
	formData := fmt.Sprintf("q=%s&from=%s&to=%s&appid=%s&salt=%d&sign=%s",
		url.QueryEscape(sb.String()), b.from, b.to, b.appID, salt, sign)

	resp, err := http.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(formData))
	if err != nil {
		return nil, fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}

	var result TranslationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败：%w", err)
	}

	if result.ErrorMsg != "" {
		return nil, fmt.Errorf("API 错误：%s", result.ErrorMsg)
	}

	var results []string
	for _, t := range result.Result {
		results = append(results, t.Dst)
	}

	return results, nil
}
