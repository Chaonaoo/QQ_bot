package learning

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"qq-bot/internal/profile"
)

// ============ 数据结构 ============

// Expression 带质量评分的表达
type Expression struct {
	Text        string    `json:"text"`
	Count       int       `json:"count"`        // 出现次数
	Sources     []string  `json:"sources"`      // 来源（不同发送者）
	FirstSeen   time.Time `json:"first_seen"`   // 首次出现
	LastSeen    time.Time `json:"last_seen"`    // 最后出现
	Score       float64   `json:"score"`        // 质量评分 (0-1)
	Layer       string    `json:"layer"`        // "temp", "stable", "core"
	IsContradict bool     `json:"is_contradict"` // 是否与核心画像矛盾
}

// LearningData 存储学习数据
type LearningData struct {
	// 反馈记录
	Feedbacks []Feedback `json:"feedbacks"`
	// 机器人回复历史
	ReplyHistory []ReplyRecord `json:"reply_history"`
	// 学到的表达（带评分）
	Expressions []Expression `json:"expressions"`
	// 话题追踪
	RecentTopics []TopicRecord `json:"recent_topics"`
	// 自我特质（机器人对自己的认知）
	SelfTraits []SelfTrait `json:"self_traits"`
	// 图片记忆（识别过的图片内容）
	ImageMemories []ImageMemory `json:"image_memories"`
	// 核心画像快照（用于矛盾检测）
	CoreCatchphrases []string `json:"core_catchphrases"`
	CoreTopics       []string `json:"core_topics"`
	// 最后更新时间
	LastUpdate time.Time `json:"last_update"`
	// 最后清理时间
	LastCleanup time.Time `json:"last_cleanup"`
}

// Feedback 反馈记录
type Feedback struct {
	Time       time.Time `json:"time"`
	GroupID    string    `json:"group_id"`
	SenderQQ   string    `json:"sender_qq"`
	SenderName string    `json:"sender_name"`
	Content    string    `json:"content"`
	Type       string    `json:"type"` // "negative", "positive", "correction"
	BotReply   string    `json:"bot_reply"`
}

// ReplyRecord 回复记录
type ReplyRecord struct {
	Time    time.Time `json:"time"`
	GroupID string    `json:"group_id"`
	Trigger string    `json:"trigger"`
	Reply   string    `json:"reply"`
	Score   float64   `json:"score"` // 回复质量评分（基于反馈）
}

// TopicRecord 话题记录
type TopicRecord struct {
	Time     time.Time `json:"time"`
	GroupID  string    `json:"group_id"`
	Topic    string    `json:"topic"`
	Keywords []string  `json:"keywords"`
	Heat     float64   `json:"heat"` // 话题热度（随时间衰减）
}

// SelfTrait 自我特质（机器人对自己的认知）
type SelfTrait struct {
	Key         string    `json:"key"`          // 特质关键词（如：小狗狗）
	Description string    `json:"description"`  // 特质描述（如：你是小狗狗）
	Response    string    `json:"response"`     // 被提及时的回应（如：汪汪）
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	CreatedBy   string    `json:"created_by"`   // 创建者
	MentionCount int      `json:"mention_count"` // 被提及次数
}

// ImageMemory 图片记忆（视觉模型识别后的内容）
type ImageMemory struct {
	Time        time.Time `json:"time"`
	GroupID     string    `json:"group_id"`
	SenderQQ    string    `json:"sender_qq"`
	SenderName  string    `json:"sender_name"` // 谁发的图
	URL         string    `json:"url"`         // 图片地址（可能失效）
	Description string    `json:"description"` // 识别出的内容描述
	Keywords    []string  `json:"keywords"`    // 从描述中提取的关键词
	MentionText string    `json:"mention_text"`// 发图时附带的文字
}

// ============ 学习管理器 ============

// LearningManager 学习管理器
type LearningManager struct {
	mu          sync.RWMutex
	data        *LearningData
	dataPath    string
	profilePath string

	// 配置
	config *LearningConfig
}

