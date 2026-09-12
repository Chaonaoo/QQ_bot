#!/usr/bin/env python3
"""
QQ 模仿机器人 - 开源版本打包脚本
创建干净的源码包，排除敏感信息
"""

import os
import sys
import zipfile
from pathlib import Path

# 设置输出编码
if sys.platform == 'win32':
    sys.stdout.reconfigure(encoding='utf-8')

# 项目根目录
ROOT = Path(__file__).parent

# 输出文件
OUTPUT = ROOT / "qq-mimic-bot-opensource.zip"

# 要包含的目录
INCLUDE_DIRS = [
    "bot",
    "go-service",
]

# 要包含的根目录文件
INCLUDE_FILES = [
    "README.md",
    ".gitignore",
    "package.py",
    "package_opensource.py",
]

# 要排除的模式
EXCLUDE_PATTERNS = [
    # 敏感配置
    ".env",
    ".env.prod",
    "config.yaml",  # 只保留 config.example.yaml
    
    # Python
    "__pycache__",
    "*.pyc",
    "*.pyo",
    "*.pyd",
    ".venv",
    "venv",
    "env",
    
    # Go
    "*.exe",
    "go-service",  # 编译后的二进制
    "data/",  # 运行时数据
    
    # IDE
    ".vscode",
    ".idea",
    "*.swp",
    "*.swo",
    
    # OS
    "Thumbs.db",
    ".DS_Store",
    "desktop.ini",
    
    # 打包文件
    "*.zip",
    
    # 日志
    "*.log",
    "logs/",
]


def should_exclude(path: Path) -> bool:
    """检查路径是否应该被排除"""
    path_str = str(path)
    name = path.name
    
    for pattern in EXCLUDE_PATTERNS:
        # 目录匹配
        if pattern.endswith("/"):
            if pattern[:-1] in path_str:
                return True
        # 通配符匹配
        elif "*" in pattern:
            import fnmatch
            if fnmatch.fnmatch(name, pattern):
                return True
        # 精确匹配
        elif name == pattern or path_str.endswith(pattern):
            return True
    
    return False


def get_files():
    """获取所有要打包的文件"""
    files = []
    
    # 根目录文件
    for fname in INCLUDE_FILES:
        fpath = ROOT / fname
        if fpath.exists():
            files.append(fpath)
    
    # 目录文件
    for dirname in INCLUDE_DIRS:
        dirpath = ROOT / dirname
        if not dirpath.exists():
            continue
        
        for root, dirs, filenames in os.walk(dirpath):
            # 过滤目录
            dirs[:] = [d for d in dirs if not should_exclude(Path(root) / d)]
            
            for filename in filenames:
                fpath = Path(root) / filename
                if not should_exclude(fpath):
                    files.append(fpath)
    
    return files


def create_readme_addendum():
    """创建开源版本说明"""
    return """
# QQ 模仿机器人 - 开源版本

## 快速开始

### 1. 配置 Go 服务
```bash
cd go-service
cp config.example.yaml config.yaml
# 编辑 config.yaml，填入你的 API Key 和目标 QQ
```

### 2. 配置 NoneBot2
```bash
cd bot
cp .env.example .env
# 编辑 .env，填入你的 OneBot Token
```

### 3. 启动服务
```bash
# 终端 1: 启动 Go 服务
cd go-service
go run main.go

# 终端 2: 启动 NoneBot2
cd bot
python bot.py
```

## 功能特性

- 🤖 模仿指定成员的说话风格
- 🧠 自学习机制（增量学习 + 反馈调整）
- 💬 主动说话（可配置概率和间隔）
- 📊 完整的指令系统（/help 查看）
- 🎭 分层学习池（保护核心画像）

## 指令列表

发送 `/help` 查看完整指令列表。

## 注意事项

1. 需要先配置 SnowLuma 或其他 OneBot 实现
2. 需要 LLM API Key（支持 DeepSeek/通义千问/智谱等）
3. 首次使用需要导入聊天记录生成画像

## 开源协议

MIT License
"""


def main():
    print("🔍 扫描项目文件...")
    files = get_files()
    
    print(f"📦 找到 {len(files)} 个文件")
    
    # 创建 zip
    print(f"📝 创建打包文件: {OUTPUT.name}")
    with zipfile.ZipFile(OUTPUT, "w", zipfile.ZIP_DEFLATED) as zf:
        for fpath in files:
            arcname = fpath.relative_to(ROOT)
            zf.write(fpath, arcname)
            print(f"  + {arcname}")
        
        # 添加开源说明
        zf.writestr("OPENSOURCE_README.md", create_readme_addendum())
        print("  + OPENSOURCE_README.md")
    
    size_mb = OUTPUT.stat().st_size / (1024 * 1024)
    print(f"\n✅ 打包完成: {OUTPUT}")
    print(f"   大小: {size_mb:.2f} MB")
    print(f"   文件数: {len(files) + 1}")


if __name__ == "__main__":
    main()
