package llm

import (
	"fmt"
	"strings"

	"qq-bot/internal/config"
	"qq-bot/internal/profile"
)

// BuildSystemPrompt 根据人物画像构建系统提示词
func BuildSystemPrompt(p *profile.Profile, style *config.StyleConfig) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("你是 QQ 群聊中的一个成员，名叫「%s」。你在群里说话的风格和性格如下，请严格按照这个风格来聊天。\n\n", p.Nickname))

	// 风格概览
	b.WriteString("【你的说话风格】\n")
	b.WriteString(fmt.Sprintf("- 平均每条消息 %0.1f 个字，%s\n", p.AvgLength, formalityDesc(p.Style.Formality)))
	b.WriteString(fmt.Sprintf("- 幽默感：%s\n", humorDesc(p.Style.HumorLevel)))

	// 口头禅
	if len(p.Style.Catchphrases) > 0 {
		b.WriteString(fmt.Sprintf("- 你经常说的话/词：%s\n", strings.Join(p.Style.Catchphrases, "、")))
	}

	// 常用表情
	if len(p.Style.CommonEmojis) > 0 {
		b.WriteString(fmt.Sprintf("- 你常用的语气词/表情：%s\n", strings.Join(p.Style.CommonEmojis, "、")))
	}

	// 句式特征
	if len(p.Style.SentencePatterns) > 0 {
		b.WriteString(fmt.Sprintf("- 你的句式特点：%s\n", strings.Join(p.Style.SentencePatterns, "；")))
	}

	// 话题偏好
	if len(p.Topics) > 0 {
		b.WriteString(fmt.Sprintf("- 你感兴趣的话题：%s\n", strings.Join(p.Topics, "、")))
	}

	// 回复模式
	b.WriteString("\n【你的回复习惯】\n")
	b.WriteString(fmt.Sprintf("- 别人问你问题时：%s\n", p.ResponsePatterns.ToQuestions))
	b.WriteString(fmt.Sprintf("- 别人跟你打招呼时：%s\n", p.ResponsePatterns.ToGreetings))
	b.WriteString(fmt.Sprintf("- 遇到争议话题时：%s\n", p.ResponsePatterns.ToControversy))

	// 示例消息
	if len(p.SampleMessages) > 0 {
		b.WriteString(fmt.Sprintf("\n【你以前在群里说过的话（真实记录）】\n"))
		for _, msg := range p.SampleMessages {
			b.WriteString(fmt.Sprintf("- %s\n", msg))
		}
	}

	// 运行时风格调整
	if style != nil {
		b.WriteString("\n【当前风格调整】\n")

		// 回复长度
		switch style.LengthPreference {
		case "short":
			b.WriteString("- 回复要简短，控制在 1-10 个字以内，像发微信一样简洁\n")
		case "long":
			b.WriteString("- 回复可以详细一些，50 字以上，充分表达你的想法\n")
		case "medium":
			b.WriteString("- 回复长度适中，10-50 个字\n")
		default:
			// 尝试解析为数字
			if style.LengthPreference != "" {
				b.WriteString(fmt.Sprintf("- 回复长度控制在 %s 个字左右\n", style.LengthPreference))
			}
		}

		// 语气风格
		if style.Tone != "" {
			toneDesc := getToneDescription(style.Tone)
			b.WriteString(fmt.Sprintf("- 语气风格：%s\n", toneDesc))
		}

		// 自定义风格
		if style.CustomStyle != "" {
			b.WriteString(fmt.Sprintf("- 额外要求：%s\n", style.CustomStyle))
		}

		// 表情使用
		if !style.UseEmoji {
			b.WriteString("- 不要使用表情符号和颜文字\n")
		}
	}

	// 规则
	b.WriteString("\n【聊天规则】\n")
	b.WriteString("- 用中文回复，像真人 QQ 聊天一样自然\n")
	b.WriteString("- 可以用网络用语、缩写、表情词，符合你的风格\n")
	b.WriteString("- 如果别人发了图片，你可以评论图片内容\n")
	b.WriteString("- 直接输出回复内容，不要加任何前缀、引号、Markdown\n")
	b.WriteString("- 不要说「作为AI」「我是一个语言模型」之类的话\n")

	return b.String()
}

// getToneDescription 获取语气描述
func getToneDescription(tone string) string {
	toneMap := map[string]string{
		"温柔": "说话温和体贴，用词柔软，让人感觉舒服",
		"暴躁": "说话直接粗暴，带点火气，可以用感叹号",
		"可爱": "说话萌一点，可以用叠词、语气词如'呀''呢''哦'",
		"高冷": "说话简短冷淡，不多废话，有点距离感",
		"搞笑": "说话幽默风趣，喜欢玩梗、开玩笑",
		"正经": "说话认真严肃，用词规范，不开玩笑",
		"傲娇": "嘴上说着不要，身体却很诚实，口是心非",
		"中二": "说话夸张戏剧化，喜欢用华丽的词藻",
		"毒舌": "说话犀利带刺，喜欢吐槽和讽刺",
		"元气": "说话充满活力，积极向上，带感叹号",
	}
	if desc, ok := toneMap[tone]; ok {
		return desc
	}
	return tone
}

// BuildUserPrompt 构建用户消息（包含上下文和当前消息）
func BuildUserPrompt(contextMessages []Message, senderName, message string, targetNickname string) []Message {
	var messages []Message

	// 添加对话上下文
	if len(contextMessages) > 0 {
		contextStr := "【群里最近的聊天】\n"
		for _, m := range contextMessages {
			contextStr += fmt.Sprintf("%s：%s\n", m.Role, m.Content)
		}
		messages = append(messages, Message{
			Role:    "user",
			Content: contextStr,
		})
	}

	// 当前消息
	messages = append(messages, Message{
		Role:    "user",
		Content: fmt.Sprintf("%s 说：%s\n\n请以 %s 的身份回复，直接说出你要说的话：", senderName, message, targetNickname),
	})

	return messages
}

func formalityDesc(f string) string {
	switch f {
	case "casual":
		return "随意/口语化"
	case "semi-formal":
		return "半正式"
	case "formal":
		return "正式"
	default:
		return f
	}
}

func humorDesc(h string) string {
	switch h {
	case "high":
		return "高，经常开玩笑"
	case "medium":
		return "中等"
	case "low":
		return "低，比较正经"
	default:
		return h
	}
}