// LearningConfig 学习配置
type LearningConfig struct {
	// 容量限制
	MaxFeedbacks      int `json:"max_feedbacks"`
	MaxReplyHistory   int `json:"max_reply_history"`
	MaxExpressions    int `json:"max_expressions"`
	MaxTopics         int `json:"max_topics"`

	// 衰减参数
	TempDecayRate     float64 `json:"temp_decay_rate"`     // 临时层衰减率（每天）
	StableDecayRate   float64 `json:"stable_decay_rate"`   // 稳定层衰减率（每天）
	TopicDecayRate    float64 `json:"topic_decay_rate"`    // 话题衰减率（每天）

	// 晋升阈值
	PromoteCount      int     `json:"promote_count"`       // 晋升到稳定层需要的次数
	PromoteSources    int     `json:"promote_sources"`     // 晋升需要的不同来源数
	PromoteScore      float64 `json:"promote_score"`       // 晋升需要的最低分

	// 清理阈值
	CleanupInterval   time.Duration `json:"cleanup_interval"`   // 清理间隔
	MinScore          float64       `json:"min_score"`          // 最低保留分数
	ContradictPenalty float64       `json:"contradict_penalty"` // 矛盾惩罚系数
}

// DefaultLearningConfig 默认配置
func DefaultLearningConfig() *LearningConfig {
	return &LearningConfig{
		MaxFeedbacks:      200,
		MaxReplyHistory:   500,
		MaxExpressions:    100,
		MaxTopics:         50,
		TempDecayRate:     0.15,  // 每天衰减 15%
		StableDecayRate:   0.03,  // 每天衰减 3%
		TopicDecayRate:    0.10,  // 每天衰减 10%
		PromoteCount:      3,
		PromoteSources:    2,
		PromoteScore:      0.6,
		CleanupInterval:   1 * time.Hour,
		MinScore:          0.2,
		ContradictPenalty: 0.5,
	}
}

// NewLearningManager 创建学习管理器
func NewLearningManager(profilePath string, dataDir string) *LearningManager {
	lm := &LearningManager{
		dataPath:    filepath.Join(dataDir, "learning.json"),
		profilePath: profilePath,
		config:      DefaultLearningConfig(),
	}

	lm.load()
	lm.loadCoreProfile()

	// 启动定期清理
	go lm.cleanupLoop()

	return lm
}

// load 加载学习数据
func (lm *LearningManager) load() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	data, err := os.ReadFile(lm.dataPath)
	if err != nil {
		lm.data = &LearningData{
			Feedbacks:        make([]Feedback, 0),
			ReplyHistory:     make([]ReplyRecord, 0),
			Expressions:      make([]Expression, 0),
			RecentTopics:     make([]TopicRecord, 0),
			SelfTraits:       make([]SelfTrait, 0),
			CoreCatchphrases: make([]string, 0),
			CoreTopics:       make([]string, 0),
			LastUpdate:       time.Now(),
			LastCleanup:      time.Now(),
		}
		return
	}

	var ld LearningData
	if err := json.Unmarshal(data, &ld); err != nil {
		lm.data = &LearningData{
			Feedbacks:        make([]Feedback, 0),
			ReplyHistory:     make([]ReplyRecord, 0),
			Expressions:      make([]Expression, 0),
			RecentTopics:     make([]TopicRecord, 0),
			SelfTraits:       make([]SelfTrait, 0),
			CoreCatchphrases: make([]string, 0),
			CoreTopics:       make([]string, 0),
			LastUpdate:       time.Now(),
			LastCleanup:      time.Now(),
		}
		return
	}

	// 确保 SelfTraits 不为 nil
	if ld.SelfTraits == nil {
		ld.SelfTraits = make([]SelfTrait, 0)
	}

	lm.data = &ld
}

// loadCoreProfile 加载核心画像（用于矛盾检测）
func (lm *LearningManager) loadCoreProfile() {
	data, err := os.ReadFile(lm.profilePath)
	if err != nil {
		return
	}

	var p profile.Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return
	}

	lm.mu.Lock()
	lm.data.CoreCatchphrases = p.Style.Catchphrases
	lm.data.CoreTopics = p.Topics
	lm.mu.Unlock()
}

