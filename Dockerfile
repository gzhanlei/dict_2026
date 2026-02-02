# 多阶段构建
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dict_new .

# 运行阶段
FROM alpine:latest

# 安装ca-certificates用于HTTPS请求
RUN apk --no-cache add ca-certificates

# 创建应用目录
WORKDIR /root/

# 从builder阶段复制可执行文件
COPY --from=builder /app/dict_new .

# 复制静态文件和配置文件
COPY --from=builder /app/static ./static
COPY --from=builder /app/config ./config

# 创建缓存和日志目录
RUN mkdir -p cache logs

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./dict_new"]