package profile

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"qq-bot/internal/history"
)

// Profile 人物画像
type Profile struct {
	Nickname        string            `json:"nickname"`
	QQ              string            `json:"qq"`
	TotalMessages   int               `json:"total_messages"`
	AvgLength       float64           `json:"avg_length"`
	Style           StyleProfile      `json:"style"`
	Topics          []string          `json:"topics"`
	ResponsePatterns ResponsePatterns `json:"response_patterns"`
	SampleMessages  []string          `json:"sample_messages"`
}

type StyleProfile struct {
	Formality        string   `json:"formality"`
	HumorLevel       string   `json:"humor_level"`
	EmojiFrequency   string   `json:"emoji_frequency"`
	CommonEmojis     []string `json:"common_emojis"`
	Catchphrases     []string `json:"catchphrases"`
	SentencePatterns []string `json:"sentence_patterns"`
}

type ResponsePatterns struct {
	ToQuestions    string `json:"to_questions"`
	ToGreetings    string `json:"to_greetings"`
	ToControversy  string `json:"to_controversy"`
}


// Build 从聊天记录构建人物画像
func Build(messages []history.Message, targetQQ string) (*Profile, error) {
	// 筛选目标成员的消息
	targetMsgs := history.FilterByQQ(messages, targetQQ)
	if len(targetMsgs) == 0 {
		return nil, fmt.Errorf("未找到 QQ 号 %s 的任何消息", targetQQ)
	}

	nickname := targetMsgs[0].Sender
	avgLen := history.AvgLength(targetMsgs)

	// 提取所有消息内容
	var contents []string
	for _, m := range targetMsgs {
		contents = append(contents, m.Content)
	}

	// 分析常用表情/口头禅
	emojis := extractEmojis(contents)
	catchphrases := extractCatchphrases(contents)
	patterns := analyzeSentencePatterns(contents)
	formality := analyzeFormality(contents)
	humorLevel := analyzeHumor(contents)
	emojiFreq := analyzeEmojiFrequency(contents, emojis)

	// 使用全部消息作为样本
	samples := selectAllSamples(targetMsgs)

	p := &Profile{
		Nickname:      nickname,
		QQ:            targetQQ,
		TotalMessages: len(targetMsgs),
		AvgLength:     avgLen,
		Style: StyleProfile{
			Formality:        formality,
			HumorLevel:       humorLevel,
			EmojiFrequency:   emojiFreq,
			CommonEmojis:     emojis,
			Catchphrases:     catchphrases,
			SentencePatterns: patterns,
		},
		Topics: extractTopics(contents),
		ResponsePatterns: ResponsePatterns{
			ToQuestions:   inferQuestionResponse(contents),
			ToGreetings:   inferGreetingResponse(contents),
			ToControversy: inferControversyResponse(contents),
		},
		SampleMessages: samples,
	}

	return p, nil
}

// extractEmojis 提取常用表情（中文常见网络表情词）
func extractEmojis(contents []string) []string {
	emojiWords := map[string]int{}
	knownEmojis := []string{
		"哈哈", "嘻嘻", "呵呵", "笑死", "绷不住了", "6", "666", "6666",
		"233", "2333", "awsl", "绝了", "好家伙", "牛", "nb", "NB",
		"草", "离谱", "乐", "蚌埠住了", "蚌埠", "捏", "呜呜", "哭了",
		"泪目", "破防了", "麻了", "赢麻了", "寄", "GG", "gg",
		"摆烂", "开摆", "真香", "爷青回", "YYDS", "yyds",
		"orz", "Orz", "OTL", "xd", "XD", "XDD", "xdd",
		"hhh", "hhhh", "hhhhh", "哈哈哈哈", "嘿嘿", "hehe",
		"awsl", "xdm", "兄弟们", "家人", "宝", "救命",
		"好耶", "耶", "芜湖", "起飞", "冲", "冲冲冲",
		"qwq", "QwQ", "QAQ", "qaq", "ovo", "OvO",
		"属于是", "笑死", "绷不住", "差不多得了", "有一说", "确实",
		"不是", "有没有可能", "怎么说", "懂不懂", "会不会", "能不能",
		"逆天", "抽象", "典", "太典了", "绷", "乐", "孝", "急",
		"赢", "麻", "润", "卷", "躺", "摆", "内卷", "躺平",
	}

	for _, c := range contents {
		for _, emoji := range knownEmojis {
			if strings.Contains(strings.ToLower(c), strings.ToLower(emoji)) {
				emojiWords[emoji]++
			}
		}
	}

	// 取出现次数最多的前10个
	type pair struct {
		key string
		cnt int
	}
	var pairs []pair
	for k, v := range emojiWords {
		pairs = append(pairs, pair{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].cnt > pairs[j].cnt })

	var result []string
	for i, p := range pairs {
		if i >= 10 {
			break
		}
		result = append(result, p.key)
	}
	return result
}