// save 保存学习数据
func (lm *LearningManager) save() error {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	data, err := json.MarshalIndent(lm.data, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(lm.dataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(lm.dataPath, data, 0644)
}

// ============ 自清洁机制 ============

// cleanupLoop 定期清理循环
func (lm *LearningManager) cleanupLoop() {
	ticker := time.NewTicker(lm.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		lm.Cleanup()
	}
}

// Cleanup 执行清理
func (lm *LearningManager) Cleanup() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()

	// 1. 衰减表达分数
	for i := range lm.data.Expressions {
		expr := &lm.data.Expressions[i]
		days := now.Sub(expr.LastSeen).Hours() / 24

		var decayRate float64
		switch expr.Layer {
		case "temp":
			decayRate = lm.config.TempDecayRate
		case "stable":
			decayRate = lm.config.StableDecayRate
		default:
			decayRate = lm.config.TempDecayRate
		}

		// 指数衰减
		expr.Score *= math.Pow(1-decayRate, days)

		// 矛盾惩罚
		if expr.IsContradict {
			expr.Score *= lm.config.ContradictPenalty
		}
	}

	// 2. 衰减话题热度
	for i := range lm.data.RecentTopics {
		topic := &lm.data.RecentTopics[i]
		days := now.Sub(topic.Time).Hours() / 24
		topic.Heat *= math.Pow(1-lm.config.TopicDecayRate, days)
	}

	// 3. 移除低分表达
	var validExpressions []Expression
	for _, expr := range lm.data.Expressions {
		if expr.Score >= lm.config.MinScore {
			validExpressions = append(validExpressions, expr)
		}
	}
	lm.data.Expressions = validExpressions

	// 4. 移除低热度话题
	var validTopics []TopicRecord
	for _, topic := range lm.data.RecentTopics {
		if topic.Heat >= 0.1 {
			validTopics = append(validTopics, topic)
		}
	}
	lm.data.RecentTopics = validTopics

	// 5. 限制数量（按分数排序，保留高分）
	if len(lm.data.Expressions) > lm.config.MaxExpressions {
		sort.Slice(lm.data.Expressions, func(i, j int) bool {
			return lm.data.Expressions[i].Score > lm.data.Expressions[j].Score
		})
		lm.data.Expressions = lm.data.Expressions[:lm.config.MaxExpressions]
	}

	if len(lm.data.RecentTopics) > lm.config.MaxTopics {
		sort.Slice(lm.data.RecentTopics, func(i, j int) bool {
			return lm.data.RecentTopics[i].Heat > lm.data.RecentTopics[j].Heat
		})
		lm.data.RecentTopics = lm.data.RecentTopics[:lm.config.MaxTopics]
	}

	if len(lm.data.Feedbacks) > lm.config.MaxFeedbacks {
		lm.data.Feedbacks = lm.data.Feedbacks[len(lm.data.Feedbacks)-lm.config.MaxFeedbacks:]
	}

	if len(lm.data.ReplyHistory) > lm.config.MaxReplyHistory {
		lm.data.ReplyHistory = lm.data.ReplyHistory[len(lm.data.ReplyHistory)-lm.config.MaxReplyHistory:]
	}

	lm.data.LastCleanup = now
	go lm.save()
}

// ============ 表达学习 ============

// LearnFromMessage 从消息中学习
func (lm *LearningManager) LearnFromMessage(groupID, senderQQ, senderName, content string) []string {
	var learned []string

	// 提取候选表达
	candidates := extractExpressions(content)

	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()

	for _, text := range candidates {
		// 检查是否与核心画像矛盾
		isContradict := lm.checkContradiction(text)

		// 查找是否已存在
		found := false
		for i := range lm.data.Expressions {
			expr := &lm.data.Expressions[i]
			if expr.Text == text {
				// 更新现有表达
				expr.Count++
				expr.LastSeen = now

				// 添加新来源
				sourceExists := false
				for _, s := range expr.Sources {
					if s == senderQQ {
						sourceExists = true
						break
					}
				}
				if !sourceExists {
					expr.Sources = append(expr.Sources, senderQQ)
				}

				// 重新计算分数
				expr.Score = lm.calculateScore(expr)
				expr.IsContradict = isContradict

				// 检查是否晋升
				lm.checkPromotion(expr)

				found = true
				break
			}
		}

		if !found && len(lm.data.Expressions) < lm.config.MaxExpressions*2 {
			// 创建新表达（临时层）
			newExpr := Expression{
				Text:         text,
				Count:        1,
				Sources:      []string{senderQQ},
				FirstSeen:    now,
				LastSeen:     now,
				Score:        0.3, // 初始分数
				Layer:        "temp",
				IsContradict: isContradict,
			}
			newExpr.Score = lm.calculateScore(&newExpr)
			lm.data.Expressions = append(lm.data.Expressions, newExpr)
			learned = append(learned, text)
		}
	}

	lm.data.LastUpdate = now
	go lm.save()

	return learned
}

// calculateScore 计算表达质量分数
func (lm *LearningManager) calculateScore(expr *Expression) float64 {
	// 基础分：出现次数（对数增长，避免刷分）
	countScore := math.Log2(float64(expr.Count) + 1) / 5.0
	if countScore > 1.0 {
		countScore = 1.0
	}

	// 来源多样性分数
	sourceScore := float64(len(expr.Sources)) / 5.0
	if sourceScore > 1.0 {
		sourceScore = 1.0
	}

	// 时间新鲜度分数
	daysSinceLast := time.Since(expr.LastSeen).Hours() / 24
	freshnessScore := math.Exp(-daysSinceLast / 7.0) // 7天半衰期

	// 综合评分
	score := countScore*0.4 + sourceScore*0.3 + freshnessScore*0.3

	// 矛盾惩罚
	if expr.IsContradict {
		score *= lm.config.ContradictPenalty
	}

	return score
}

// checkPromotion 检查是否晋升到稳定层
func (lm *LearningManager) checkPromotion(expr *Expression) {
	if expr.Layer == "temp" &&
		expr.Count >= lm.config.PromoteCount &&
		len(expr.Sources) >= lm.config.PromoteSources &&
		expr.Score >= lm.config.PromoteScore {
		expr.Layer = "stable"
	}
}

// checkContradiction 检查是否与核心画像矛盾
func (lm *LearningManager) checkContradiction(text string) bool {
	// 检查是否与核心口头禅冲突
	for _, catchphrase := range lm.data.CoreCatchphrases {
		// 如果新表达与核心口头禅语义相反（简单实现：包含否定词）
		if strings.Contains(text, "不") && strings.Contains(catchphrase, text[3:]) {
			return true
		}
	}

	// 检查是否包含明显负面词汇（与画像风格不符）
	negativeWords := []string{"讨厌", "不喜欢", "无聊", "烦"}
	positiveCore := []string{"喜欢", "爱", "有趣"}

	hasNegative := false
	for _, w := range negativeWords {
		if strings.Contains(text, w) {
			hasNegative = true
			break
		}
	}

	if hasNegative {
		for _, w := range positiveCore {
			for _, topic := range lm.data.CoreTopics {
				if strings.Contains(topic, w) {
					return true
				}
			}
		}
	}

	return false
}

// ============ 反馈处理 ============

// DetectFeedback 检测反馈
func (lm *LearningManager) DetectFeedback(content string, groupID, senderQQ, senderName string, lastBotReply string) *Feedback {
	feedbackKeywords := map[string]string{
		// 负面反馈
		"不像": "negative", "不对": "negative", "不是这样": "negative",
		"假": "negative", "机器人": "negative", "AI": "negative",
		"人工智障": "negative", "垃圾": "negative", "差劲": "negative",
		// 正面反馈
		"像": "positive", "不错": "positive", "可以": "positive",
		"厉害": "positive", "牛": "positive", "6": "positive",
		// 纠正
		"应该是": "correction", "他一般说": "correction",
		"他会说": "correction", "你该说": "correction",
	}

	contentLower := strings.ToLower(content)

	for keyword, feedbackType := range feedbackKeywords {
		if strings.Contains(contentLower, strings.ToLower(keyword)) {
			fb := &Feedback{
				Time:       time.Now(),
				GroupID:    groupID,
				SenderQQ:   senderQQ,
				SenderName: senderName,
				Content:    content,
				Type:       feedbackType,
				BotReply:   lastBotReply,
			}

			lm.mu.Lock()
			lm.data.Feedbacks = append(lm.data.Feedbacks, *fb)

			// 根据反馈调整相关表达的分数
			lm.adjustScoresByFeedback(feedbackType, lastBotReply)

			lm.data.LastUpdate = time.Now()
			lm.mu.Unlock()

			go lm.save()
			return fb
		}
	}

	return nil
}

// adjustScoresByFeedback 根据反馈调整分数
func (lm *LearningManager) adjustScoresByFeedback(feedbackType string, botReply string) {
	if botReply == "" {
		return
	}

	// 找到回复中使用的表达
	for i := range lm.data.Expressions {
		expr := &lm.data.Expressions[i]
		if strings.Contains(botReply, expr.Text) {
			switch feedbackType {
			case "positive":
				// 正面反馈：增加分数
				expr.Score = math.Min(1.0, expr.Score*1.2)
			case "negative":
				// 负面反馈：降低分数
				expr.Score *= 0.7
			case "correction":
				// 纠正：大幅降低分数
				expr.Score *= 0.4
			}
		}
	}
}

// ============ 回复记录 ============

// RecordReply 记录机器人回复
func (lm *LearningManager) RecordReply(groupID, trigger, reply string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	record := ReplyRecord{
		Time:    time.Now(),
		GroupID: groupID,
		Trigger: trigger,
		Reply:   reply,
		Score:   0.5, // 初始分数
	}

	lm.data.ReplyHistory = append(lm.data.ReplyHistory, record)

	// 限制数量
	if len(lm.data.ReplyHistory) > lm.config.MaxReplyHistory {
		lm.data.ReplyHistory = lm.data.ReplyHistory[len(lm.data.ReplyHistory)-lm.config.MaxReplyHistory:]
	}

	lm.data.LastUpdate = time.Now()
	go lm.save()
}

// ============ 话题追踪 ============

// TrackTopic 追踪话题
func (lm *LearningManager) TrackTopic(groupID, content string) {
	keywords := extractKeywords(content)
	if len(keywords) == 0 {
		return
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()

	// 检查是否是现有话题的延续
	found := false
	for i := range lm.data.RecentTopics {
		topic := &lm.data.RecentTopics[i]
		if topic.GroupID == groupID && hasOverlap(topic.Keywords, keywords) {
			// 更新现有话题
			topic.Time = now
			topic.Heat = math.Min(1.0, topic.Heat+0.2)
			// 合并关键词
			topic.Keywords = mergeKeywords(topic.Keywords, keywords)
			found = true
			break
		}
	}

	if !found {
		// 创建新话题
		topic := TopicRecord{
			Time:     now,
			GroupID:  groupID,
			Topic:    content,
			Keywords: keywords,
			Heat:     0.5, // 初始热度
		}
		lm.data.RecentTopics = append(lm.data.RecentTopics, topic)
	}

	lm.data.LastUpdate = now
	go lm.save()
}

// ============ 查询接口 ============

// GetLearningStats 获取学习统计
func (lm *LearningManager) GetLearningStats() map[string]interface{} {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	tempCount := 0
	stableCount := 0
	contradictCount := 0
	for _, expr := range lm.data.Expressions {
		switch expr.Layer {
		case "temp":
			tempCount++
		case "stable":
			stableCount++
		}
		if expr.IsContradict {
			contradictCount++
		}
	}

	negativeCount := 0
	positiveCount := 0
	correctionCount := 0
	for _, fb := range lm.data.Feedbacks {
		switch fb.Type {
		case "negative":
			negativeCount++
		case "positive":
			positiveCount++
		case "correction":
			correctionCount++
		}
	}

	return map[string]interface{}{
		"total_feedbacks":     len(lm.data.Feedbacks),
		"negative_feedbacks":  negativeCount,
		"positive_feedbacks":  positiveCount,
		"corrections":         correctionCount,
		"reply_history":       len(lm.data.ReplyHistory),
		"total_expressions":   len(lm.data.Expressions),
		"temp_expressions":    tempCount,
		"stable_expressions":  stableCount,
		"contradict_expressions": contradictCount,
		"recent_topics":       len(lm.data.RecentTopics),
		"last_update":         lm.data.LastUpdate,
		"last_cleanup":        lm.data.LastCleanup,
	}
}

// GetLearningContext 获取学习上下文（用于 prompt）
func (lm *LearningManager) GetLearningContext() string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var context strings.Builder

	// 自我特质（最高优先级）
	if len(lm.data.SelfTraits) > 0 {
		context.WriteString("【你的自我认知（别人这样叫你时，你要认同并相应回应）】\n")
		for _, trait := range lm.data.SelfTraits {
			context.WriteString(fmt.Sprintf("- 当有人说\"%s\"时，你认同这个称呼，可以回应：\"%s\"\n", trait.Key, trait.Response))
		}
		context.WriteString("\n")
	}

	// 只使用稳定层的高质量表达
	var stableExpressions []Expression
	for _, expr := range lm.data.Expressions {
		if expr.Layer == "stable" && !expr.IsContradict && expr.Score > 0.5 {
			stableExpressions = append(stableExpressions, expr)
		}
	}

	if len(stableExpressions) > 0 {
		// 按分数排序
		sort.Slice(stableExpressions, func(i, j int) bool {
			return stableExpressions[i].Score > stableExpressions[j].Score
		})

		context.WriteString("【你最近学到的新表达（已验证）】\n")
		limit := 10
		if len(stableExpressions) < limit {
			limit = len(stableExpressions)
		}
		for i := 0; i < limit; i++ {
			context.WriteString(fmt.Sprintf("- %s\n", stableExpressions[i].Text))
		}
		context.WriteString("\n")
	}

	// 添加热门话题
	var hotTopics []TopicRecord
	for _, topic := range lm.data.RecentTopics {
		if topic.Heat > 0.3 {
			hotTopics = append(hotTopics, topic)
		}
	}

	if len(hotTopics) > 0 {
		sort.Slice(hotTopics, func(i, j int) bool {
			return hotTopics[i].Heat > hotTopics[j].Heat
		})

		context.WriteString("【群里最近讨论的话题】\n")
		limit := 5
		if len(hotTopics) < limit {
			limit = len(hotTopics)
		}
		for i := 0; i < limit; i++ {
			context.WriteString(fmt.Sprintf("- %s\n", hotTopics[i].Topic))
		}
		context.WriteString("\n")
	}

	// 负面反馈提醒
	negativeCount := 0
	for _, fb := range lm.data.Feedbacks {
		if fb.Type == "negative" && time.Since(fb.Time) < 24*time.Hour {
			negativeCount++
		}
	}
	if negativeCount > 2 {
		context.WriteString("【注意】最近有人说你说话不像，请更加注意模仿风格，多用你的口头禅。\n\n")
	}

	return context.String()
}

// GetLastReply 获取指定群的最后一条机器人回复
func (lm *LearningManager) GetLastReply(groupID string) string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	for i := len(lm.data.ReplyHistory) - 1; i >= 0; i-- {
		if lm.data.ReplyHistory[i].GroupID == groupID {
			return lm.data.ReplyHistory[i].Reply
		}
	}
	return ""
}

