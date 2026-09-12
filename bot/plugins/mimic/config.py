from pydantic import BaseModel
from typing import List


class MimicConfig(BaseModel):
    """模仿机器人配置"""

    # Go 微服务地址
    go_service_url: str = "http://localhost:8080"

    # 目标成员 QQ 号
    target_qq: str = "123456789"

    # 目标群号（留空表示所有群都生效）
    target_group_id: str = ""

    # 触发模式
    trigger_modes: List[str] = ["at", "keyword", "random"]

    # 关键词触发列表
    trigger_keywords: List[str] = []

    # 随机触发概率 (0.0 ~ 1.0)
    random_probability: float = 0.3

    # 每字延迟基数（秒），模拟打字速度
    reply_delay_base: float = 0.05

    # 最大上下文消息数
    max_context: int = 10

    # 机器人的 QQ 号（用于判断 @）
    bot_qq: str = ""

    # ===== 主动说话配置 =====
    # 是否启用主动说话
    proactive_enabled: bool = True

    # 检查间隔（秒）
    proactive_check_interval: int = 60

    # 群安静多久后主动说话（秒），0 表示不需要安静
    proactive_quiet_threshold: int = 0

    # 主动说话概率 (0.0 ~ 1.0)
    proactive_probability: float = 0.5

    # 主动说话最小间隔（秒），避免太频繁
    proactive_min_interval: int = 180  # 3分钟
