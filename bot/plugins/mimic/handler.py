import asyncio
import random
import re
import time
from collections import defaultdict
from typing import Dict, List

from nonebot import on_message, logger, get_driver, get_bot
from nonebot.adapters.onebot.v11 import (
    Bot,
    GroupMessageEvent,
    Message,
    MessageSegment,
)

from .config import MimicConfig
from .go_client import GoClient
from .commands import is_muted, is_proactive_enabled

# ============ 全局实例 ============
config = MimicConfig()
go_client = GoClient(config.go_service_url)

# 群消息历史缓存（用于构建上下文）
_group_history: Dict[str, List[dict]] = defaultdict(list)

# 主动说话相关
_last_message_time: Dict[str, float] = {}  # 群最后一条消息时间
_last_proactive_time: Dict[str, float] = {}  # 群最后一次主动说话时间
_proactive_task: asyncio.Task = None


def _should_respond(event: GroupMessageEvent, bot_qq: str) -> bool:
    """判断是否应该回复"""
    # 检查是否被禁言
    if is_muted():
        return False
    
    # 使用 raw_message 获取原始 CQ 码格式
    raw_message = event.raw_message if hasattr(event, 'raw_message') else str(event.get_message())
    plain_text = event.get_plaintext()
    
    logger.debug(f"_should_respond: raw_message={raw_message}, bot_qq={bot_qq}, plain_text={plain_text}")

    # 忽略目标成员自己发的消息
    if str(event.user_id) == config.target_qq:
        return False

    # 忽略机器人自己发的消息
    if str(event.user_id) == bot_qq:
        return False

    # 如果配置了指定群号，只处理该群
    if config.target_group_id and str(event.group_id) != config.target_group_id:
        return False

    for mode in config.trigger_modes:
        if mode == "at":
            # 检查是否 @ 了机器人自己
            # OneBot V11 可能用 [CQ:at,qq=xxx] 或 [at:qq=xxx] 格式
            at_patterns = [
                rf"\[CQ:at,qq={bot_qq}\]",
                rf"\[at:qq={bot_qq}\]",
            ]
            for pattern in at_patterns:
                if re.search(pattern, raw_message):
                    return True
            # 也检查文本中 @名字 的情况
            if f"@{bot_qq}" in plain_text:
                return True

        elif mode == "keyword":
            for keyword in config.trigger_keywords:
                if keyword in plain_text:
                    return True

        elif mode == "random":
            if random.random() < config.random_probability:
                return True

    return False


def _add_to_history(group_id: str, sender_name: str, content: str):
    """将消息添加到群历史缓存"""
    _group_history[group_id].append({
        "sender": sender_name,
        "content": content,
    })
    # 保持最近 50 条
    if len(_group_history[group_id]) > 50:
        _group_history[group_id] = _group_history[group_id][-50:]
    # 记录最后消息时间
    _last_message_time[group_id] = time.time()


def _get_context(group_id: str, limit: int) -> List[dict]:
    """获取群聊最近 N 条消息作为上下文"""
    history = _group_history.get(group_id, [])
    return history[-limit:] if len(history) > limit else history


def _is_at_bot(event: GroupMessageEvent, bot_qq: str) -> bool:
    """判断该消息是否 @ 了机器人本人"""
    try:
        for seg in event.message:
            if seg.type == "at" and str(seg.data.get("qq", "")) == bot_qq:
                return True
    except Exception:
        pass
    # 兜底：检查原始 CQ 码
    raw_message = event.raw_message if hasattr(event, 'raw_message') else ""
    return bool(re.search(rf"\[CQ:at,qq={bot_qq}\]", raw_message))


def _extract_image_urls(event: GroupMessageEvent) -> List[str]:
    """从消息中提取图片 URL（优先 url，其次 file）"""
    urls: List[str] = []
    try:
        for seg in event.message:
            if seg.type == "image":
                url = seg.data.get("url") or seg.data.get("file")
                if url:
                    urls.append(url)
    except Exception as e:
        logger.warning(f"提取图片 URL 失败: {e}")
    return urls


# ============ 消息处理器 ============
mimic_handler = on_message(priority=10, block=False)


