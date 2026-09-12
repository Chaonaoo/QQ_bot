package history

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Message 单条聊天记录
type Message struct {
	Time    string `json:"time"`
	Sender  string `json:"sender"`
	QQ      string `json:"qq"`
	Content string `json:"content"`
}

// ParseJSON 解析 JSON 格式的聊天记录
// 支持两种格式：
// 1. 简单数组: [{"time":"...","sender":"...","qq":"...","content":"..."}, ...]
// 2. QQChatExporter: {"metadata":...,"messages":[{"time":"...","sender":{"uin":"...","name":"..."},"type":"text","content":{"text":"..."}},...]}
func ParseJSON(filePath string) ([]Message, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 先尝试解析为简单数组格式
	var simpleMessages []Message
	if err := json.Unmarshal(data, &simpleMessages); err == nil && len(simpleMessages) > 0 {
		return simpleMessages, nil
	}

	// 再尝试解析为 QQChatExporter 格式
	return ParseQQChatExporterData(data)
}

// txt 格式的正则：时间戳 + 昵称(QQ号)
// 例如: 2024-01-01 10:00:00 张三(123456)
var txtHeaderRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\s+(.+?)\((\d+)\)\s*$`,
)

// ParseTXT 解析纯文本格式的聊天记录
// 格式:
// 2024-01-01 10:00:00 张三(123456)
// 消息内容第一行
//
// 2024-01-01 10:01:00 李四(789012)
// 回复内容
func ParseTXT(filePath string) ([]Message, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var messages []Message
	var current *Message
	var contentLines []string

	flushCurrent := func() {
		if current != nil && len(contentLines) > 0 {
			current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
			if current.Content != "" {
				messages = append(messages, *current)
			}
		}
		contentLines = nil
		current = nil
	}

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if matches := txtHeaderRe.FindStringSubmatch(line); matches != nil {
			// 先把上一条消息落盘
			flushCurrent()
			current = &Message{
				Time:   matches[1],
				Sender: matches[2],
				QQ:     matches[3],
			}
		} else if current != nil {
			contentLines = append(contentLines, line)
		}
	}
	// 处理最后一条
	flushCurrent()

	return messages, nil
}

// ============ QQChatExporter 格式解析 ============

// qqExporterRoot QQChatExporter 导出文件的根结构
type qqExporterRoot struct {
	Messages []qqExporterMessage `json:"messages"`
}

type qqExporterMessage struct {
	Time    string `json:"time"`
	Sender  struct {
		Uin  string `json:"uin"`
		Name string `json:"name"`
	} `json:"sender"`
	Type    string `json:"type"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
	System bool `json:"system"`
}

// ParseQQChatExporterData 解析 QQChatExporter 导出的 JSON 数据
func ParseQQChatExporterData(data []byte) ([]Message, error) {
	var root qqExporterRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("解析 QQChatExporter 格式失败: %w", err)
	}

	var messages []Message
	for _, m := range root.Messages {
		// 跳过系统消息和非文本消息
		if m.System {
			continue
		}
		if m.Type != "text" {
			continue
		}
		if m.Content.Text == "" {
			continue
		}

		// 转换时间格式: "2026-05-13T06:36:34.000Z" -> "2026-05-13 14:36:34"
		localTime := convertISOToLocal(m.Time)

		messages = append(messages, Message{
			Time:    localTime,
			Sender:  m.Sender.Name,
			QQ:      m.Sender.Uin,
			Content: m.Content.Text,
		})
	}

	return messages, nil
}

// ParseQQChatExporter 从文件解析 QQChatExporter 格式
func ParseQQChatExporter(filePath string) ([]Message, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return ParseQQChatExporterData(data)
}

// convertISOToLocal 将 ISO 8601 时间转换为本地时间字符串
func convertISOToLocal(isoTime string) string {
	t, err := time.Parse(time.RFC3339, isoTime)
	if err != nil {
		// 尝试其他格式
		t, err = time.Parse("2006-01-02T15:04:05.000Z", isoTime)
		if err != nil {
			return isoTime
		}
	}
	return t.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
}

// FilterByQQ 筛选出指定 QQ 号的消息
func FilterByQQ(messages []Message, targetQQ string) []Message {
	var result []Message
	for _, m := range messages {
		if m.QQ == targetQQ {
			result = append(result, m)
		}
	}
	return result
}

// ActiveHours 统计活跃时段（按小时分布）
func ActiveHours(messages []Message) map[int]int {
	dist := make(map[int]int)
	layout := "2006-01-02 15:04:05"
	for _, m := range messages {
		t, err := time.Parse(layout, m.Time)
		if err != nil {
			continue
		}
		dist[t.Hour()]++
	}
	return dist
}

// AvgLength 计算消息的平均字符长度
func AvgLength(messages []Message) float64 {
	if len(messages) == 0 {
		return 0
	}
	total := 0
	for _, m := range messages {
		total += len([]rune(m.Content))
	}
	return float64(total) / float64(len(messages))
}
