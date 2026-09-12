package chat

import (
	"sync"

	"qq-bot/internal/llm"
)

// ContextMessage 上下文消息
type ContextMessage struct {
	Sender  string `json:"sender"`
	Content string `json:"content"`
}

// ContextManager 群聊上下文管理（滑动窗口）
type ContextManager struct {
	mu       sync.RWMutex
	groups   map[string][]ContextMessage
	maxSize  int
}

// NewContextManager 创建上下文管理器
func NewContextManager(maxSize int) *ContextManager {
	return &ContextManager{
		groups:  make(map[string][]ContextMessage),
		maxSize: maxSize,
	}
}

// Add 添加一条消息到群聊上下文
func (cm *ContextManager) Add(groupID string, msg ContextMessage) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.groups[groupID] = append(cm.groups[groupID], msg)

	// 滑动窗口：超出最大长度时移除最早的消息
	if len(cm.groups[groupID]) > cm.maxSize {
		cm.groups[groupID] = cm.groups[groupID][len(cm.groups[groupID])-cm.maxSize:]
	}
}

// Get 获取指定群的最近 N 条上下文消息（转换为 LLM 消息格式）
func (cm *ContextManager) Get(groupID string, limit int) []llm.Message {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	msgs := cm.groups[groupID]
	if len(msgs) == 0 {
		return nil
	}

	if limit > len(msgs) {
		limit = len(msgs)
	}

	// 取最后 limit 条
	start := len(msgs) - limit
	var result []llm.Message
	for _, m := range msgs[start:] {
		result = append(result, llm.Message{
			Role:    m.Sender,
			Content: m.Content,
		})
	}
	return result
}

// Clear 清空指定群的上下文
func (cm *ContextManager) Clear(groupID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.groups, groupID)
}
