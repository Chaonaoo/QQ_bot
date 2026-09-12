package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用全局配置
type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	LLM      LLMConfig      `yaml:"llm" json:"llm"`
	Vision   VisionConfig   `yaml:"vision" json:"vision"`
	Target   TargetConfig   `yaml:"target" json:"target"`
	Response ResponseConfig `yaml:"response" json:"response"`
	// 运行时风格配置（不保存到 yaml，通过 API 动态调整）
	Style *StyleConfig `yaml:"-" json:"style"`
}

type ServerConfig struct {
	Port int `yaml:"port" json:"port"`
}

type LLMConfig struct {
	Provider    string  `yaml:"provider" json:"provider"`
	APIKey      string  `yaml:"api_key" json:"api_key"`
	Model       string  `yaml:"model" json:"model"`
	BaseURL     string  `yaml:"base_url" json:"base_url"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	// TopP 核采样阈值（0 表示用服务端默认），与 temperature 配合控制发散程度
	TopP float64 `yaml:"top_p" json:"top_p"`
	// PresencePenalty 存在惩罚（-2.0~2.0），正值降低重复、鼓励谈论新内容
	PresencePenalty float64 `yaml:"presence_penalty" json:"presence_penalty"`
	// FrequencyPenalty 频率惩罚（-2.0~2.0），正值按出现频次抑制复读
	FrequencyPenalty float64 `yaml:"frequency_penalty" json:"frequency_penalty"`
	// DisableThinking 关闭思考模式：deepseek-flash/deepseek-v4-pro 默认开启思考，
	// 会导致 content 延迟返回（低 max_tokens 时为空）且 temperature/penalty 参数全部失效；
	// 群聊模仿需快速直白回复，置 true 时向请求体写入 thinking:{type:disabled}
	DisableThinking bool `yaml:"disable_thinking" json:"disable_thinking"`
}

// VisionConfig 视觉模型配置（用于图片识别，需 OpenAI 兼容的多模态接口）
type VisionConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Provider  string `yaml:"provider" json:"provider"`
	APIKey    string `yaml:"api_key" json:"api_key"`
	Model     string `yaml:"model" json:"model"`
	BaseURL   string `yaml:"base_url" json:"base_url"`
	MaxTokens int    `yaml:"max_tokens" json:"max_tokens"`
	// Proxy 可选：HTTP 代理地址（如 http://127.0.0.1:7890），用于访问被墙的 OpenAI 等
	Proxy string `yaml:"proxy" json:"proxy"`
	// DisableThinking 关闭思考模式：当 vision 用 deepseek-flash 等默认开思考的模型时须置 true，
	// 否则识别结果会落到 reasoning_content、content 为空
	DisableThinking bool `yaml:"disable_thinking" json:"disable_thinking"`
}

type TargetConfig struct {
	QQ      string `yaml:"qq" json:"qq"`
	GroupID string `yaml:"group_id" json:"group_id"`
}

type ResponseConfig struct {
	TriggerModes      []string `yaml:"trigger_modes" json:"trigger_modes"`
	TriggerKeywords   []string `yaml:"trigger_keywords" json:"trigger_keywords"`
	RandomProbability float64  `yaml:"random_probability" json:"random_probability"`
	ReplyDelayBase    float64  `yaml:"reply_delay_base" json:"reply_delay_base"`
	MaxContext        int      `yaml:"max_context" json:"max_context"`
}

// StyleConfig 运行时风格配置（可通过命令动态调整）
type StyleConfig struct {
	// 回复长度偏好: "short"(1-10字), "medium"(10-50字), "long"(50+字), 或具体数字
	LengthPreference string `json:"length_preference"`
	// 语气风格: "温柔", "暴躁", "可爱", "高冷", "搞笑", "正经", "傲娇", "中二"
	Tone string `json:"tone"`
	// 额外风格指令（自定义）
	CustomStyle string `json:"custom_style"`
	// 是否使用表情包/颜文字
	UseEmoji bool `json:"use_emoji"`
	// 回复速度倍率 (0.5=慢, 1.0=正常, 2.0=快)
	SpeedMultiplier float64 `json:"speed_multiplier"`
}

// DefaultStyleConfig 默认风格配置
func DefaultStyleConfig() *StyleConfig {
	return &StyleConfig{
		LengthPreference: "medium",
		Tone:             "",
		CustomStyle:      "",
		UseEmoji:         true,
		SpeedMultiplier:  1.0,
	}
}

// Load 从指定路径加载 YAML 配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	return &cfg, nil
}