// ShouldAvoidReply 检查是否应该避免类似回复
func (lm *LearningManager) ShouldAvoidReply(reply string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// 检查最近 30 条回复
	start := len(lm.data.ReplyHistory) - 30
	if start < 0 {
		start = 0
	}

	for _, record := range lm.data.ReplyHistory[start:] {
		if similarity(record.Reply, reply) > 0.75 {
			return true
		}
	}

	return false
}

// GetRecentFeedbacks 获取最近反馈
func (lm *LearningManager) GetRecentFeedbacks(limit int) []Feedback {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if limit <= 0 || limit > len(lm.data.Feedbacks) {
		limit = len(lm.data.Feedbacks)
	}

	start := len(lm.data.Feedbacks) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Feedback, limit)
	copy(result, lm.data.Feedbacks[start:])
	return result
}

// GetNewExpressions 获取学到的表达（只返回稳定层）
func (lm *LearningManager) GetNewExpressions() []string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var result []string
	for _, expr := range lm.data.Expressions {
		if expr.Layer == "stable" && !expr.IsContradict {
			result = append(result, expr.Text)
		}
	}
	return result
}

// GetRecentTopics 获取最近话题
func (lm *LearningManager) GetRecentTopics(limit int) []TopicRecord {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// 按热度排序
	sorted := make([]TopicRecord, len(lm.data.RecentTopics))
	copy(sorted, lm.data.RecentTopics)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Heat > sorted[j].Heat
	})

	if limit <= 0 || limit > len(sorted) {
		limit = len(sorted)
	}

	result := make([]TopicRecord, limit)
	copy(result, sorted[:limit])
	return result
}

