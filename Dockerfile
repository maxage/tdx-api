# 多阶段构建 - 第一阶段：构建
FROM golang:1.22-alpine AS builder

# 设置工作目录
WORKDIR /app

# 设置Go代理（使用国内镜像加速）
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# 复制整个项目（需要完整的依赖关系）
COPY . .

# 下载主项目依赖
RUN go mod download

# 编译web应用
WORKDIR /app/web
RUN go mod tidy && go build -ldflags="-s -w" -o stock-web .

# 多阶段构建 - 第二阶段：运行
FROM alpine:latest

# 安装必要的运行时依赖（包括 wget 用于健康检查）
RUN apk --no-cache add ca-certificates tzdata wget

# 设置时区为上海
ENV TZ=Asia/Shanghai

# 创建非root用户
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /app/web/stock-web .

# 复制静态文件
COPY --from=builder /app/web/static ./static

# 更改文件所有者
RUN chown -R appuser:appuser /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查（使用 API 健康检查端点）
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# 启动应用
CMD ["./stock-web"]

