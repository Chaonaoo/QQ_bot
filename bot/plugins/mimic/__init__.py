from nonebot import get_driver
from nonebot.plugin import PluginMetadata

from .config import MimicConfig
from .go_client import GoClient
from . import handler
from . import commands  # 加载指令系统

__plugin_meta__ = PluginMetadata(
    name="mimic",
    description="QQ群模仿机器人 - 模仿指定成员的说话风格",
    usage="自动回复，使用 /help 查看所有命令",
)

driver = get_driver()


@driver.on_startup
async def _on_startup():
    """启动时加载配置"""
    # 从 NoneBot 配置中读取（如有）
    try:
        from nonebot import get_plugin_config
        plugin_config = get_plugin_config(MimicConfig)
        if plugin_config:
            handler.config = plugin_config
            handler.go_client = GoClient(plugin_config.go_service_url)
    except Exception:
        pass

    handler.go_client = GoClient(handler.config.go_service_url)


@driver.on_shutdown
async def _on_shutdown():
    """关闭时清理资源"""
    await handler.go_client.close()
