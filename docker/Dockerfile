# 使用官方Go镜像作为构建环境（须与 go.mod 的 go 指令匹配）
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用（不写死 GOARCH，跟随构建平台，ARM 服务器上也能原生构建）
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o discord-lfg-bot \
    ./cmd/bot

# 使用scratch作为最终镜像（最小化）
FROM scratch

# 从builder阶段复制必要文件
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/discord-lfg-bot /discord-lfg-bot

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非root用户（在scratch中需要手动创建）
# 使用数字ID而不是用户名
USER 1000:1000

# 暴露健康检查端口
EXPOSE 8080

# 健康检查（scratch 镜像无 shell，必须用 exec 形式；探针为二进制自带的 -health-check 模式）
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD ["/discord-lfg-bot", "-health-check"]

# 运行应用
ENTRYPOINT ["/discord-lfg-bot"] 