// ============ 工具函数 ============

func extractExpressions(content string) []string {
	var expressions []string
	runes := []rune(content)

	for i := 0; i < len(runes); i++ {
		for length := 2; length <= 6 && i+length <= len(runes); length++ {
			sub := string(runes[i : i+length])
			hasChinese := false
			for _, r := range sub {
				if r >= 0x4e00 && r <= 0x9fff {
					hasChinese = true
					break
				}
			}
			if hasChinese && !isCommonWord(sub) {
				expressions = append(expressions, sub)
			}
		}
	}

	if len(expressions) > 5 {
		expressions = expressions[:5]
	}

	return expressions
}

// ============ 自我特质管理 ============

// AddSelfTrait 添加自我特质
func (lm *LearningManager) AddSelfTrait(key, description, response, createdBy string) *SelfTrait {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// 检查是否已存在
	for i, trait := range lm.data.SelfTraits {
		if trait.Key == key {
			// 更新现有特质
			lm.data.SelfTraits[i].Description = description
			lm.data.SelfTraits[i].Response = response
			lm.data.LastUpdate = time.Now()
			go lm.save()
			return &lm.data.SelfTraits[i]
		}
	}

	// 添加新特质
	trait := SelfTrait{
		Key:          key,
		Description:  description,
		Response:     response,
		CreatedAt:    time.Now(),
		CreatedBy:    createdBy,
		MentionCount: 0,
	}
	lm.data.SelfTraits = append(lm.data.SelfTraits, trait)
	lm.data.LastUpdate = time.Now()
	go lm.save()

	return &trait
}

