# QQ 群仿生机器人

一个能够模仿指定 QQ 群成员说话风格的群机器人。通过导入该成员的聊天记录，构建人物画像，并使用 LLM 以该成员的身份在群中聊天。

## 功能特性

* **人物画像构建**：从聊天记录中自动提取口头禅、常用词、句式特征、话题偏好、回复模式

* **LLM 模仿回复**：基于 DeepSeek（兼容 OpenAI 格式）生成符合目标成员风格的回复

* **多种触发方式**：@机器人、关键词、随机概率触发

* **群聊上下文**：保留最近 N 条群消息作为对话上下文

* **模拟打字延迟**：根据回复长度计算延迟，更像真人

* **多格式支持**：支持 QQChatExporter 导出的 JSON 格式和简单 JSON/TXT 格式

## 架构

```
┌─────────────┐     WebSocket      ┌──────────────┐     HTTP      ┌─────────────────┐
│  SnowLuma   │ ◄────────────────► │   NoneBot2   │ ◄───────────► │   Go 微服务      │
│ (QQ 协议端)  │    OneBot v11      │  (消息路由)    │   REST API    │  (AI 核心逻辑)   │
└─────────────┘                    └──────────────┘               └─────────────────┘
      │                                   │                              │
      ▼                                   ▼                              ▼
   QQ 服务器                         群消息事件                  人物画像 + LLM 调用
```

### 组件说明

| 组件       | 技术             | 职责                   |
| -------- | -------------- | -------------------- |
| SnowLuma | OneBot v11 协议端 | QQ 消息收发（进程注入方式）      |
| NoneBot2 | Python         | 消息路由、触发判断、回复发送       |
| Go 微服务   | Go + Gin       | 聊天记录解析、人物画像构建、LLM 调用 |

## 目录结构

```
QQ_bot/
├── bot/                          # NoneBot2 机器人
│   ├── bot.py                    # 入口文件
│   ├── .env                      # 环境配置
│   ├── pyproject.toml            # Python 依赖
│   └── plugins/
│       └── mimic/                # 模仿插件
│           ├── __init__.py       # 插件注册
│           ├── config.py         # 插件配置
│           ├── handler.py        # 消息处理（触发判断、回复发送）
│           └── go_client.py      # Go 服务 HTTP 客户端
│
├── go-service/                   # Go 微服务
│   ├── main.go                   # 入口文件
│   ├── config.yaml               # 服务配置
│   ├── go.mod / go.sum           # Go 依赖
│   ├── internal/
│   │   ├── config/config.go      # 配置加载
│   │   ├── history/parser.go     # 聊天记录解析（支持 QQChatExporter）
│   │   ├── profile/
│   │   │   ├── builder.go        # 人物画像构建
│   │   │   └── store.go          # 画像持久化
│   │   ├── llm/
│   │   │   ├── client.go         # LLM HTTP 客户端
│   │   │   └── prompt.go         # Prompt 构建
│   │   └── chat/
│   │       ├── handler.go        # API 处理函数
│   │       └── context.go        # 群聊上下文管理
│   └── data/
│       ├── history/              # 聊天记录存放目录
│       └── profiles/             # 人物画像存放目录
│
└── .gitignore
```

## 环境要求

* **Go** >= 1.21

* **Python** >= 3.10

* **SnowLuma**（或任意 OneBot v11 实现）

* **LLM API Key**（DeepSeek / 通义千问 / 智谱，均兼容 OpenAI 格式）

## 安装步骤

### 1. 安装 Go 依赖

```bash
cd go-service
go mod tidy
go build -o mimic-server.exe .
```

### 2. 安装 Python 依赖

```bash
cd bot
pip install nonebot2[fastapi] nonebot-adapter-onebot httpx websockets
```

### 3. 配置 Go 服务

编辑 `go-service/config.yaml`：

```yaml
server:
  port: 8080

llm:
  provider: "deepseek"           # 或 qwen / zhipu
  api_key: "sk-xxx"              # 你的 API Key
  model: "deepseek-flash"        # DeepSeek-V4.1-Flash 的 API 参数名（deepseek-chat 已弃用）
  base_url: "https://api.deepseek.com/v1"
  max_tokens: 1000
  temperature: 1.05
  disable_thinking: true         # deepseek-flash 默认开思考，须关闭，否则 content 为空且采样参数失效

target:
  qq: "目标成员QQ号"              # 要模仿的人
  group_id: "群号"               # 机器人所在的群

response:
  trigger_modes:
    - "at"                       # @机器人触发
    - "keyword"                  # 关键词触发
    - "random"                   # 随机触发
  trigger_keywords: []
  random_probability: 0.3
  reply_delay_base: 0.02         # 每字延迟（秒）
  max_context: 10                # 上下文条数
```

### 4. 配置 NoneBot2

编辑 `bot/.env`：