// extractCatchphrases 提取口头禅（高频短词/短语）
func extractCatchphrases(contents []string) []string {
	wordFreq := map[string]int{}
	// 统计 2-4 字的高频纯中文片段
	for _, c := range contents {
		runes := []rune(c)
		for n := 2; n <= 4; n++ {
			for i := 0; i+n <= len(runes); i++ {
				word := string(runes[i : i+n])
				if isAllChinese(word) {
					wordFreq[word]++
				}
			}
		}
	}

	// 过滤出现次数太少的
	type pair struct {
		key string
		cnt int
	}
	var pairs []pair
	threshold := len(contents) / 50
	if threshold < 2 {
		threshold = 2
	}
	for k, v := range wordFreq {
		if v >= threshold {
			pairs = append(pairs, pair{k, v})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].cnt > pairs[j].cnt })

	var result []string
	for i, p := range pairs {
		if i >= 20 {
			break
		}
		result = append(result, p.key)
	}
	return result
}

func isAllChinese(s string) bool {
	for _, r := range s {
		if !unicode.Is(unicode.Han, r) {
			return false
		}
	}
	return true
}

// analyzeSentencePatterns 分析句式特征
func analyzeSentencePatterns(contents []string) []string {
	var patterns []string

	questionCount := 0
	shortCount := 0
	longCount := 0
	exclamationCount := 0

	for _, c := range contents {
		runes := []rune(c)
		if len(runes) <= 5 {
			shortCount++
		}
		if len(runes) >= 50 {
			longCount++
		}
		if strings.Contains(c, "？") || strings.Contains(c, "?") {
			questionCount++
		}
		if strings.Contains(c, "！") || strings.Contains(c, "!") {
			exclamationCount++
		}
	}

	total := len(contents)
	if float64(shortCount)/float64(total) > 0.5 {
		patterns = append(patterns, "喜欢用短句")
	}
	if float64(longCount)/float64(total) > 0.3 {
		patterns = append(patterns, "偶尔发长文")
	}
	if float64(questionCount)/float64(total) > 0.2 {
		patterns = append(patterns, "经常反问/提问")
	}
	if float64(exclamationCount)/float64(total) > 0.2 {
		patterns = append(patterns, "语气比较强烈，常用感叹号")
	}

	return patterns
}

func analyzeFormality(contents []string) string {
	// 简单判断：如果消息平均长度短且表情多，则偏随意
	avgLen := 0
	for _, c := range contents {
		avgLen += len([]rune(c))
	}
	if len(contents) > 0 {
		avgLen /= len(contents)
	}
	if avgLen < 15 {
		return "casual"
	} else if avgLen < 40 {
		return "semi-formal"
	}
	return "formal"
}

func analyzeHumor(contents []string) string {
	humorWords := []string{"哈", "笑", "乐", "绷", "蚌埠", "绝", "离谱", "草", "寄", "赢"}
	count := 0
	for _, c := range contents {
		for _, w := range humorWords {
			if strings.Contains(c, w) {
				count++
				break
			}
		}
	}
	ratio := float64(count) / float64(len(contents))
	if ratio > 0.3 {
		return "high"
	} else if ratio > 0.1 {
		return "medium"
	}
	return "low"
}

func analyzeEmojiFrequency(contents []string, emojis []string) string {
	if len(emojis) == 0 {
		return "low"
	}
	count := 0
	for _, c := range contents {
		for _, e := range emojis {
			if strings.Contains(c, e) {
				count++
				break
			}
		}
	}
	ratio := float64(count) / float64(len(contents))
	if ratio > 0.4 {
		return "high"
	} else if ratio > 0.15 {
		return "medium"
	}
	return "low"
}