@mimic_handler.handle()
async def handle_group_message(bot: Bot, event: GroupMessageEvent):
    """处理群消息事件"""
    bot_qq = str(bot.self_id)
    group_id = str(event.group_id)
    sender_qq = str(event.user_id)
    sender_name = event.sender.nickname or event.sender.card or sender_qq
    plain_text = event.get_plaintext()

    # 所有群消息都记录到历史（用于上下文）
    _add_to_history(group_id, sender_name, plain_text)

    # 判断是否应该回复
    if not _should_respond(event, bot_qq):
        return

    logger.info(
        f"触发模仿回复 - 群: {group_id}, 发送者: {sender_name}({sender_qq}), "
        f"消息: {plain_text[:50]}"
    )

    # 获取上下文
    context = _get_context(group_id, config.max_context)

    # 仅当被 @ 时才提取并识别图片（节省视觉 API 调用）
    image_urls: List[str] = []
    if _is_at_bot(event, bot_qq):
        image_urls = _extract_image_urls(event)
        if image_urls:
            logger.info(f"检测到 {len(image_urls)} 张图片，将发送给视觉模型识别")

    # 调用 Go 服务
    result = await go_client.chat(
        group_id=group_id,
        sender_qq=sender_qq,
        sender_name=sender_name,
        message=plain_text,
        context=context,
        image_urls=image_urls,
    )

    if result is None:
        logger.warning("Go 服务未返回结果，跳过回复")
        return

    reply = result.get("reply", "")
    delay = result.get("delay", 0)

    if not reply:
        return

    # 模拟打字延迟
    if delay > 0:
        await asyncio.sleep(delay)
    else:
        # 默认延迟：基于回复长度
        await asyncio.sleep(len(reply) * config.reply_delay_base)

    # 发送回复
    await mimic_handler.finish(MessageSegment.text(reply))


# ============ 主动说话功能 ============

async def _proactive_speak_loop():
    """主动说话后台任务"""
    logger.info("主动说话任务已启动")
    
    while True:
        await asyncio.sleep(config.proactive_check_interval)
        
        if not config.proactive_enabled or not is_proactive_enabled():
            continue
        
        try:
            bot = get_bot()
            if not bot:
                continue
            
            bot_qq = str(bot.self_id)
            now = time.time()
            
            # 构建要检查的群列表：已知群 + 配置的目标群
            groups_to_check = set(_last_message_time.keys())
            if config.target_group_id:
                groups_to_check.add(config.target_group_id)
            
            logger.debug(f"主动说话检查: {len(groups_to_check)} 个群")
            
            for group_id in groups_to_check:
                last_msg_time = _last_message_time.get(group_id, 0)
                last_proactive = _last_proactive_time.get(group_id, 0)
                
                # 检查是否安静足够久（如果阈值为0则跳过此检查）
                if config.proactive_quiet_threshold > 0:
                    quiet_duration = now - last_msg_time
                    if quiet_duration < config.proactive_quiet_threshold:
                        continue
                
                # 检查是否距离上次主动说话足够久
                if now - last_proactive < config.proactive_min_interval:
                    logger.debug(f"群 {group_id} 距离上次主动说话太近，跳过")
                    continue
                
                # 概率触发
                if random.random() > config.proactive_probability:
                    logger.debug(f"群 {group_id} 概率未触发，跳过")
                    continue
                
                # 生成主动消息
                logger.info(f"主动说话触发 - 群: {group_id}")
                
                # 获取上下文
                context = _get_context(group_id, config.max_context)
                
                # 调用 Go 服务，使用特殊提示
                result = await go_client.chat(
                    group_id=group_id,
                    sender_qq="system",
                    sender_name="系统",
                    message="（群里安静了一会儿，你主动说点什么吧）",
                    context=context,
                )
                
                if result and result.get("reply"):
                    reply = result["reply"]
                    delay = result.get("delay", 0)
                    
                    if delay > 0:
                        await asyncio.sleep(delay)
                    
                    # 发送主动消息
                    await bot.send_group_msg(
                        group_id=int(group_id),
                        message=MessageSegment.text(reply)
                    )
                    
                    # 记录主动说话时间
                    _last_proactive_time[group_id] = time.time()
                    
                    logger.info(f"主动说话成功 - 群: {group_id}, 内容: {reply[:30]}")
                    
                    # 每次只在一个群主动说话，避免同时多个群
                    break
                    
        except Exception as e:
            logger.error(f"主动说话任务出错: {e}")


# 启动时注册后台任务
driver = get_driver()

@driver.on_startup
async def start_proactive_task():
    global _proactive_task
    if config.proactive_enabled:
        _proactive_task = asyncio.create_task(_proactive_speak_loop())
        logger.info("主动说话功能已启用")

@driver.on_shutdown
async def stop_proactive_task():
    global _proactive_task
    if _proactive_task:
        _proactive_task.cancel()
        logger.info("主动说话任务已停止")
