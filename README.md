# AI 开发考核 · loganalyzer（Go 日志分析 CLI）

> 本仓库为「华傲数据 2026 AI 开发考核」提交物。
> 开发语言：Go ｜ 编程智能体：Kilo Code ｜ 题目：命令行日志分析工具

---

## 一、选题说明

- **题目**：`loganalyzer` —— 一个轻量命令行日志分析工具。读取本地日志文件，统计各级别（INFO/WARN/ERROR）出现次数，支持按时间区间 / 关键字过滤、Top N 高频错误、文本表格与 JSON 两种输出格式。
- **选题理由**：
  1. 需求边界清晰、可完整落地，符合「轻量、可交付」要求；
  2. 天然可拆成多轮迭代（读文件 → 解析级别 → 过滤 → 格式化 → 性能 → 异常 → 测试 → 重构 → 容器化 → 文档），每一轮都能体现「指令→代码变更→提交」的闭环；
  3. 可用 Docker 容器化，呼应岗位对容器 / Linux 能力的要求，形成差异化。
- **环境准备说明**：仓库初始化、`go.mod`、`.gitignore`、本 README 模板、`tools/gen_jsonl.py`、目录骨架由候选人在使用智能体前完成（属于工程脚手架，**不计入智能体交互轮次**）。JSONL 的 `round_id` 从第一次 Kilo Code 功能交互（第 1 轮）开始编号。

---

## 二、开发轮次规划（与 JSONL / Git 提交一一对应）

| 轮次 | 目标 | 关键产出 |
|---|---|---|
| 1 | 初始化 CLI，逐行读取日志文件并输出总行数 | 文件读取骨架 |
| 2 | 解析日志级别并分级统计 | 级别统计 |
| 3 | 支持按时间区间过滤 | 时间解析与过滤 |
| 4 | 支持 `--contains` 关键字过滤 | 关键字过滤 |
| 5 | 支持 `--format text/json` 两种输出 | 输出格式化 |
| 6 | 支持 `--top N` 高频错误展示 | Top N |
| 7 | 大文件流式读取 + 进度提示 | 性能优化 |
| 8 | 文件不存在 / 无权限的友好报错 | 异常处理 |
| 9 | 单元测试覆盖解析与统计 | 测试 |
| 10 | 重构：拆分 parser / stats / output 子包 | 结构优化 |
| 11 | 编写 Dockerfile 并构建镜像 | 容器化 |
| 12 | 完善 README、示例与最终自测 | 交付收尾 |

> 实际轮次以 JSONL 记录为准；上表为规划，可在开发中按需增删。

---

## 三、提示词（Prompt）产生的方法

> 本節说明我如何把业务需求转化为给 Kilo Code 的指令，供评审参考。

方法论（由候选人在开发中持续补充，建议每轮记录「为什么这样写指令」）：

1. **先拆后问**：把一个大需求拆成最小可验证单元，每轮只给一个明确目标，避免一次性大指令导致难以 review。
2. **给上下文不替写**：指令里说明「现有结构与意图」，但让智能体产出实现，我再审核 diff。
3. **可验证性**：每条指令都附带「如何验证这轮成功」（如运行命令 / 预期输出），方便自测与写测试。
4. **逐步加约束**：先跑通主路径，再叠加错误处理、性能、格式等约束，符合「可完整落地」原则。

（每轮实际使用的 `prompt_content` 见 `AI开发考核_姓名_loganalyzer.jsonl`。）

---

## 四、JSONL 文件的生成方法

1. 每一轮用 Kilo Code 完成改动后，**立即** `git add -A && git commit -m "round N: <本轮目标>"`。
2. 在 `rounds.json`（可复制 `rounds.template.json`）中追加该轮记录：`round_id` / `prompt_content`（你真实发出的指令）/ `commit_hash` / `modify_time` / `agent_type` / `dev_language`。
3. 运行生成器自动补全 `modify_diff`（取自真实 git 历史）：
   ```bash
   python3 tools/gen_jsonl.py \
     --repo . \
     --rounds rounds.json \
     --out "AI开发考核_姓名_loganalyzer.jsonl"
   ```
4. 输出文件编码 UTF-8、每行一条 JSON、无空行、无注释，符合考核规范第三部分。

> 设计要点：`modify_diff` 由脚本从 git 抓取，**保证 diff 与提交一一对应、不可篡改**；候选人需确保 `prompt_content` 为真实指令、`commit_hash` 与当轮提交一致。

---

## 五、过程中遇到的问题和解决方法

> 由候选人在开发中按轮次如实填写（这是评分重点之一）。

- 问题 1（待补充）：……
  - 现象：……
  - 排查：……
  - 解决（含 Kilo Code 输出如何被审核/纠错）：……
- 问题 2（待补充）：……

---

## 六、运行 / 构建 / 容器化

```bash
# 构建
go build -o loganalyzer ./cmd/loganalyzer

# 基础用法
./loganalyzer --file app.log
./loganalyzer --file app.log --level ERROR --contains "timeout" --top 5 --format json

# 测试
go test ./...

# 容器化（轮次 11）
docker build -t loganalyzer .
docker run --rm -v "$PWD":/data loganalyzer --file /data/app.log
```

---

## 七、Git 提交规范

- 每个智能体轮次 = 一个独立 commit，message 形如 `round N: <目标>`。
- 仓库全程保留完整提交历史，不删减、不 amend 已对应 JSONL 的提交。
- 提交历史与 `AI开发考核_姓名_loganalyzer.jsonl` 逐轮对应，可相互追溯。