// extractTopics 简单话题提取（基于关键词）
func extractTopics(contents []string) []string {
	topicKeywords := map[string][]string{
		"游戏":   {"游戏", "打团", "副本", "排位", "上分", "吃鸡", "原神", "王者", "LOL", "DOTA", "CSGO", "MC", "Steam", "电竞", "比赛", "冠军", "MVP", "KDA", "ACE", "faker", "niko", "acs", "boss", "副本", "装备", "段位"},
		"编程":   {"代码", "编程", "bug", "Bug", "BUG", "debug", "编译", "算法", "Python", "Go", "Java", "C++", "前端", "后端", "服务器", "数据库", "API"},
		"动漫":   {"动漫", "番剧", "追番", "新番", "漫画", "二次元", "cos", "Cos", "ACG", "轻小说", "B站"},
		"美食":   {"好吃", "美食", "餐厅", "做饭", "外卖", "奶茶", "火锅", "烧烤", "吃"},
		"音乐":   {"音乐", "歌曲", "专辑", "演唱会", "乐队", "唱歌", "听歌"},
		"运动":   {"篮球", "足球", "跑步", "健身", "游泳", "运动", "锻炼"},
		"影视":   {"电影", "电视剧", "综艺", "追剧", "影评", "导演", "演员"},
		"学习":   {"考试", "作业", "论文", "学习", "复习", "挂科", "GPA", "大专", "本科"},
		"工作":   {"上班", "加班", "项目", "需求", "开会", "同事", "老板", "工资"},
		"科技":   {"手机", "电脑", "显卡", "CPU", "GPU", "AI", "ChatGPT", "科技", "数码"},
		"生活":   {"睡觉", "起床", "天气", "下雨", "冷", "热", "无聊", "困", "累"},
	}

	topicCount := map[string]int{}
	for _, c := range contents {
		lower := strings.ToLower(c)
		for topic, keywords := range topicKeywords {
			for _, kw := range keywords {
				if strings.Contains(lower, strings.ToLower(kw)) {
					topicCount[topic]++
					break
				}
			}
		}
	}

	type pair struct {
		topic string
		cnt   int
	}
	var pairs []pair
	for t, c := range topicCount {
		if c >= 3 {
			pairs = append(pairs, pair{t, c})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].cnt > pairs[j].cnt })

	var result []string
	for i, p := range pairs {
		if i >= 8 {
			break
		}
		result = append(result, p.topic)
	}
	return result
}

func inferQuestionResponse(contents []string) string {
	// 简单启发式推断
	hasJoke := false
	hasDirect := false
	for _, c := range contents {
		if strings.Contains(c, "？") || strings.Contains(c, "?") {
			if strings.Contains(c, "哈") || strings.Contains(c, "笑") {
				hasJoke = true
			}
		}
		if len([]rune(c)) < 10 && !strings.Contains(c, "？") {
			hasDirect = true
		}
	}
	if hasJoke {
		return "喜欢先调侃再回答"
	}
	if hasDirect {
		return "回答简洁直接"
	}
	return "正常回答问题"
}

func inferGreetingResponse(contents []string) string {
	shortCount := 0
	for _, c := range contents {
		if len([]rune(c)) <= 3 {
			shortCount++
		}
	}
	if float64(shortCount)/float64(len(contents)) > 0.3 {
		return "回复简短，可能比较敷衍"
	}
	return "正常回应打招呼"
}

func inferControversyResponse(contents []string) string {
	agreeWords := 0
	argueWords := 0
	for _, c := range contents {
		if strings.Contains(c, "确实") || strings.Contains(c, "确实") || strings.Contains(c, "同意") {
			agreeWords++
		}
		if strings.Contains(c, "不是") || strings.Contains(c, "不对") || strings.Contains(c, "胡说") {
			argueWords++
		}
	}
	if argueWords > agreeWords*2 {
		return "喜欢反驳和争论"
	}
	if agreeWords > argueWords {
		return "倾向于附和，不喜欢争论"
	}
	return "立场比较中立，喜欢和稀泥"
}

// selectAllSamples 使用全部消息作为样本
func selectAllSamples(messages []history.Message) []string {
	var result []string
	for _, m := range messages {
		if len([]rune(m.Content)) >= 2 {
			result = append(result, m.Content)
		}
	}
	return result
}

// selectSamples 精选代表性消息（均匀采样）
func selectSamples(messages []history.Message, maxCount int) []string {
	if len(messages) <= maxCount {
		var result []string
		for _, m := range messages {
			if len([]rune(m.Content)) >= 3 {
				result = append(result, m.Content)
			}
		}
		return result
	}

	// 均匀采样
	step := len(messages) / maxCount
	var result []string
	for i := 0; i < len(messages) && len(result) < maxCount; i += step {
		c := messages[i].Content
		if len([]rune(c)) >= 3 {
			result = append(result, c)
		}
	}
	return result
}
