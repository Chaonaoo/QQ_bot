package chat

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"qq-bot/internal/config"
	"qq-bot/internal/history"
	"qq-bot/internal/learning"
	"qq-bot/internal/llm"
	"qq-bot/internal/profile"

	"github.com/gin-gonic/gin"
)

// Handler 聊天处理器
type Handler struct {
	llmClient    *llm.Client
	visionClient *llm.VisionClient
	store        *profile.Store
	cfg          *config.Config
	cfgMu        sync.RWMutex
	ctxMgr       *ContextManager
	learner      *learning.LearningManager

	// 运行时风格配置
	style   *config.StyleConfig
	styleMu sync.RWMutex

	// 统计
	totalReceived  atomic.Int64
	totalReplied   atomic.Int64
}

// NewHandler 创建聊天处理器
func NewHandler(llmClient *llm.Client, visionClient *llm.VisionClient, store *profile.Store, cfg *config.Config, learner *learning.LearningManager) *Handler {
	return &Handler{
		llmClient:    llmClient,
		visionClient: visionClient,
		store:        store,
		cfg:          cfg,
		ctxMgr:       NewContextManager(cfg.Response.MaxContext),
		learner:      learner,
		style:        config.DefaultStyleConfig(),
	}
}

// ============ 请求/响应结构体 ============

type ChatRequest struct {
	GroupID    string           `json:"group_id"`
	SenderQQ   string           `json:"sender_qq"`
	SenderName string           `json:"sender_name"`
	Message    string           `json:"message"`
	Context    []ContextMessage `json:"context"`
	ImageURLs  []string         `json:"image_urls"` // 本次消息附带的图片地址
}

type ChatResponse struct {
	Reply string  `json:"reply"`
	Delay float64 `json:"delay"` // 建议延迟（秒）
}

type ImportResponse struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Profile *profile.Profile `json:"profile,omitempty"`
}

type StatsResponse struct {
	TotalReceived int64  `json:"total_received"`
	TotalReplied  int64  `json:"total_replied"`
	TargetQQ      string `json:"target_qq"`
	GroupID       string `json:"group_id"`
	ProfileLoaded bool   `json:"profile_loaded"`
}

// ============ API 处理函数 ============

// ImportHistory 上传聊天记录文件并构建画像（支持多文件）
func (h *Handler) ImportHistory(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传聊天记录文件"})
		return
	}

	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传至少一个聊天记录文件"})
		return
	}

	h.cfgMu.RLock()
	targetQQ := h.cfg.Target.QQ
	h.cfgMu.RUnlock()

	var allMessages []history.Message
	var fileResults []string

	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			continue
		}

		// 保存到 data/history/
		savePath := filepath.Join("data", "history", header.Filename)
		dst, err := os.Create(savePath)
		if err != nil {
			file.Close()
			continue
		}
		io.Copy(dst, file)
		dst.Close()
		file.Close()

		// 根据扩展名选择解析器
		var messages []history.Message
		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".json":
			messages, err = history.ParseJSON(savePath)
		case ".txt":
			messages, err = history.ParseTXT(savePath)
		default:
			fileResults = append(fileResults, fmt.Sprintf("%s: 不支持的格式", header.Filename))
			continue
		}

		if err != nil {
			fileResults = append(fileResults, fmt.Sprintf("%s: 解析失败 (%v)", header.Filename, err))
			continue
		}

		allMessages = append(allMessages, messages...)
		fileResults = append(fileResults, fmt.Sprintf("%s: %d 条消息", header.Filename, len(messages)))
	}

	if len(allMessages) == 0 {
		c.JSON(http.StatusInternalServerError, ImportResponse{
			Status:  "error",
			Message: "未能从任何文件中解析出消息",
		})
		return
	}

	// 构建人物画像
	p, err := profile.Build(allMessages, targetQQ)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ImportResponse{
			Status:  "error",
			Message: fmt.Sprintf("构建人物画像失败: %v", err),
		})
		return
	}

	// 保存画像
	if err := h.store.Save(p); err != nil {
		c.JSON(http.StatusInternalServerError, ImportResponse{
			Status:  "error",
			Message: fmt.Sprintf("保存画像失败: %v", err),
		})
		return
	}

	log.Printf("成功构建人物画像: %s (QQ: %s)，共 %d 条消息（来自 %d 个文件）", p.Nickname, p.QQ, len(allMessages), len(files))

	c.JSON(http.StatusOK, ImportResponse{
		Status:  "success",
		Message: fmt.Sprintf("从 %d 个文件导入 %d 条消息，构建了 %s 的人物画像。详情: %s", len(files), len(allMessages), p.Nickname, strings.Join(fileResults, "; ")),
		Profile: p,
	})
}