// GetSelfTraits 获取所有自我特质
func (lm *LearningManager) GetSelfTraits() []SelfTrait {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make([]SelfTrait, len(lm.data.SelfTraits))
	copy(result, lm.data.SelfTraits)
	return result
}

// RemoveSelfTrait 移除自我特质
func (lm *LearningManager) RemoveSelfTrait(key string) bool {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for i, trait := range lm.data.SelfTraits {
		if trait.Key == key {
			lm.data.SelfTraits = append(lm.data.SelfTraits[:i], lm.data.SelfTraits[i+1:]...)
			lm.data.LastUpdate = time.Now()
			go lm.save()
			return true
		}
	}
	return false
}

// MatchSelfTrait 检查消息是否提及某个自我特质
func (lm *LearningManager) MatchSelfTrait(message string) *SelfTrait {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	msgLower := strings.ToLower(message)
	for i, trait := range lm.data.SelfTraits {
		if strings.Contains(msgLower, strings.ToLower(trait.Key)) {
			// 增加提及次数（异步保存）
			go func(idx int) {
				lm.mu.Lock()
				if idx < len(lm.data.SelfTraits) {
					lm.data.SelfTraits[idx].MentionCount++
					lm.data.LastUpdate = time.Now()
					go lm.save()
				}
				lm.mu.Unlock()
			}(i)
			return &lm.data.SelfTraits[i]
		}
	}
	return nil
}

