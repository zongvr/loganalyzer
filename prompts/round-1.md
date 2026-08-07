# Round 1 提示词：CLI 骨架 + 文件读取 + 基础行数统计

> 本目录存放发给 Kilo Code 的每一轮提示词定稿（便于复盘与团队协作）。
> 第 1 轮的目标：让智能体先把"读日志文件 + 统计总行数/空行数"这条主干路打通，
> 不追求功能完整，留出后续 11 轮清晰迭代空间。

---

## 直接发给 Kilo Code 的指令（复制下方"【提示词正文】"整段）

【提示词正文】

任务：在当前 Go 项目 `loganalyzer` 中实现命令行日志分析 CLI 的**最小骨架**（本轮只完成这一个目标，不要扩散）。

具体要求：

1. 入口在 `cmd/loganalyzer/main.go`。
2. 使用 `flag` 标准库解析命令行参数：必填 `--file <path>`（日志文件路径），可选 `--help`；未知/缺失参数给出友好中文错误信息并以退出码 1 退出。
3. 使用 `bufio.NewReader`（或 `bufio.Scanner`）以**流式**方式逐行读取 `--file` 指定的文件，**禁止一次性 `os.ReadFile` 大文件读入内存**。
4. 全部读取完毕后，向 stdout 输出两行（注意冒号后是一个空格）：
   - `Total lines: <N>`
   - `Empty lines: <M>`
   其中空行 = 去除首尾空白（用 `strings.TrimSpace`）后为空的行。
5. **仅使用 Go 标准库**，不得引入任何第三方依赖（不要 `go get`）。
6. 健壮性要求：
   - 未传 `--file` → 中文友好错误 + 退出码 1；
   - 文件不存在 / 无读取权限 → 中文友好错误 + 退出码 1；
   - 正常完成 → 退出码 0。
7. 代码结构：将文件读取 + 行计数拆为独立函数
   ```go
   func countLines(path string) (total, empty int, err error)
   ```
   `main()` 只做参数解析与调度，便于后续单元测试。
8. 完成后**不要**在本对话里输出完整 diff（会浪费上下文，我会自己 `git diff` 审核）。只需简短说明：
   - 修改了哪些文件；
   - 关键改动点（不超过 5 行文字）；
   - 一条手工验证命令（形如 `./loganalyzer --file test.log`）。

我接下来会做的事（你不用管，照常等待即可）：
- 跑 `go build ./cmd/loganalyzer` 验证可编译；
- 跑 `./loganalyzer --file <某个测试日志>` 验证输出；
- 自己 `git diff` 审你的改动；
- `git commit -m "round 1: 实现 --file 参数与基础行数统计"`；
- 在仓库根 `rounds.json` 追加 round 1 记录，再跑 `python3 tools/gen_jsonl.py` 生成 JSONL。

---

## 自测建议（候选人在提交前自己跑一遍）

```bash
# 1) 准备测试日志
head -n 5 /var/log/system.log > test.log 2>/dev/null || printf "INFO start\n\nWARN foo\nERROR bar\nINFO end\n" > test.log

# 2) 构建
go build -o loganalyzer ./cmd/loganalyzer

# 3) 正常路径：期望输出 Total lines / Empty lines 两条
./loganalyzer --file test.log

# 4) 异常路径：期望退出码 1 + 中文错误
./loganalyzer                    # 缺参数
./loganalyzer --file no_such.log  # 文件不存在

# 5) vet + 格式化
go vet ./...
gofmt -l .
```

---

## 评分看点（这一轮要体现的能力）

- **指令接收与转化**：把模糊需求拆成具体可执行项（8 条）。
- **代码风格与结构**：拆 `countLines` 函数、错误处理、`bufio` 选择。
- **健壮性**：参数校验、错误退出码、不读大文件到内存。
- **协议遵守**：不吐完整 diff、留可验证命令。