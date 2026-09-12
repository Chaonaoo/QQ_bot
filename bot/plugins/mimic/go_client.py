import httpx
from typing import Optional, List, Dict, Any
from nonebot import logger


class GoClient:
    """Go 微服务 HTTP 客户端"""

    def __init__(self, base_url: str, timeout: float = 60.0):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self._client: Optional[httpx.AsyncClient] = None

    async def _get_client(self) -> httpx.AsyncClient:
        if self._client is None or self._client.is_closed:
            self._client = httpx.AsyncClient(timeout=self.timeout)
        return self._client

    async def close(self):
        if self._client and not self._client.is_closed:
            await self._client.aclose()

    async def chat(
        self,
        group_id: str,
        sender_qq: str,
        sender_name: str,
        message: str,
        context: List[Dict[str, str]],
        image_urls: Optional[List[str]] = None,
    ) -> Optional[Dict[str, Any]]:
        """
        调用 Go 服务的 /api/chat 接口，获取模仿回复。

        Returns:
            {"reply": "回复内容", "delay": 0.5} 或 None（出错时）
        """
        client = await self._get_client()
        payload = {
            "group_id": group_id,
            "sender_qq": sender_qq,
            "sender_name": sender_name,
            "message": message,
            "context": context,
            "image_urls": image_urls or [],
        }

        try:
            resp = await client.post(
                f"{self.base_url}/api/chat",
                json=payload,
            )
            resp.raise_for_status()
            return resp.json()
        except httpx.HTTPStatusError as e:
            logger.error(f"Go 服务返回错误状态: {e.response.status_code} - {e.response.text}")
            return None
        except httpx.RequestError as e:
            logger.error(f"请求 Go 服务失败: {e}")
            return None

    async def import_history(self, file_path: str) -> Optional[Dict[str, Any]]:
        """上传聊天记录文件到 Go 服务"""
        client = await self._get_client()
        try:
            with open(file_path, "rb") as f:
                resp = await client.post(
                    f"{self.base_url}/api/import",
                    files={"file": f},
                )
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"导入聊天记录失败: {e}")
            return None

    async def get_profile(self) -> Optional[Dict[str, Any]]:
        """获取当前人物画像"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/profile")
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"获取人物画像失败: {e}")
            return None

    async def get_stats(self) -> Optional[Dict[str, Any]]:
        """获取统计信息"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/stats")
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"获取统计信息失败: {e}")
            return None

    # ============ 自学习接口 ============

    async def get_learning_stats(self) -> Optional[Dict[str, Any]]:
        """获取学习统计"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/learning/stats")
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"获取学习统计失败: {e}")
            return None

    async def get_learning_context(self) -> str:
        """获取学习上下文（用于 prompt）"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/learning/context")
            resp.raise_for_status()
            data = resp.json()
            return data.get("context", "")
        except Exception as e:
            logger.error(f"获取学习上下文失败: {e}")
            return ""

    async def get_recent_feedbacks(self, limit: int = 10) -> List[Dict[str, Any]]:
        """获取最近反馈"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/learning/feedbacks", params={"limit": limit})
            resp.raise_for_status()
            data = resp.json()
            return data.get("feedbacks", [])
        except Exception as e:
            logger.error(f"获取反馈失败: {e}")
            return []

    async def get_new_expressions(self) -> List[str]:
        """获取学到的新表达"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/learning/expressions")
            resp.raise_for_status()
            data = resp.json()
            return data.get("expressions", [])
        except Exception as e:
            logger.error(f"获取新表达失败: {e}")
            return []

    async def get_recent_topics(self, limit: int = 10) -> List[Dict[str, Any]]:
        """获取最近话题"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/learning/topics", params={"limit": limit})
            resp.raise_for_status()
            data = resp.json()
            return data.get("topics", [])
        except Exception as e:
            logger.error(f"获取话题失败: {e}")
            return []

    async def submit_feedback(
        self,
        group_id: str,
        sender_qq: str,
        sender_name: str,
        content: str,
        bot_reply: str = ""
    ) -> Optional[Dict[str, Any]]:
        """提交反馈"""
        client = await self._get_client()
        payload = {
            "group_id": group_id,
            "sender_qq": sender_qq,
            "sender_name": sender_name,
            "content": content,
            "bot_reply": bot_reply,
        }
        try:
            resp = await client.post(f"{self.base_url}/api/learning/feedback", json=payload)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"提交反馈失败: {e}")
            return None

    # ============ 自我特质 API ============

    async def add_self_trait(
        self,
        key: str,
        description: str = "",
        response: str = "",
        created_by: str = ""
    ) -> Optional[Dict[str, Any]]:
        """添加自我特质"""
        client = await self._get_client()
        payload = {
            "key": key,
            "description": description,
            "response": response,
            "created_by": created_by,
        }
        try:
            resp = await client.post(f"{self.base_url}/api/learning/traits", json=payload)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"添加自我特质失败: {e}")
            return None

    async def get_self_traits(self) -> List[Dict[str, Any]]:
        """获取所有自我特质"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/learning/traits")
            resp.raise_for_status()
            data = resp.json()
            return data.get("traits", [])
        except Exception as e:
            logger.error(f"获取自我特质失败: {e}")
            return []

    async def remove_self_trait(self, key: str) -> bool:
        """移除自我特质"""
        client = await self._get_client()
        try:
            resp = await client.delete(f"{self.base_url}/api/learning/traits/{key}")
            resp.raise_for_status()
            return True
        except Exception as e:
            logger.error(f"移除自我特质失败: {e}")
            return False

    # ============ 风格配置 API ============

    async def get_style(self) -> Optional[Dict[str, Any]]:
        """获取当前风格配置"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/style")
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"获取风格配置失败: {e}")
            return None

    async def update_style(
        self,
        length_preference: Optional[str] = None,
        tone: Optional[str] = None,
        custom_style: Optional[str] = None,
        use_emoji: Optional[bool] = None,
        speed_multiplier: Optional[float] = None
    ) -> Optional[Dict[str, Any]]:
        """更新风格配置"""
        client = await self._get_client()
        payload = {}
        if length_preference is not None:
            payload["length_preference"] = length_preference
        if tone is not None:
            payload["tone"] = tone
        if custom_style is not None:
            payload["custom_style"] = custom_style
        if use_emoji is not None:
            payload["use_emoji"] = use_emoji
        if speed_multiplier is not None:
            payload["speed_multiplier"] = speed_multiplier

        try:
            resp = await client.put(f"{self.base_url}/api/style", json=payload)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"更新风格配置失败: {e}")
            return None

    async def reset_style(self) -> Optional[Dict[str, Any]]:
        """重置风格配置"""
        client = await self._get_client()
        try:
            resp = await client.post(f"{self.base_url}/api/style/reset")
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"重置风格配置失败: {e}")
            return None

    # ============ 图片记忆 API ============

    async def get_image_memories(
        self, group_id: str = "", limit: int = 10
    ) -> List[Dict[str, Any]]:
        """获取图片记忆"""
        client = await self._get_client()
        params = {"limit": limit}
        if group_id:
            params["group_id"] = group_id
        try:
            resp = await client.get(f"{self.base_url}/api/learning/images", params=params)
            resp.raise_for_status()
            data = resp.json()
            return data.get("images", [])
        except Exception as e:
            logger.error(f"获取图片记忆失败: {e}")
            return []

    async def get_vision_status(self) -> Dict[str, Any]:
        """获取视觉模型启用状态"""
        client = await self._get_client()
        try:
            resp = await client.get(f"{self.base_url}/api/vision/status")
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"获取视觉状态失败: {e}")
            return {"enabled": False, "model": ""}
