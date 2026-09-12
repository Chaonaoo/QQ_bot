"""
QQ 模仿机器人 - 指令系统
支持 / 开头的命令交互
"""

import asyncio
from typing import Optional

from nonebot import on_command, logger, get_driver
from nonebot.adapters.onebot.v11 import (
    Bot,
    GroupMessageEvent,
    Message,
    MessageSegment,
)
from nonebot.params import CommandArg
from nonebot.rule import to_me

from .config import MimicConfig
from .go_client import GoClient

# ============ 全局实例 ============
config = MimicConfig()
go_client = GoClient(config.go_service_url)

# 运行时状态
_bot_muted: bool = False  # 是否被禁言
_proactive_enabled: bool = config.proactive_enabled  # 主动说话开关


# ============ 帮助命令 ============
help_cmd = on_command("help", aliases={"帮助", "命令"}, priority=1, block=True)


@help_cmd.handle()
async def handle_help(bot: Bot, event: GroupMessageEvent):
    """显示帮助信息"""
    help_text = """🤖 QQ模仿机器人指令列表

📊 信息查询
/help - 显示此帮助
/stats - 查看统计信息
/profile - 查看人物画像
/learn - 查看学习状态
/expressions - 查看学到的表达
/topics - 查看最近话题

🎮 控制命令
/mute - 禁言机器人（暂停回复）
/unmute - 解除禁言
/proactive on - 开启主动说话
/proactive off - 关闭主动说话
/probability <0-1> - 设置随机触发概率

💬 互动命令
/say <内容> - 让机器人说指定的话
/ask <问题> - 向机器人提问
/feedback <内容> - 提交反馈（如：不像/应该是...）

🧠 自我认知
/remember <关键词> <回应> - 记住自己的特质
  例：/remember 小狗狗 汪汪
/forget <关键词> - 忘记某个特质
/traits - 查看记住的特质

🖼️ 图片识别
@我并发图 - 识别图片内容并记住
/images - 查看我记住的图片
/vision - 查看图片识别是否开启

🎨 风格调整
/length <short|medium|long|数字> - 回复长度
/tone <温柔|暴躁|可爱|高冷|搞笑|正经|傲娇|中二|毒舌|元气> - 语气
/style - 查看当前风格
/style reset - 重置风格
/frequency <0-100> - 随机回复概率(%)

⚙️ 管理命令
/reset - 重置学习数据（谨慎使用）
/config - 查看当前配置

💡 提示：也可以 @我 直接聊天"""

    await help_cmd.finish(MessageSegment.text(help_text))


# ============ 统计命令 ============
stats_cmd = on_command("stats", aliases={"统计"}, priority=1, block=True)


@stats_cmd.handle()
async def handle_stats(bot: Bot, event: GroupMessageEvent):
    """查看统计信息"""
    result = await go_client.get_stats()
    if result:
        msg = (
            f"📊 机器人统计\n"
            f"━━━━━━━━━━\n"
            f"接收消息: {result.get('total_received', 0)}\n"
            f"回复消息: {result.get('total_replied', 0)}\n"
            f"目标QQ: {result.get('target_qq', 'N/A')}\n"
            f"目标群: {result.get('group_id', 'N/A')}\n"
            f"画像状态: {'✅ 已加载' if result.get('profile_loaded') else '❌ 未加载'}\n"
            f"━━━━━━━━━━\n"
            f"禁言状态: {'🔇 已禁言' if _bot_muted else '🔊 正常'}\n"
            f"主动说话: {'✅ 开启' if _proactive_enabled else '❌ 关闭'}"
        )
    else:
        msg = "❌ 无法获取统计信息，Go 服务可能未启动"

    await stats_cmd.finish(MessageSegment.text(msg))


# ============ 画像命令 ============
profile_cmd = on_command("profile", aliases={"画像"}, priority=1, block=True)


