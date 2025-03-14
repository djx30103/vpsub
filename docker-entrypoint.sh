#!/bin/sh

# 获取配置文件目录路径（从环境变量中提取目录部分）
CONFIG_DIR=$(dirname "$VPSUB_CONF_PATH")
CONFIG_FILE=$(basename "$VPSUB_CONF_PATH")

# 确保配置目录存在
mkdir -p "$CONFIG_DIR"

# 检查配置文件是否存在，不存在则复制默认配置
if [ ! -f "$VPSUB_CONF_PATH" ]; then
    echo "配置文件不存在，使用默认配置..."
    cp /app/default_config/config.yml "$VPSUB_CONF_PATH"
fi

# 启动应用
exec /app/vpsub "$@"
