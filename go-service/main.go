package main

import (
	"fmt"
	"log"

	"qq-bot/internal/chat"
	"qq-bot/internal/config"
	"qq-bot/internal/learning"
	"qq-bot/internal/llm"
	"qq-bot/internal/profile"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化各模块
	store := profile.NewStore("data/profiles")
	llmClient := llm.NewClient(llm.ClientOptions{
		BaseURL:          cfg.LLM.BaseURL,
		APIKey:           cfg.LLM.APIKey,
		Model:            cfg.LLM.Model,
		MaxTokens:        cfg.LLM.MaxTokens,
		Temperature:      cfg.LLM.Temperature,
		TopP:             cfg.LLM.TopP,
		PresencePenalty:  cfg.LLM.PresencePenalty,
		FrequencyPenalty: cfg.LLM.FrequencyPenalty,
		DisableThinking:  cfg.LLM.DisableThinking,
	})

	// 初始化视觉模型客户端（用于图片识别，可选）
	var visionClient *llm.VisionClient
	if cfg.Vision.Enabled && cfg.Vision.APIKey != "" {
		visionClient = llm.NewVisionClient(
			cfg.Vision.BaseURL,
			cfg.Vision.APIKey,
			cfg.Vision.Model,
			cfg.Vision.MaxTokens,
			cfg.Vision.Proxy,
			cfg.Vision.DisableThinking,
		)
		if cfg.Vision.Proxy != "" {
			log.Printf("视觉模型已启用: %s (%s)，代理: %s，图片识别功能开启", cfg.Vision.Model, cfg.Vision.Provider, cfg.Vision.Proxy)
		} else {
			log.Printf("视觉模型已启用: %s (%s)，图片识别功能开启", cfg.Vision.Model, cfg.Vision.Provider)
		}
	} else {
		log.Printf("视觉模型未启用，图片识别功能关闭（在 config.yaml 的 vision 节配置）")
	}

	// 初始化学习模块
	profilePath := fmt.Sprintf("data/profiles/%s.json", cfg.Target.QQ)
	learner := learning.NewLearningManager(profilePath, "data/learning")
	log.Printf("自学习模块已启用")

	chatHandler := chat.NewHandler(llmClient, visionClient, store, cfg, learner)

	r := gin.Default()

	// API 路由
	api := r.Group("/api")
	{
		// 基础接口
		api.POST("/import", chatHandler.ImportHistory)
		api.GET("/profile", chatHandler.GetProfile)
		api.POST("/chat", chatHandler.Chat)
		api.PUT("/config", chatHandler.UpdateConfig)
		api.GET("/stats", chatHandler.GetStats)

		// 自学习接口
		api.GET("/learning/stats", chatHandler.GetLearningStats)
		api.GET("/learning/context", chatHandler.GetLearningContext)
		api.GET("/learning/feedbacks", chatHandler.GetRecentFeedbacks)
		api.GET("/learning/expressions", chatHandler.GetNewExpressions)
		api.GET("/learning/topics", chatHandler.GetRecentTopics)
		api.POST("/learning/feedback", chatHandler.SubmitFeedback)

		// 自我特质接口
		api.GET("/learning/traits", chatHandler.GetSelfTraits)
		api.POST("/learning/traits", chatHandler.AddSelfTrait)
		api.DELETE("/learning/traits/:key", chatHandler.RemoveSelfTrait)

		// 图片记忆接口
		api.GET("/learning/images", chatHandler.GetImageMemories)
		api.GET("/vision/status", chatHandler.GetVisionStatus)

		// 风格配置接口
		api.GET("/style", chatHandler.GetStyle)
		api.PUT("/style", chatHandler.UpdateStyle)
		api.POST("/style/reset", chatHandler.ResetStyle)
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("QQ模仿机器人服务启动于 %s", addr)
	log.Printf("目标模仿QQ号: %s", cfg.Target.QQ)
	log.Printf("LLM提供商: %s, 模型: %s", cfg.LLM.Provider, cfg.LLM.Model)

	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