```ini
HOST=0.0.0.0
PORT=8081
DRIVER=~fastapi+~websockets
LOG_LEVEL=INFO

# OneBot V11 配置（SnowLuma WebSocket）
ONEBOT_ACCESS_TOKEN=你的SnowLuma Token
ONEBOT_WS_URLS=["ws://127.0.0.1:3001"]
```

### 5. 配置 SnowLuma

1. 下载并启动 SnowLuma
2. 打开 WebUI（默认 http://localhost:5099）
3. 在「节点配置」中新建 WebSocket 服务端，端口 3001
4. 设置授权 Token（与 .env 中一致）
5. 在「进程注入」中加载已登录的 QQ 客户端

## 使用方法

### 启动服务

```bash
# 终端 1：启动 Go 微服务
cd go-service
./mimic-server.exe

# 终端 2：启动 NoneBot2
cd bot
python bot.py
```

### 导入聊天记录

将 QQChatExporter 导出的 JSON 文件放入 `go-service/data/history/`，然后：

```bash
curl -X POST http://localhost:8080/api/import \
  -F "file=@data/history/群聊记录1.json" \
  -F "file=@data/history/群聊记录2.json"
```

或使用 Python：

```python
import httpx

files = [
    ("file", ("group1.json", open("group1.json", "rb"), "application/json")),
    ("file", ("group2.json", open("group2.json", "rb"), "application/json")),
]
resp = httpx.post("http://localhost:8080/api/import", files=files, timeout=120)
print(resp.json())
```

### 查看人物画像

```bash
curl http://localhost:8080/api/profile
```

### 查看统计

```bash
curl http://localhost:8080/api/stats
```

### 群内管理命令

| 命令               | 功能       |
| ---------------- | -------- |
| `/mimic stats`   | 查看机器人统计  |
| `/mimic profile` | 查看当前人物画像 |

## API 接口

| 方法   | 路径             | 说明            |
| ---- | -------------- | ------------- |
| POST | `/api/import`  | 上传聊天记录（支持多文件） |
| GET  | `/api/profile` | 获取当前人物画像      |
| POST | `/api/chat`    | 发送消息获取模仿回复    |
| PUT  | `/api/config`  | 运行时更新配置       |
| GET  | `/api/stats`   | 获取统计信息        |

### /api/chat 请求示例

```json
{
  "group_id": "你的群号",
  "sender_qq": "发送者QQ号",
  "sender_name": "发送者昵称",
  "message": "今晚打游戏吗",
  "context": [
    {"sender": "群友A", "content": "上一把太菜了"},
    {"sender": "发送者昵称", "content": "确实"}
  ]
}
```

### /api/chat 响应示例

```json
{
  "reply": "打啥游戏",
  "delay": 0.16
}
```

## 聊天记录格式

### QQChatExporter 格式（推荐）

使用 [QQChatExporter](https://github.com/shuakami/qq-chat-exporter) 导出的 JSON 文件，系统会自动解析。

### 简单 JSON 格式

```json
[
  {"time": "2024-01-01 10:00:00", "sender": "张三", "qq": "123456", "content": "消息内容"},
  {"time": "2024-01-01 10:01:00", "sender": "李四", "qq": "789012", "content": "另一条消息"}
]
```

### TXT 格式

```
2024-01-01 10:00:00 张三(123456)
消息内容

2024-01-01 10:01:00 李四(789012)
另一条消息
```

## 人物画像维度

系统从聊天记录中提取以下特征：

| 维度     | 说明                     |
| ------ | ---------------------- |
| 口头禅    | 高频 2-4 字中文片段           |
| 常用词/表情 | 网络用语、语气词（6、牛、草、寄、GG 等） |
| 句式特征   | 短句偏好、反问频率、感叹号使用        |
| 话题偏好   | 游戏、编程、动漫、美食、生活等        |
| 回复模式   | 对问题/打招呼/争议话题的反应方式      |
| 样本消息   | 目标成员的全部历史发言（作为 LLM 参考） |

## 触发机制

| 模式        | 说明                               |
| --------- | -------------------------------- |
| `at`      | 群成员 @机器人 时触发                     |
| `keyword` | 消息包含 `trigger_keywords` 中的词时触发   |
| `random`  | 每条群消息有 `random_probability` 概率触发 |

## 常见问题

### Q: 机器人不回复？

1. 确认 SnowLuma 已加载且 QQ 已登录
2. 确认 NoneBot2 日志显示 `Bot xxx connected`
3. 确认已导入聊天记录（`/api/profile` 有返回）
4. 确认触发条件满足（@机器人 或关键词）

### Q: 回复不像目标成员？

1. 增加聊天记录量（建议 1000+ 条）
2. 检查画像是否正确（`/api/profile`）
3. 调整 `temperature`（越低越保守，越高越有创意）

### Q: 响应慢？

1. 检查 LLM API 网络延迟
2. 降低 `max_context` 减少上下文长度
3. 降低 `reply_delay_base` 减少模拟打字延迟

## 许可证

MIT License
