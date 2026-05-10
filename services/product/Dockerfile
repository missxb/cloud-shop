# 多阶段构建 - 编译阶段
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 启用Go模块代理
ENV GOPROXY=https://goproxy.cn,direct

# 复制go.mod和go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o product main.go

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 复制编译好的二进制文件
COPY --from=builder /app/product .

# 暴露端口
EXPOSE 8081

# 运行
CMD ["./product"]
