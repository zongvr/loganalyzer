# 华傲数据考核 - 日志分析 CLI 容器化
# 多阶段构建：build 阶段编译 + 质量门，runtime 阶段用 scratch 零依赖最小镜像。
# 行为契约：外部 text/json 输出必须与 round 9 (4f6d8d1) 逐项字节一致；本 Dockerfile 仅打包，不改任何 Go 源码。

# ---- build 阶段：编译 + 测试质量门 ----
FROM golang:1.22-alpine AS build
WORKDIR /src

# 先只拷贝 go.mod 以充分利用构建缓存（零第三方依赖，此步极快）。
COPY go.mod ./
RUN go mod download

# 拷贝全部源码（.dockerignore 已排除非构建必需项）。
COPY . .

# 质量门：在构建阶段跑单测，失败则镜像构建失败。
RUN go test ./...

# 静态编译：CGO_ENABLED=0 保证产物不依赖 libc；-s -w 去除符号表与调试信息瘦身。
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /loganalyzer ./cmd/loganalyzer

# ---- runtime 阶段：scratch 零依赖最小镜像 ----
FROM scratch

LABEL maintainer="华傲数据考核"
LABEL description="日志分析 CLI (loganalyzer) - 多阶段构建最小镜像"
LABEL version="round12"

# 仅拷贝编译产物，无 libc / ca-certificates / shell 依赖（本工具纯本地文件处理，无 TLS/网络需求）。
COPY --from=build /loganalyzer /loganalyzer

ENTRYPOINT ["/loganalyzer"]
