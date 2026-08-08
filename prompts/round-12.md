# Round 12 — 编写 Dockerfile 并构建镜像（容器化 / 零依赖静态编译）

【提示词正文】

## 目标
将 loganalyzer 容器化，产出可在任意支持 OCI 的运行时（Docker / containerd / Kubernetes）直接拉起的最小镜像。本轮**只新增容器化文件，严禁改动任何 Go 源码或测试**（round 11 完成后的行为契约是硬约束，外部 text/json 输出必须与 round 9 `4f6d8d1` 逐项字节一致）。

## 现状（已存在占位文件，需完善而非从零）
仓库根已有一个占位 `Dockerfile`（Aug 7 脚手架提交，多阶段构建雏形，但运行阶段用 `alpine:latest`、无 `.dockerignore`、无测试质量门、未用 `scratch`、无静态链接裁剪）。本轮请**重写该 Dockerfile** 为生产级版本，并**新增 `.dockerignore`**。

## 目标 Dockerfile 规格（多阶段构建）
- **build 阶段**：`FROM golang:1.22-alpine AS build`（与 `go.mod` 的 `go 1.22` 对齐）。
  - `WORKDIR /src`、`COPY go.mod ./`、`RUN go mod download`（零第三方依赖时此步极快，仍保留以符合标准实践）。
  - `COPY . .`。
  - **质量门**：`RUN go test ./...`（在 build 阶段跑单测，失败则镜像构建失败；体现工程严谨，依赖 round 10/11 已落地的 21 个测试）。
  - **静态编译**：`RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /loganalyzer ./cmd/loganalyzer`。`CGO_ENABLED=0` 保证产物不依赖 libc；`-s -w` 去除符号表与调试信息瘦身。
- **runtime 阶段**：`FROM scratch`（真正零依赖、最小攻击面——本 CLI 纯本地文件处理、无网络/TLS/ca 需求，`scratch` 足够）。
  - `COPY --from=build /loganalyzer /loganalyzer`。
  - 可选 `LABEL` 元数据（maintainer / version / 华傲数据考核）。
  - `ENTRYPOINT ["/loganalyzer"]`（保持与本地二进制一致的 flag 行为；用户通过 `docker run --rm -v <log>:/data/app.log loganalyzer --file /data/app.log [其他 flag]` 传入日志与过滤参数）。

## 目标 .dockerignore 规格
排除一切非构建必需项，避免把 `.git`、JSONL 考核件、prompts、工具脚本、本地二进制、无关日志打进 build stage：
```
.git
.gitignore
*.md
AI开发考核*
*.jsonl
prompts/
tools/
internal/
/loganalyzer
cmd/loganalyzer/loganalyzer
*.test
*.out
*.exe
firebase-debug.log
test.log
/dist/
```
（保留：`go.mod`、`cmd/`、`parser/`、`stats/`、`output/`、`testdata/`——后者便于构建期 `go test` 与镜像内演示；如不想进镜像可一并排除 `testdata/`，二选一即可。）

## 实现约束（硬）
1. **不改 Go 源码/测试**：`cmd/`、`parser/`、`stats/`、`output/` 任意 `.go` 文件一律不动；Docker 只是打包，行为零变更是验收前提。
2. **运行阶段不得引入 libc/ca-certificates 依赖**：用 `scratch`；若坚持用 `alpine` 须说明理由（本工具无 TLS 需求，不推荐）。
3. **禁止使用 `latest` 浮动手标签于基础镜像之外**（build 阶段 `golang:1.22-alpine` 与运行阶段 `scratch` 均固定）。
4. 不新增第三方 Go 依赖（保持 `go.mod` 无 require）。

## 验收标准（双锁）
- **A. 镜像可构建且测试门通过**：`docker build -t loganalyzer:r12 .` 成功，`go test ./...` 在 build 阶段执行且无失败。
- **B. 行为零回归（最重要）**：从镜像内提取二进制与 round 9 基线（`4f6d8d1` 单 main.go 版重建）做逐项差分——
  - 方法一（推荐）：`docker run --rm -v "$PWD/testdata:/data" loganalyzer:r12 --file /data/sample.log` 对比本地 `./loganalyzer --file testdata/sample.log`（本地二进制由 `git show 4f6d8d1` 重建）输出逐字节一致。
  - 方法二：把镜像内 `/loganalyzer` 用 `docker create`+`docker cp` 取出，与 round 9 基线二进制逐一比对 `sample.log`/`utf8_edge.log`/`--top 2`/`--format json`/`--top 0` 等用例，全部一致。
  - 非法 `--from 2026-13-99` / `--format xml` 仍报退出码 1。
- **C. 镜像体积合理**：因 `scratch` + `-s -w`，镜像应远小于基于 `alpine` 的版本（通常 < 10MB 级，取决于 Go 版本）。
- **D. 仓库纯洁**：`git diff` 在 round 12 提交中仅含 `Dockerfile`（修改）+ `.dockerignore`（新增），无 Go 源码改动。

## 自测清单
- [ ] `docker build -t loganalyzer:r12 .` 成功，`go test ./...` 在 build 阶段执行通过
- [ ] `docker images loganalyzer:r12` 记录镜像大小（应显著小于 alpine 版）
- [ ] `docker run --rm -v "$PWD/testdata:/data" loganalyzer:r12 --file /data/sample.log` 输出与 round 9 基线逐项一致
- [ ] `docker run --rm -v "$PWD/testdata:/data" loganalyzer:r12 --file /data/utf8_edge.log --top 2 --format json` 与基线一致
- [ ] 非法 `--from` / `--format` 在容器内退出码 1
- [ ] `git status` round 12 提交仅含 `Dockerfile` + `.dockerignore`
- [ ] `gofmt -l .` / `go vet ./...` 不变（无 Go 改动）

## 本轮评分看点
- 多阶段构建 + `scratch` 运行阶段体现"最小镜像 / 零依赖"的工程意识；`-s -w` 与 `CGO_ENABLED=0` 展示对 Go 静态链接的掌握。
- build 阶段内嵌 `go test ./...` 质量门，体现"不可测的镜像不发布"的 CI 思维。
- `.dockerignore` 展示对构建上下文污染的防范意识。
- 严格遵守"不改源码、行为零回归"，证明容器化未引入任何隐性行为差异。
