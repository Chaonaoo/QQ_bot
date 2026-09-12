package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message 对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client LLM API 客户端（兼容 OpenAI 格式）
type Client struct {
	baseURL          string
	apiKey           string
	model            string
	maxTokens        int
	temperature      float64
	topP             float64
	presencePenalty  float64
	frequencyPenalty float64
	disableThinking  bool
	httpClient       *http.Client
}

// ClientOptions LLM 客户端初始化参数
type ClientOptions struct {
	BaseURL          string
	APIKey           string
	Model            string
	MaxTokens        int
	Temperature      float64
	TopP             float64
	PresencePenalty  float64
	FrequencyPenalty float64
	DisableThinking  bool
}

// NewClient 创建 LLM 客户端
func NewClient(opts ClientOptions) *Client {
	return &Client{
		baseURL:          opts.BaseURL,
		apiKey:           opts.APIKey,
		model:            opts.Model,
		maxTokens:        opts.MaxTokens,
		temperature:      opts.Temperature,
		topP:             opts.TopP,
		presencePenalty:  opts.PresencePenalty,
		frequencyPenalty: opts.FrequencyPenalty,
		disableThinking:  opts.DisableThinking,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// thinkingControl DeepSeek 思考模式开关（type: "enabled"/"disabled"）
type thinkingControl struct {
	Type string `json:"type"`
}

// chatRequest OpenAI 兼容请求体
type chatRequest struct {
	Model            string           `json:"model"`
	Messages         []Message        `json:"messages"`
	MaxTokens        int              `json:"max_tokens"`
	Temperature      float64          `json:"temperature"`
	TopP             float64          `json:"top_p,omitempty"`
	PresencePenalty  float64          `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64          `json:"frequency_penalty,omitempty"`
	Thinking         *thinkingControl `json:"thinking,omitempty"`
}

// chatResponse OpenAI 兼容响应体
type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Chat 发送对话请求，返回模型回复文本
func (c *Client) Chat(systemPrompt string, messages []Message) (string, error) {
	return c.chat(systemPrompt, messages, c.temperature, c.presencePenalty, c.frequencyPenalty)
}

// ChatDiverse 用更高的随机性和更强的重复惩罚重新生成，
// 用于打破「来来回回就那几句」的复读循环（重复检测触发时调用）
func (c *Client) ChatDiverse(systemPrompt string, messages []Message) (string, error) {
	temp := c.temperature + 0.15
	if temp > 1.3 {
		temp = 1.3
	}
	presence := c.presencePenalty + 0.5
	if presence > 2.0 {
		presence = 2.0
	}
	freq := c.frequencyPenalty + 0.4
	if freq > 2.0 {
		freq = 2.0
	}
	return c.chat(systemPrompt, messages, temp, presence, freq)
}

// chat 内部实现：按给定的采样参数发起一次对话请求
func (c *Client) chat(systemPrompt string, messages []Message, temperature, presencePenalty, frequencyPenalty float64) (string, error) {
	// 组装消息列表：系统提示 + 对话上下文
	allMessages := make([]Message, 0, len(messages)+1)
	allMessages = append(allMessages, Message{
		Role:    "system",
		Content: systemPrompt,
	})
	allMessages = append(allMessages, messages...)

	reqBody := chatRequest{
		Model:            c.model,
		Messages:         allMessages,
		MaxTokens:        c.maxTokens,
		Temperature:      temperature,
		TopP:             c.topP,
		PresencePenalty:  presencePenalty,
		FrequencyPenalty: frequencyPenalty,
	}
	// deepseek-flash/v4-pro 默认思考模式会拖慢回复、低 max_tokens 时 content 为空，
	// 且思考模式下 temperature/presence/frequency 全部失效，故按需关闭
	if c.disableThinking {
		reqBody.Thinking = &thinkingControl{Type: "disabled"}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 LLM API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API 返回状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("LLM API 错误: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM API 未返回任何结果")
	}

	return chatResp.Choices[0].Message.Content, nil
}
