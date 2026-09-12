package llm

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

// VisionClient 视觉模型客户端（兼容 OpenAI 多模态格式，用于图片识别）
type VisionClient struct {
	baseURL         string
	apiKey          string
	model           string
	maxTokens       int
	disableThinking bool
	httpClient      *http.Client
}

// NewVisionClient 创建视觉模型客户端
// proxy 为空时直连；非空时通过该 HTTP 代理访问（如 http://127.0.0.1:7890）
// disableThinking 为 true 时发送 thinking:{type:disabled}（deepseek-flash 等默认开思考的模型必须关闭，否则 content 为空）
func NewVisionClient(baseURL, apiKey, model string, maxTokens int, proxy string, disableThinking bool) *VisionClient {
	if maxTokens <= 0 {
		maxTokens = 500
	}
	httpClient := &http.Client{Timeout: 60 * time.Second}
	if strings.TrimSpace(proxy) != "" {
		if proxyURL, err := url.Parse(proxy); err == nil {
			httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}
	return &VisionClient{
		baseURL:         strings.TrimRight(baseURL, "/"),
		apiKey:          apiKey,
		model:           model,
		maxTokens:       maxTokens,
		disableThinking: disableThinking,
		httpClient:      httpClient,
	}
}

// Enabled 判断视觉客户端是否可用（配置了 key 和 model）
func (c *VisionClient) Enabled() bool {
	return c != nil && c.apiKey != "" && c.model != "" && c.baseURL != ""
}

// visionImageURL 图片地址对象
type visionImageURL struct {
	URL string `json:"url"`
}

// visionContent 多模态消息内容片段（文本或图片）
type visionContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

// visionMessage 多模态消息
type visionMessage struct {
	Role    string          `json:"role"`
	Content []visionContent `json:"content"`
}

// visionRequest OpenAI 兼容的多模态请求体
type visionRequest struct {
	Model     string           `json:"model"`
	Messages  []visionMessage  `json:"messages"`
	MaxTokens int              `json:"max_tokens"`
	Thinking  *thinkingControl `json:"thinking,omitempty"`
}

// DescribeImage 识别单张图片内容，返回简洁的中文文字描述
func (c *VisionClient) DescribeImage(imageURL string, instruction string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("视觉模型未启用")
	}
	if imageURL == "" {
		return "", fmt.Errorf("图片地址为空")
	}

	if instruction == "" {
		instruction = "请用简洁的中文描述这张图片的主要内容，包括画面主体、场景、文字（如有）、以及可能表达的情绪或梗。控制在 60 字以内，直接输出描述，不要加前缀。"
	}

	reqBody := visionRequest{
		Model: c.model,
		Messages: []visionMessage{
			{
				Role: "user",
				Content: []visionContent{
					{Type: "text", Text: instruction},
					{Type: "image_url", ImageURL: &visionImageURL{URL: imageURL}},
				},
			},
		},
		MaxTokens: c.maxTokens,
	}
	// deepseek-flash 作视觉模型时默认思考模式会导致 content 为空，按需关闭
	if c.disableThinking {
		reqBody.Thinking = &thinkingControl{Type: "disabled"}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化视觉请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建视觉请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求视觉 API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取视觉响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("视觉 API 返回状态 %d: %s", resp.StatusCode, string(respBody))
	}

	// 复用文本响应结构（choices[0].message.content 为字符串）
	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("解析视觉响应失败: %w", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("视觉 API 错误: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("视觉 API 未返回任何结果")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}