// GetSelfTraitsContext 获取自我特质的 prompt 上下文
func (lm *LearningManager) GetSelfTraitsContext() string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if len(lm.data.SelfTraits) == 0 {
		return ""
	}

	var context strings.Builder
	context.WriteString("【你的自我认知（别人这样叫你时，你要认同并相应回应）】\n")
	for _, trait := range lm.data.SelfTraits {
		context.WriteString(fmt.Sprintf("- 当有人说\"%s\"时，你认同这个称呼，可以回应：\"%s\"\n", trait.Key, trait.Response))
	}
	context.WriteString("\n")

	return context.String()
}

// ============ 图片记忆管理 ============

// AddImageMemory 添加一条图片记忆
func (lm *LearningManager) AddImageMemory(groupID, senderQQ, senderName, url, description, mentionText string) *ImageMemory {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if strings.TrimSpace(description) == "" {
		return nil
	}

	mem := ImageMemory{
		Time:        time.Now(),
		GroupID:     groupID,
		SenderQQ:    senderQQ,
		SenderName:  senderName,
		URL:         url,
		Description: description,
		Keywords:    extractKeywords(description),
		MentionText: mentionText,
	}
	lm.data.ImageMemories = append(lm.data.ImageMemories, mem)

	// 容量限制：最多保留 200 条
	if len(lm.data.ImageMemories) > 200 {
		lm.data.ImageMemories = lm.data.ImageMemories[len(lm.data.ImageMemories)-200:]
	}

	lm.data.LastUpdate = time.Now()
	// 必须异步保存：save() 内部会获取读锁，而本方法已持有写锁，
	// 同步调用将导致 RWMutex 重入死锁（写锁未释放时再取读锁会永久阻塞），
	// 与 Cleanup/TrackTopic/DetectFeedback/LearnFromMessage 等其他调用点保持一致
	go lm.save()
	return &mem
}