// GetProfile 查看当前人物画像
func (h *Handler) GetProfile(c *gin.Context) {
	h.cfgMu.RLock()
	targetQQ := h.cfg.Target.QQ
	h.cfgMu.RUnlock()

	p, err := h.store.Get(targetQQ)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("未找到 QQ %s 的人物画像，请先导入聊天记录", targetQQ)})
		return
	}

	c.JSON(http.StatusOK, p)
}

// Chat 处理聊天请求
func (h *Handler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	h.totalReceived.Add(1)

	h.cfgMu.RLock()
	targetQQ := h.cfg.Target.QQ
	delayBase := h.cfg.Response.ReplyDelayBase
	h.cfgMu.RUnlock()

	// 忽略目标成员自己的消息
	if req.SenderQQ == targetQQ {
		c.JSON(http.StatusOK, ChatResponse{Reply: ""})
		return
	}

	// 获取画像
	p, err := h.store.Get(targetQQ)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "人物画像未加载，请先导入聊天记录"})
		return
	}

	// ===== 自学习：从消息中学习 =====
	if h.learner != nil {
		// 学习新表达
		h.learner.LearnFromMessage(req.GroupID, req.SenderQQ, req.SenderName, req.Message)
		// 追踪话题
		h.learner.TrackTopic(req.GroupID, req.Message)
		// 检测反馈
		lastReply := h.getLastBotReply(req.GroupID)
		if fb := h.learner.DetectFeedback(req.Message, req.GroupID, req.SenderQQ, req.SenderName, lastReply); fb != nil {
			log.Printf("[学习] 检测到反馈: %s - %s", fb.Type, fb.Content)
		}
	}

	// 将外部上下文加入管理器
	for _, ctx := range req.Context {
		h.ctxMgr.Add(req.GroupID, ctx)
	}
	// 将当前消息也加入上下文
	h.ctxMgr.Add(req.GroupID, ContextMessage{
		Sender:  req.SenderName,
		Content: req.Message,
	})

	// ===== 图片识别：当消息附带图片且视觉模型可用时 =====
	var currentImageDescs []string
	if len(req.ImageURLs) > 0 {
		if h.visionClient != nil && h.visionClient.Enabled() {
			for _, imgURL := range req.ImageURLs {
				if strings.TrimSpace(imgURL) == "" {
					continue
				}
				desc, err := h.visionClient.DescribeImage(imgURL, "")
				if err != nil {
					log.Printf("[图片] 识别失败: %v", err)
					continue
				}
				currentImageDescs = append(currentImageDescs, desc)
				// 存入学习记忆
				if h.learner != nil {
					h.learner.AddImageMemory(req.GroupID, req.SenderQQ, req.SenderName, imgURL, desc, req.Message)
				}
				log.Printf("[图片] %s 发的图识别为: %s", req.SenderName, desc)
			}
		} else {
			log.Printf("[图片] 收到 %d 张图片，但视觉模型未启用，跳过识别", len(req.ImageURLs))
		}
	}

	// 构建 Prompt（包含学习上下文和风格配置）
	h.styleMu.RLock()
	currentStyle := h.style
	h.styleMu.RUnlock()

	systemPrompt := llm.BuildSystemPrompt(p, currentStyle)
	if h.learner != nil {
		learningContext := h.learner.GetLearningContext()
		if learningContext != "" {
			systemPrompt += "\n" + learningContext
		}
		// 召回相关的历史图片记忆
		imgCtx := h.learner.GetImageMemoriesContext(req.GroupID, req.Message)
		if imgCtx != "" {
			systemPrompt += "\n" + imgCtx
		}
	}
	contextMsgs := h.ctxMgr.Get(req.GroupID, 10)
	userMessages := llm.BuildUserPrompt(contextMsgs, req.SenderName, req.Message, p.Nickname)

	// 若本次消息带有图片，将识别结果告知模型，以便它针对图片回应
	if len(currentImageDescs) > 0 {
		imgNote := fmt.Sprintf("（对方刚刚发来了图片，图片内容是：%s。请结合图片内容和你的风格自然地回应）",
			strings.Join(currentImageDescs, "；"))
		userMessages = append(userMessages, llm.Message{Role: "user", Content: imgNote})
	}

	// 调用 LLM
	reply, err := h.llmClient.Chat(systemPrompt, userMessages)
	if err != nil {
		log.Printf("LLM 调用失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("LLM 调用失败: %v", err)})
		return
	}

	// 清理回复（去掉可能的引号包裹和多余空白）
	reply = cleanReply(reply)

	// ===== 自学习：检查是否重复 =====
	if h.learner != nil && h.learner.ShouldAvoidReply(reply) {
		log.Printf("[学习] 回复与历史重复，用更高多样性重新生成")
		// 用更高随机性 + 更强重复惩罚重新生成，打破复读循环
		reply2, err2 := h.llmClient.ChatDiverse(systemPrompt, userMessages)
		if err2 == nil {
			reply = cleanReply(reply2)
		}
	}

	h.totalReplied.Add(1)

	// 计算模拟打字延迟
	delay := float64(len([]rune(reply))) * delayBase

	// 将回复也加入上下文（作为目标成员的消息）
	h.ctxMgr.Add(req.GroupID, ContextMessage{
		Sender:  p.Nickname,
		Content: reply,
	})

	// ===== 自学习：记录回复 =====
	if h.learner != nil {
		h.learner.RecordReply(req.GroupID, req.Message, reply)
	}

	log.Printf("[%s] %s: %s -> %s: %s", req.GroupID, req.SenderName, req.Message, p.Nickname, reply)

	c.JSON(http.StatusOK, ChatResponse{
		Reply: reply,
		Delay: delay,
	})
}

