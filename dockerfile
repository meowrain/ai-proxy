# ---- Build Stage ----
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 只复制 go.mod 和 go.sum 先拉依赖（加速缓存）
COPY go.mod go.sum ./
RUN go mod download

# 再复制源代码
COPY . .

# 编译（静态链接，去除调试信息）
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o ai-proxy .

# ---- Run Stage ----
FROM alpine:latest

# 创建工作目录
WORKDIR /app

# 从构建阶段复制编译好的二进制
COPY --from=builder /app/ai-proxy .

COPY --from=builder /app/api.json .
# 暴露端口（如有需要）
EXPOSE 8094

# 启动命令（推荐 JSON 格式）
CMD ["./ai-proxy"]