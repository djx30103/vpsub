# 使用多阶段构建
FROM golang:1.24-alpine AS builder

WORKDIR /build

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -trimpath -ldflags "-s -w" -o vpsub ./cmd/server

# 最终镜像
FROM alpine:latest

WORKDIR /app

# 添加时区数据
RUN apk add --no-cache tzdata

# 创建必要的目录
RUN mkdir -p /app/config /app/subscriptions /app/default_config && \
    chmod 755 /app/config /app/subscriptions

# 复制配置文件到默认位置
COPY config/config.yml /app/default_config/config.yml

# 设置时区
ENV TZ=Asia/Shanghai

# 设置默认配置文件路径（可被运行时环境变量覆盖）
ENV VPSUB_CONF_PATH=/app/config/config.yml

# 从构建阶段复制二进制文件
COPY --from=builder /build/vpsub /app/vpsub

# 复制启动脚本
COPY docker-entrypoint.sh /app/
RUN chmod +x /app/docker-entrypoint.sh

EXPOSE 30103

# 使用启动脚本
ENTRYPOINT ["/app/docker-entrypoint.sh"]