// getLastBotReply 获取机器人在该群的最后一条回复
func (h *Handler) getLastBotReply(groupID string) string {
	if h.learner == nil {
		return ""
	}
	return h.learner.GetLastReply(groupID)
}

// UpdateConfig 运行时更新配置
func (h *Handler) UpdateConfig(c *gin.Context) {
	var newCfg config.Config
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置格式错误"})
		return
	}

	h.cfgMu.Lock()
	h.cfg = &newCfg
	h.cfgMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "配置已更新"})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	h.cfgMu.RLock()
	targetQQ := h.cfg.Target.QQ
	groupID := h.cfg.Target.GroupID
	h.cfgMu.RUnlock()

	_, err := h.store.Get(targetQQ)

	c.JSON(http.StatusOK, StatsResponse{
		TotalReceived: h.totalReceived.Load(),
		TotalReplied:  h.totalReplied.Load(),
		TargetQQ:      targetQQ,
		GroupID:       groupID,
		ProfileLoaded: err == nil,
	})
}

// ============ 自学习 API ============

// GetLearningStats 获取学习统计
func (h *Handler) GetLearningStats(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "学习模块未启用"})
		return
	}

	stats := h.learner.GetLearningStats()
	c.JSON(http.StatusOK, stats)
}

// GetLearningContext 获取学习上下文
func (h *Handler) GetLearningContext(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusOK, gin.H{"context": ""})
		return
	}

	context := h.learner.GetLearningContext()
	c.JSON(http.StatusOK, gin.H{"context": context})
}

// GetRecentFeedbacks 获取最近反馈
func (h *Handler) GetRecentFeedbacks(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusOK, gin.H{"feedbacks": []interface{}{}})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	feedbacks := h.learner.GetRecentFeedbacks(limit)
	c.JSON(http.StatusOK, gin.H{"feedbacks": feedbacks})
}

// GetNewExpressions 获取学到的新表达
func (h *Handler) GetNewExpressions(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusOK, gin.H{"expressions": []string{}})
		return
	}

	expressions := h.learner.GetNewExpressions()
	c.JSON(http.StatusOK, gin.H{"expressions": expressions})
}

// GetRecentTopics 获取最近话题
func (h *Handler) GetRecentTopics(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusOK, gin.H{"topics": []interface{}{}})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	topics := h.learner.GetRecentTopics(limit)
	c.JSON(http.StatusOK, gin.H{"topics": topics})
}

