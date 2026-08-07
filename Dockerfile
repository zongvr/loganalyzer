# 华傲数据考核 - 日志分析 CLI 容器化模板
# 由候选人在「容器化」轮次中根据最终目录结构补全并验证。
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /loganalyzer ./cmd/loganalyzer

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /loganalyzer /usr/local/bin/loganalyzer
ENTRYPOINT ["loganalyzer"]