// GetImageMemories 获取指定群的图片记忆（最近 limit 条，按时间倒序）
func (lm *LearningManager) GetImageMemories(groupID string, limit int) []ImageMemory {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var matched []ImageMemory
	for i := len(lm.data.ImageMemories) - 1; i >= 0; i-- {
		m := lm.data.ImageMemories[i]
		if groupID == "" || m.GroupID == groupID {
			matched = append(matched, m)
			if limit > 0 && len(matched) >= limit {
				break
			}
		}
	}
	return matched
}

// GetImageMemoriesContext 构建图片记忆的 prompt 上下文
// 优先返回与当前消息相关的图片记忆，其次返回最近几条
func (lm *LearningManager) GetImageMemoriesContext(groupID, message string) string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if len(lm.data.ImageMemories) == 0 {
		return ""
	}

	msgLower := strings.ToLower(message)

	// 1. 相关图片记忆（当前消息命中了图片关键词，或提到了发图的人）
	var relevant []ImageMemory
	for i := len(lm.data.ImageMemories) - 1; i >= 0; i-- {
		m := lm.data.ImageMemories[i]
		if groupID != "" && m.GroupID != groupID {
			continue
		}
		hit := false
		if strings.Contains(msgLower, strings.ToLower(m.SenderName)) {
			hit = true
		}
		for _, kw := range m.Keywords {
			if len([]rune(kw)) >= 2 && strings.Contains(msgLower, strings.ToLower(kw)) {
				hit = true
				break
			}
		}
		if hit {
			relevant = append(relevant, m)
			if len(relevant) >= 3 {
				break
			}
		}
	}

	// 2. 若没有相关记忆，取最近 2 条作为背景
	useList := relevant
	if len(useList) == 0 {
		count := 0
		for i := len(lm.data.ImageMemories) - 1; i >= 0 && count < 2; i-- {
			m := lm.data.ImageMemories[i]
			if groupID != "" && m.GroupID != groupID {
				continue
			}
			useList = append(useList, m)
			count++
		}
	}

	if len(useList) == 0 {
		return ""
	}

	var context strings.Builder
	if len(relevant) > 0 {
		context.WriteString("【群里之前发过的、与当前对话相关的图片（你可以引用这些内容）】\n")
	} else {
		context.WriteString("【群里最近发过的图片（你可以适当提及）】\n")
	}
	for _, m := range useList {
		when := humanizeTime(m.Time)
		context.WriteString(fmt.Sprintf("- %s %s发过一张图，内容是：%s\n", when, m.SenderName, m.Description))
	}
	context.WriteString("\n")
	return context.String()
}

// humanizeTime 将时间转为相对描述
func humanizeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d小时前", int(d.Hours()))
	default:
		return fmt.Sprintf("%d天前", int(d.Hours()/24))
	}
}

func extractKeywords(content string) []string {
	var keywords []string
	runes := []rune(content)

	for i := 0; i < len(runes); i++ {
		for length := 2; length <= 4 && i+length <= len(runes); length++ {
			sub := string(runes[i : i+length])
			hasChinese := false
			for _, r := range sub {
				if r >= 0x4e00 && r <= 0x9fff {
					hasChinese = true
					break
				}
			}
			if hasChinese {
				keywords = append(keywords, sub)
			}
		}
	}

	seen := make(map[string]bool)
	var unique []string
	for _, kw := range keywords {
		if !seen[kw] {
			seen[kw] = true
			unique = append(unique, kw)
		}
	}

	if len(unique) > 5 {
		unique = unique[:5]
	}

	return unique
}

func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "他": true, "她": true,
	}
	return commonWords[word]
}

func similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	runesA := []rune(a)
	runesB := []rune(b)

	setA := make(map[rune]bool)
	setB := make(map[rune]bool)

	for _, r := range runesA {
		setA[r] = true
	}
	for _, r := range runesB {
		setB[r] = true
	}

	intersection := 0
	for r := range setA {
		if setB[r] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

func hasOverlap(a, b []string) bool {
	setA := make(map[string]bool)
	for _, s := range a {
		setA[s] = true
	}
	for _, s := range b {
		if setA[s] {
			return true
		}
	}
	return false
}

func mergeKeywords(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] && len(result) < 10 {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