// SubmitFeedback 手动提交反馈
func (h *Handler) SubmitFeedback(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "学习模块未启用"})
		return
	}

	var req struct {
		GroupID    string `json:"group_id"`
		SenderQQ   string `json:"sender_qq"`
		SenderName string `json:"sender_name"`
		Content    string `json:"content"`
		BotReply   string `json:"bot_reply"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	fb := h.learner.DetectFeedback(req.Content, req.GroupID, req.SenderQQ, req.SenderName, req.BotReply)
	if fb != nil {
		c.JSON(http.StatusOK, gin.H{"status": "success", "feedback_type": fb.Type})
	} else {
		c.JSON(http.StatusOK, gin.H{"status": "no_feedback_detected"})
	}
}

// ============ 自我特质 API ============

// AddSelfTrait 添加自我特质
func (h *Handler) AddSelfTrait(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "学习模块未启用"})
		return
	}

	var req struct {
		Key         string `json:"key"`         // 特质关键词（如：小狗狗）
		Description string `json:"description"` // 特质描述（如：你是小狗狗）
		Response    string `json:"response"`    // 被提及时的回应（如：汪汪）
		CreatedBy   string `json:"created_by"`  // 创建者
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if req.Key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key 不能为空"})
		return
	}

	trait := h.learner.AddSelfTrait(req.Key, req.Description, req.Response, req.CreatedBy)
	c.JSON(http.StatusOK, gin.H{"status": "success", "trait": trait})
}

// GetSelfTraits 获取所有自我特质
func (h *Handler) GetSelfTraits(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusOK, gin.H{"traits": []interface{}{}})
		return
	}

	traits := h.learner.GetSelfTraits()
	c.JSON(http.StatusOK, gin.H{"traits": traits})
}

// RemoveSelfTrait 移除自我特质
func (h *Handler) RemoveSelfTrait(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "学习模块未启用"})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key 不能为空"})
		return
	}

	removed := h.learner.RemoveSelfTrait(key)
	if removed {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "特质不存在"})
	}
}

// ============ 图片记忆 API ============

// GetImageMemories 获取图片记忆
func (h *Handler) GetImageMemories(c *gin.Context) {
	if h.learner == nil {
		c.JSON(http.StatusOK, gin.H{"images": []interface{}{}})
		return
	}

	groupID := c.Query("group_id")
	limit := 10
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	images := h.learner.GetImageMemories(groupID, limit)
	c.JSON(http.StatusOK, gin.H{"images": images})
}

// GetVisionStatus 获取视觉模型启用状态
func (h *Handler) GetVisionStatus(c *gin.Context) {
	enabled := h.visionClient != nil && h.visionClient.Enabled()
	model := ""
	if enabled {
		h.cfgMu.RLock()
		model = h.cfg.Vision.Model
		h.cfgMu.RUnlock()
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled, "model": model})
}

// ============ 风格配置 API ============

// GetStyle 获取当前风格配置
func (h *Handler) GetStyle(c *gin.Context) {
	h.styleMu.RLock()
	defer h.styleMu.RUnlock()
	c.JSON(http.StatusOK, h.style)
}

// UpdateStyle 更新风格配置
func (h *Handler) UpdateStyle(c *gin.Context) {
	var req struct {
		LengthPreference *string  `json:"length_preference"`
		Tone             *string  `json:"tone"`
		CustomStyle      *string  `json:"custom_style"`
		UseEmoji         *bool    `json:"use_emoji"`
		SpeedMultiplier  *float64 `json:"speed_multiplier"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	h.styleMu.Lock()
	defer h.styleMu.Unlock()

	if req.LengthPreference != nil {
		h.style.LengthPreference = *req.LengthPreference
	}
	if req.Tone != nil {
		h.style.Tone = *req.Tone
	}
	if req.CustomStyle != nil {
		h.style.CustomStyle = *req.CustomStyle
	}
	if req.UseEmoji != nil {
		h.style.UseEmoji = *req.UseEmoji
	}
	if req.SpeedMultiplier != nil {
		h.style.SpeedMultiplier = *req.SpeedMultiplier
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "style": h.style})
}

// ResetStyle 重置风格配置
func (h *Handler) ResetStyle(c *gin.Context) {
	h.styleMu.Lock()
	defer h.styleMu.Unlock()
	h.style = config.DefaultStyleConfig()
	c.JSON(http.StatusOK, gin.H{"status": "success", "style": h.style})
}

// ShouldRespond 判断是否应该回复（供 NoneBot2 侧参考）
func ShouldRespond(message string, targetQQ string, targetName string, cfg *config.Config) bool {
	for _, mode := range cfg.Response.TriggerModes {
		switch mode {
		case "at":
			// 消息中 @ 了目标成员（NoneBot2 侧会处理 CQ 码）
			if strings.Contains(message, fmt.Sprintf("@%s", targetName)) {
				return true
			}
		case "keyword":
			for _, kw := range cfg.Response.TriggerKeywords {
				if strings.Contains(message, kw) {
					return true
				}
			}
		case "random":
			if rand.Float64() < cfg.Response.RandomProbability {
				return true
			}
		}
	}
	return false
}

// cleanReply 清理 LLM 返回的文本
func cleanReply(reply string) string {
	reply = strings.TrimSpace(reply)

	// 去掉首尾引号（使用 rune 切片以正确处理 UTF-8 多字节字符）
	runes := []rune(reply)
	if len(runes) >= 2 {
		first, last := runes[0], runes[len(runes)-1]
		if (first == '"' && last == '"') ||
			(first == '\u201c' && last == '\u201d') || // "..."
			(first == '\u300c' && last == '\u300d') { // 「...」
			reply = string(runes[1 : len(runes)-1])
		}
	}

	return reply
}