@profile_cmd.handle()
async def handle_profile(bot: Bot, event: GroupMessageEvent):
    """查看人物画像"""
    result = await go_client.get_profile()
    if result:
        nickname = result.get("nickname", "未知")
        total = result.get("total_messages", 0)
        avg_len = result.get("avg_length", 0)
        style = result.get("style", {})
        catchphrases = style.get("catchphrases", [])
        emojis = style.get("common_emojis", [])
        topics = result.get("topics", [])

        msg = (
            f"👤 人物画像 - {nickname}\n"
            f"━━━━━━━━━━\n"
            f"总消息数: {total}\n"
            f"平均长度: {avg_len:.1f} 字\n"
            f"━━━━━━━━━━\n"
            f"口头禅: {', '.join(catchphrases[:8]) if catchphrases else '无'}\n"
            f"常用表情: {', '.join(emojis[:8]) if emojis else '无'}\n"
            f"话题偏好: {', '.join(topics[:5]) if topics else '无'}"
        )
    else:
        msg = "❌ 人物画像未加载，请先导入聊天记录"

    await profile_cmd.finish(MessageSegment.text(msg))


# ============ 学习状态命令 ============
learn_cmd = on_command("learn", aliases={"学习", "learning"}, priority=1, block=True)


@learn_cmd.handle()
async def handle_learn(bot: Bot, event: GroupMessageEvent):
    """查看学习状态"""
    result = await go_client.get_learning_stats()
    if result:
        msg = (
            f"🧠 自学习状态\n"
            f"━━━━━━━━━━\n"
            f"总反馈数: {result.get('total_feedbacks', 0)}\n"
            f"  ├ 负面: {result.get('negative_feedbacks', 0)}\n"
            f"  ├ 正面: {result.get('positive_feedbacks', 0)}\n"
            f"  └ 纠正: {result.get('corrections', 0)}\n"
            f"━━━━━━━━━━\n"
            f"表达学习: {result.get('total_expressions', 0)} 个\n"
            f"  ├ 临时层: {result.get('temp_expressions', 0)}\n"
            f"  ├ 稳定层: {result.get('stable_expressions', 0)}\n"
            f"  └ 矛盾: {result.get('contradict_expressions', 0)}\n"
            f"━━━━━━━━━━\n"
            f"回复历史: {result.get('reply_history', 0)} 条\n"
            f"话题追踪: {result.get('recent_topics', 0)} 个"
        )
    else:
        msg = "❌ 无法获取学习状态"

    await learn_cmd.finish(MessageSegment.text(msg))


# ============ 禁言命令 ============
mute_cmd = on_command("mute", aliases={"禁言"}, priority=1, block=True)


@mute_cmd.handle()
async def handle_mute(bot: Bot, event: GroupMessageEvent):
    """禁言机器人"""
    global _bot_muted
    _bot_muted = True
    await mute_cmd.finish(MessageSegment.text("🔇 已禁言，我将不再回复消息。\n使用 /unmute 解除禁言"))


unmute_cmd = on_command("unmute", aliases={"解禁", "解除禁言"}, priority=1, block=True)


@unmute_cmd.handle()
async def handle_unmute(bot: Bot, event: GroupMessageEvent):
    """解除禁言"""
    global _bot_muted
    _bot_muted = False
    await unmute_cmd.finish(MessageSegment.text("🔊 已解除禁言，我回来了！"))


# ============ 主动说话控制 ============
proactive_cmd = on_command("proactive", aliases={"主动"}, priority=1, block=True)


