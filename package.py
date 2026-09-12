import zipfile
import os

root = r"d:\Git\QQ_bot"
output = r"d:\Git\QQ_bot\qq-mimic-bot-source.zip"

# 排除规则
exclude_dirs = {
    "__pycache__", ".git", ".venv", "node_modules",
    "egg-info", "qq_mimic_bot.egg-info",
    "history",  # 私密聊天记录
    "profiles", # 人物画像数据
}
exclude_files = {
    ".env",           # 含 Token
    "config.yaml",    # 含 API Key
    "mimic-server.exe",
    "mimic-server.exe~",
    "qq-mimic-bot-source.zip",
}
exclude_exts = {".exe", ".pyc", ".zip"}

count = 0
with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as zf:
    for dirpath, dirnames, filenames in os.walk(root):
        # 过滤目录
        dirnames[:] = [d for d in dirnames if d not in exclude_dirs and not d.endswith(".egg-info")]
        for fn in filenames:
            if fn in exclude_files:
                continue
            ext = os.path.splitext(fn)[1].lower()
            if ext in exclude_exts:
                continue
            full = os.path.join(dirpath, fn)
            arcname = os.path.relpath(full, root)
            zf.write(full, arcname)
            count += 1

size_mb = os.path.getsize(output) / 1024 / 1024
print(f"打包完成: {output}")
print(f"包含文件数: {count}")
print(f"压缩包大小: {size_mb:.2f} MB")