@proactive_cmd.handle()
async def handle_proactive(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """控制主动说话"""
    global _proactive_enabled

    arg_text = args.extract_plain_text().strip().lower()

    if arg_text in ("on", "开启", "enable", "1", "true"):
        _proactive_enabled = True
        config.proactive_enabled = True
        await proactive_cmd.finish(MessageSegment.text("✅ 主动说话已开启"))
    elif arg_text in ("off", "关闭", "disable", "0", "false"):
        _proactive_enabled = False
        config.proactive_enabled = False
        await proactive_cmd.finish(MessageSegment.text("❌ 主动说话已关闭"))
    else:
        status = "✅ 开启" if _proactive_enabled else "❌ 关闭"
        await proactive_cmd.finish(MessageSegment.text(
            f"主动说话状态: {status}\n"
            f"使用 /proactive on 或 /proactive off 切换"
        ))


# ============ 概率设置 ============
probability_cmd = on_command("probability", aliases={"概率"}, priority=1, block=True)


@probability_cmd.handle()
async def handle_probability(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """设置随机触发概率"""
    arg_text = args.extract_plain_text().strip()

    if not arg_text:
        await probability_cmd.finish(MessageSegment.text(
            f"当前随机触发概率: {config.random_probability:.0%}\n"
            f"使用 /probability <0-1> 设置，如 /probability 0.5"
        ))
        return

    try:
        value = float(arg_text)
        if 0 <= value <= 1:
            config.random_probability = value
            await probability_cmd.finish(MessageSegment.text(f"✅ 随机触发概率已设置为 {value:.0%}"))
        else:
            await probability_cmd.finish(MessageSegment.text("❌ 概率值必须在 0-1 之间"))
    except ValueError:
        await probability_cmd.finish(MessageSegment.text("❌ 请输入有效的数字，如 /probability 0.5"))


# ============ 说话命令 ============
say_cmd = on_command("say", aliases={"说"}, priority=1, block=True)


@say_cmd.handle()
async def handle_say(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """让机器人说指定的话"""
    content = args.extract_plain_text().strip()

    if not content:
        await say_cmd.finish(MessageSegment.text("❌ 请指定要说的内容，如 /say 大家好"))
        return

    await say_cmd.finish(MessageSegment.text(content))


# ============ 提问命令 ============
ask_cmd = on_command("ask", aliases={"问", "询问"}, priority=1, block=True)


@ask_cmd.handle()
async def handle_ask(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """向机器人提问"""
    question = args.extract_plain_text().strip()

    if not question:
        await ask_cmd.finish(MessageSegment.text("❌ 请指定问题，如 /ask 今晚打游戏吗"))
        return

    group_id = str(event.group_id)
    sender_qq = str(event.user_id)
    sender_name = event.sender.nickname or event.sender.card or sender_qq

    result = await go_client.chat(
        group_id=group_id,
        sender_qq=sender_qq,
        sender_name=sender_name,
        message=question,
        context=[],
    )

    if result and result.get("reply"):
        await ask_cmd.finish(MessageSegment.text(result["reply"]))
    else:
        await ask_cmd.finish(MessageSegment.text("❌ 无法获取回复，请稍后再试"))


# ============ 反馈命令 ============
feedback_cmd = on_command("feedback", aliases={"反馈"}, priority=1, block=True)


@feedback_cmd.handle()
async def handle_feedback(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """提交反馈"""
    content = args.extract_plain_text().strip()

    if not content:
        await feedback_cmd.finish(MessageSegment.text(
            "❌ 请指定反馈内容\n"
            "例如：\n"
            "/feedback 不像\n"
            "/feedback 应该是 666\n"
            "/feedback 不错"
        ))
        return

    group_id = str(event.group_id)
    sender_qq = str(event.user_id)
    sender_name = event.sender.nickname or event.sender.card or sender_qq

    result = await go_client.submit_feedback(
        group_id=group_id,
        sender_qq=sender_qq,
        sender_name=sender_name,
        content=content,
    )

    if result:
        feedback_type = result.get("feedback_type", "unknown")
        type_names = {
            "negative": "负面反馈",
            "positive": "正面反馈",
            "correction": "纠正",
        }
        type_name = type_names.get(feedback_type, feedback_type)
        await feedback_cmd.finish(MessageSegment.text(f"✅ 已记录{type_name}，我会努力改进！"))
    else:
        await feedback_cmd.finish(MessageSegment.text("❌ 反馈提交失败"))


# ============ 自我认知命令 ============
remember_cmd = on_command("remember", aliases={"记住", "记忆"}, priority=1, block=True)


@remember_cmd.handle()
async def handle_remember(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """记住自我特质"""
    content = args.extract_plain_text().strip()

    if not content:
        await remember_cmd.finish(MessageSegment.text(
            "🧠 让我记住自己的特质\n"
            "━━━━━━━━━━\n"
            "格式：/remember <关键词> <回应>\n"
            "例子：\n"
            "  /remember 小狗狗 汪汪\n"
            "  /remember 大哥 在呢\n"
            "  /remember 笨蛋 你才笨\n"
            "━━━━━━━━━━\n"
            "之后别人叫我\"小狗狗\"时，我会回应\"汪汪\""
        ))
        return

    # 解析参数：第一个词是关键词，后面是回应
    parts = content.split(maxsplit=1)
    if len(parts) < 2:
        await remember_cmd.finish(MessageSegment.text(
            "❌ 格式错误\n"
            "正确格式：/remember <关键词> <回应>\n"
            "例如：/remember 小狗狗 汪汪"
        ))
        return

    key = parts[0]
    response = parts[1]

    sender_name = event.sender.nickname or event.sender.card or str(event.user_id)

    result = await go_client.add_self_trait(
        key=key,
        description=f"我是{key}",
        response=response,
        created_by=sender_name,
    )

    if result:
        await remember_cmd.finish(MessageSegment.text(
            f"✅ 记住了！\n"
            f"━━━━━━━━━━\n"
            f"当有人叫我\"{key}\"时\n"
            f"我会回应：\"{response}\"\n"
            f"━━━━━━━━━━\n"
            f"由 {sender_name} 设定"
        ))
    else:
        await remember_cmd.finish(MessageSegment.text("❌ 记忆失败，请稍后再试"))


forget_cmd = on_command("forget", aliases={"忘记", "忘掉"}, priority=1, block=True)


@forget_cmd.handle()
async def handle_forget(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """忘记自我特质"""
    key = args.extract_plain_text().strip()

    if not key:
        await forget_cmd.finish(MessageSegment.text(
            "❌ 请指定要忘记的关键词\n"
            "例如：/forget 小狗狗"
        ))
        return

    removed = await go_client.remove_self_trait(key)

    if removed:
        await forget_cmd.finish(MessageSegment.text(f"✅ 已忘记\"{key}\"这个特质"))
    else:
        await forget_cmd.finish(MessageSegment.text(f"❌ 没有找到\"{key}\"这个特质"))


traits_cmd = on_command("traits", aliases={"特质", "自我认知"}, priority=1, block=True)


@traits_cmd.handle()
async def handle_traits(bot: Bot, event: GroupMessageEvent):
    """查看自我特质"""
    traits = await go_client.get_self_traits()

    if traits:
        msg = f"🧠 我的自我认知 ({len(traits)} 个)\n━━━━━━━━━━\n"
        for trait in traits:
            key = trait.get("key", "")
            response = trait.get("response", "")
            mentions = trait.get("mention_count", 0)
            msg += f"• {key} → {response} (被提及 {mentions} 次)\n"
        msg += "━━━━━━━━━━\n"
        msg += "使用 /remember 添加，/forget 删除"
    else:
        msg = "🧠 我还没有特殊的自我认知\n使用 /remember <关键词> <回应> 来设定"

    await traits_cmd.finish(MessageSegment.text(msg))


# ============ 风格调整命令 ============
length_cmd = on_command("length", aliases={"长度", "字数"}, priority=1, block=True)


@length_cmd.handle()
async def handle_length(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """设置回复长度"""
    arg = args.extract_plain_text().strip().lower()

    if not arg:
        style = await go_client.get_style()
        current = style.get("length_preference", "medium") if style else "medium"
        await length_cmd.finish(MessageSegment.text(
            f"📏 当前回复长度: {current}\n"
            f"━━━━━━━━━━\n"
            f"可选值:\n"
            f"  short - 简短 (1-10字)\n"
            f"  medium - 适中 (10-50字)\n"
            f"  long - 详细 (50+字)\n"
            f"  数字 - 指定字数\n"
            f"━━━━━━━━━━\n"
            f"例: /length short 或 /length 20"
        ))
        return

    # 验证参数
    valid_values = ["short", "medium", "long"]
    if arg not in valid_values:
        try:
            num = int(arg)
            if num < 1 or num > 500:
                await length_cmd.finish(MessageSegment.text("❌ 数字范围: 1-500"))
                return
        except ValueError:
            await length_cmd.finish(MessageSegment.text(
                "❌ 无效值\n可选: short, medium, long, 或 1-500 的数字"
            ))
            return

    result = await go_client.update_style(length_preference=arg)
    if result:
        await length_cmd.finish(MessageSegment.text(f"✅ 回复长度已设置为: {arg}"))
    else:
        await length_cmd.finish(MessageSegment.text("❌ 设置失败"))


tone_cmd = on_command("tone", aliases={"语气", "风格"}, priority=1, block=True)


@tone_cmd.handle()
async def handle_tone(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """设置语气风格"""
    arg = args.extract_plain_text().strip()

    if not arg:
        style = await go_client.get_style()
        current = style.get("tone", "") if style else ""
        current_display = current if current else "默认（跟随画像）"
        await tone_cmd.finish(MessageSegment.text(
            f"🎭 当前语气: {current_display}\n"
            f"━━━━━━━━━━\n"
            f"可选语气:\n"
            f"  温柔 - 温和体贴\n"
            f"  暴躁 - 直接粗暴\n"
            f"  可爱 - 萌萌哒\n"
            f"  高冷 - 冷淡简短\n"
            f"  搞笑 - 幽默玩梗\n"
            f"  正经 - 严肃认真\n"
            f"  傲娇 - 口是心非\n"
            f"  中二 - 夸张戏剧\n"
            f"  毒舌 - 犀利吐槽\n"
            f"  元气 - 充满活力\n"
            f"━━━━━━━━━━\n"
            f"例: /tone 可爱\n"
            f"清除: /tone 默认"
        ))
        return

    # 清除语气
    if arg in ("默认", "清除", "reset", "none", ""):
        result = await go_client.update_style(tone="")
        if result:
            await tone_cmd.finish(MessageSegment.text("✅ 已恢复默认语气"))
        else:
            await tone_cmd.finish(MessageSegment.text("❌ 设置失败"))
        return

    result = await go_client.update_style(tone=arg)
    if result:
        await tone_cmd.finish(MessageSegment.text(f"✅ 语气已设置为: {arg}"))
    else:
        await tone_cmd.finish(MessageSegment.text("❌ 设置失败"))


style_cmd = on_command("style", aliases={"风格设置"}, priority=1, block=True)


@style_cmd.handle()
async def handle_style(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """查看/重置风格配置"""
    arg = args.extract_plain_text().strip().lower()

    if arg == "reset":
        result = await go_client.reset_style()
        if result:
            await style_cmd.finish(MessageSegment.text("✅ 风格已重置为默认"))
        else:
            await style_cmd.finish(MessageSegment.text("❌ 重置失败"))
        return

    style = await go_client.get_style()
    if style:
        length = style.get("length_preference", "medium")
        tone = style.get("tone", "") or "默认"
        emoji = "✅" if style.get("use_emoji", True) else "❌"
        speed = style.get("speed_multiplier", 1.0)
        custom = style.get("custom_style", "") or "无"

        msg = (
            f"🎨 当前风格配置\n"
            f"━━━━━━━━━━\n"
            f"回复长度: {length}\n"
            f"语气风格: {tone}\n"
            f"使用表情: {emoji}\n"
            f"回复速度: {speed}x\n"
            f"自定义: {custom}\n"
            f"━━━━━━━━━━\n"
            f"/style reset - 重置所有风格"
        )
        await style_cmd.finish(MessageSegment.text(msg))
    else:
        await style_cmd.finish(MessageSegment.text("❌ 无法获取风格配置"))


frequency_cmd = on_command("frequency", aliases={"频率", "概率"}, priority=1, block=True)


@frequency_cmd.handle()
async def handle_frequency(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """设置随机回复频率"""
    arg = args.extract_plain_text().strip()

    if not arg:
        await frequency_cmd.finish(MessageSegment.text(
            f"📊 当前随机回复概率: {config.random_probability:.0%}\n"
            f"━━━━━━━━━━\n"
            f"使用 /frequency <0-100> 设置\n"
            f"例: /frequency 50 (50%概率回复)"
        ))
        return

    try:
        value = int(arg)
        if value < 0 or value > 100:
            await frequency_cmd.finish(MessageSegment.text("❌ 范围: 0-100"))
            return

        config.random_probability = value / 100.0
        await frequency_cmd.finish(MessageSegment.text(f"✅ 随机回复概率已设置为: {value}%"))
    except ValueError:
        await frequency_cmd.finish(MessageSegment.text("❌ 请输入 0-100 的数字"))


# ============ 表达列表 ============
expressions_cmd = on_command("expressions", aliases={"表达", "新词"}, priority=1, block=True)


@expressions_cmd.handle()
async def handle_expressions(bot: Bot, event: GroupMessageEvent):
    """查看学到的表达"""
    expressions = await go_client.get_new_expressions()

    if expressions:
        # 只显示前 20 个
        display = expressions[:20]
        msg = f"📝 学到的表达 ({len(expressions)} 个)\n━━━━━━━━━━\n"
        msg += "\n".join([f"• {expr}" for expr in display])
        if len(expressions) > 20:
            msg += f"\n... 还有 {len(expressions) - 20} 个"
    else:
        msg = "📝 还没有学到新表达"

    await expressions_cmd.finish(MessageSegment.text(msg))


# ============ 话题列表 ============
topics_cmd = on_command("topics", aliases={"话题"}, priority=1, block=True)


@topics_cmd.handle()
async def handle_topics(bot: Bot, event: GroupMessageEvent):
    """查看最近话题"""
    topics = await go_client.get_recent_topics(limit=10)

    if topics:
        msg = f"💬 最近话题\n━━━━━━━━━━\n"
        for i, topic in enumerate(topics[:10], 1):
            heat = topic.get("heat", 0)
            heat_bar = "🔥" * int(heat * 5) if heat > 0 else ""
            msg += f"{i}. {topic.get('topic', 'N/A')[:30]} {heat_bar}\n"
    else:
        msg = "💬 还没有追踪到话题"

    await topics_cmd.finish(MessageSegment.text(msg))


# ============ 配置查看 ============
config_cmd = on_command("config", aliases={"配置"}, priority=1, block=True)


@config_cmd.handle()
async def handle_config(bot: Bot, event: GroupMessageEvent):
    """查看当前配置"""
    msg = (
        f"⚙️ 当前配置\n"
        f"━━━━━━━━━━\n"
        f"目标QQ: {config.target_qq}\n"
        f"目标群: {config.target_group_id or '所有群'}\n"
        f"触发模式: {', '.join(config.trigger_modes)}\n"
        f"随机概率: {config.random_probability:.0%}\n"
        f"回复延迟: {config.reply_delay_base}s/字\n"
        f"上下文数: {config.max_context}\n"
        f"━━━━━━━━━━\n"
        f"主动说话: {'✅' if _proactive_enabled else '❌'}\n"
        f"  检查间隔: {config.proactive_check_interval}s\n"
        f"  触发概率: {config.proactive_probability:.0%}\n"
        f"  最小间隔: {config.proactive_min_interval}s"
    )

    await config_cmd.finish(MessageSegment.text(msg))


# ============ 重置命令 ============
reset_cmd = on_command("reset", aliases={"重置"}, priority=1, block=True)


@reset_cmd.handle()
async def handle_reset(bot: Bot, event: GroupMessageEvent, args: Message = CommandArg()):
    """重置学习数据"""
    confirm = args.extract_plain_text().strip().lower()

    if confirm != "confirm":
        await reset_cmd.finish(MessageSegment.text(
            "⚠️ 此操作将清空所有学习数据！\n"
            "包括：反馈记录、回复历史、学到的表达、话题追踪\n\n"
            "如确认重置，请使用：/reset confirm"
        ))
        return

    # TODO: 调用 Go 服务的重置接口
    await reset_cmd.finish(MessageSegment.text("✅ 学习数据已重置（功能开发中）"))


# ============ 图片记忆命令 ============
images_cmd = on_command("images", aliases={"图片记忆", "看图"}, priority=1, block=True)


@images_cmd.handle()
async def handle_images(bot: Bot, event: GroupMessageEvent):
    """查看记住的图片"""
    group_id = str(event.group_id)
    images = await go_client.get_image_memories(group_id=group_id, limit=8)

    if images:
        msg = f"🖼️ 我记住的图片 ({len(images)} 张)\n━━━━━━━━━━\n"
        for img in images:
            sender = img.get("sender_name", "某人")
            desc = img.get("description", "")
            if len(desc) > 40:
                desc = desc[:40] + "..."
            msg += f"• {sender}：{desc}\n"
        msg += "━━━━━━━━━━\n@我并发图可以让我识别新图片"
    else:
        msg = "🖼️ 我还没有记住任何图片\n@我并发送图片，我就会识别并记住它"

    await images_cmd.finish(MessageSegment.text(msg))


vision_cmd = on_command("vision", aliases={"视觉状态"}, priority=1, block=True)


@vision_cmd.handle()
async def handle_vision(bot: Bot, event: GroupMessageEvent):
    """查看视觉模型状态"""
    status = await go_client.get_vision_status()
    enabled = status.get("enabled", False)
    model = status.get("model", "")

    if enabled:
        msg = f"👁️ 图片识别：✅ 已启用\n视觉模型：{model}\n\n@我并发图即可识别"
    else:
        msg = (
            "👁️ 图片识别：❌ 未启用\n\n"
            "需在 go-service/config.yaml 的 vision 节填入视觉模型配置（如 Qwen-VL / GLM-4V）并重启服务。"
        )

    await vision_cmd.finish(MessageSegment.text(msg))


# ============ 导出状态检查函数 ============
def is_muted() -> bool:
    """检查是否被禁言"""
    return _bot_muted


def is_proactive_enabled() -> bool:
    """检查主动说话是否开启"""
    return _proactive_enabled
