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

| 轮次 | 目标 | 关键产出 | 状态 | commit |
|---|---|---|---|---|
| 1 | 初始化 CLI，`--file` 参数解析 + 流式统计总行数 / 空行数 | 文件读取骨架 | ✅ 完成 | `cf12b94` |
| 2 | **修复超长行崩溃**：`bufio.Scanner` 64KB 上限导致 `token too long` | 改用 `bufio.Reader` 逐行读取 | ✅ 完成 | `267b759` |
| 3 | 解析日志级别并分级统计（ERROR/WARN/INFO/DEBUG/UNKNOWN） | 级别统计 | 🔄 进行中 | — |
| 4 | 支持 `--contains` 关键字过滤 | 关键字过滤 | 待开始 | — |
| 5 | 支持按时间区间过滤 | 时间解析与过滤 | 待开始 | — |
| 6 | 支持 `--format text/json` 两种输出 | 输出格式化 | 待开始 | — |
| 7 | 支持 `--top N` 高频错误展示 | Top N | 待开始 | — |
| 8 | 文件不存在 / 无权限 / 传入目录的友好报错 | 异常处理 | 待开始 | — |
| 9 | 单元测试覆盖解析与统计 | 测试 | 待开始 | — |
| 10 | 重构：拆分 parser / stats / output 子包 | 结构优化 | 待开始 | — |
| 11 | 编写 Dockerfile 并构建镜像 | 容器化 | 待开始 | — |
| 12 | 完善 README、示例与最终自测 | 交付收尾 | 待开始 | — |

> **关于轮次与初始规划的偏差（如实说明）**：原计划第 2 轮做「级别统计」，但在第 1 轮代码审核中
> 实测发现了 `bufio.Scanner` 的超长行崩溃缺陷（详见第五节问题 1），因此第 2 轮改为**修复该缺陷**，
> 功能开发整体顺延一轮。这是「计划服从于真实代码质量」的取舍，而非机械照搬预设清单。
> 实际轮次一律以 JSONL 记录与 Git 提交为准。
>
> 另：仓库中 `chore:` 前缀的提交为脚手架 / 提示词归档 / JSONL 归档等**环境准备类操作**，
> 不属于智能体交互轮次，不计入 JSONL；只有 `round N:` 前缀的提交与 JSONL 记录一一对应。

---

## 三、提示词（Prompt）产生的方法

> 本节说明我如何把业务需求转化为给 Kilo Code 的指令，供评审参考。

方法论（由候选人在开发中持续补充，建议每轮记录「为什么这样写指令」）：

1. **先拆后问**：把一个大需求拆成最小可验证单元，每轮只给一个明确目标，避免一次性大指令导致难以 review。
2. **给上下文不替写**：指令里说明「现有结构与意图」，但让智能体产出实现，我再审核 diff。
3. **可验证性**：每条指令都附带「如何验证这轮成功」（如运行命令 / 预期输出），方便自测与写测试。
4. **逐步加约束**：先跑通主路径，再叠加错误处理、性能、格式等约束，符合「可完整落地」原则。

（每轮实际使用的 `prompt_content` 见 `AI开发考核_刘贵达_loganalyzer.jsonl`。）

---

## 四、JSONL 文件的生成方法

1. 每一轮用 Kilo Code 完成改动后，**立即** `git add -A && git commit -m "round N: <本轮目标>"`。
2. 在 `rounds.json`（可复制 `rounds.template.json`）中追加该轮记录：`round_id` / `prompt_content`（你真实发出的指令）/ `commit_hash` / `modify_time` / `agent_type` / `dev_language`。
3. 运行生成器自动补全 `modify_diff`（取自真实 git 历史）：
   ```bash
   python3 tools/gen_jsonl.py \
     --repo . \
     --rounds rounds.json \
     --out "AI开发考核_刘贵达_loganalyzer.jsonl"
   ```
4. 输出文件编码 UTF-8、每行一条 JSON、无空行、无注释，符合考核规范第三部分。

> 设计要点：`modify_diff` 由脚本从 git 抓取，**保证 diff 与提交一一对应、不可篡改**；候选人需确保 `prompt_content` 为真实指令、`commit_hash` 与当轮提交一致。

---

## 五、过程中遇到的问题和解决方法

> 按轮次如实记录，包含 Kilo Code 生成代码的缺陷、我如何发现与定位、以及如何转化为下一轮修复指令。

### 问题 1：`bufio.Scanner` 单行 64KB 上限导致超长行崩溃（round 1 → round 2）

- **发现方式**：round 1 的代码通过了 `go vet`、`gofmt`、正常文件、缺参数、文件不存在等全部常规用例。
  但我没有止步于「跑通即通过」，而是**主动构造边界用例**——生成一个含 10 万字符单行的日志文件：
  ```bash
  python3 -c "open('/tmp/longline.log','w').write('A'*100000 + '\n' + 'short\n' + '\n')"
  ./loganalyzer --file /tmp/longline.log
  ```
- **现象**：程序报错退出，`错误：读取文件失败：bufio.Scanner: token too long`（退出码 1）。
  期望应为 `Total lines: 3 / Empty lines: 1`。
- **定位**：`bufio.Scanner` 的单个 token 默认上限是 `bufio.MaxScanTokenSize`（64KB）。
  这**不是钻牛角尖的边界**——生产日志中的长 JSON 请求体、Java 异常堆栈单行超过 64KB 极为常见，
  属于会在真实环境直接导致统计失败的**生产级缺陷**。
- **转化为修复指令**：我在 round 2 的提示词中提供了三要素——① 可复现的最小命令与实际/期望输出对比；
  ② 已定位的根因（`bufio.MaxScanTokenSize`）；③ 两个候选方案及取舍依据：
  - **方案 A**：`scanner.Buffer()` 扩容上限——缺点是仍需硬编码一个可能仍不够的上限值；
  - **方案 B**：改用 `bufio.Reader` + `ReadString('\n')`——天然无单行长度上限。

  同时给出边界约束：必须保持流式（内存不随文件总大小增长）、末行无换行符仍需计数、空文件输出 0/0 且退出码 0。
- **结果**：Kilo Code 选择了**方案 B**，并正确处理了 `io.EOF` 时最后一段无换行内容的计数。
  我复测了 10 项用例（含原 bug 用例、空文件、末行无换行、仅一个换行、CRLF 行尾、目录、无权限文件）全部通过；
  另做 10MB 单行压测：耗时 0.03s、峰值常驻内存 24MB，证明内存只随**单行长度**增长而非文件总大小，流式特性未被破坏。
- **复盘**：这一轮验证了「AI 生成的代码能跑通 ≠ 代码可用」。评审真正该看的是**审核者是否具备构造边界用例、
  定位根因、并把根因转化为精准修复指令的能力**——而不是 AI 一次写对的运气。

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
- 提交历史与 `AI开发考核_刘贵达_loganalyzer.jsonl` 逐轮对应，可相互追溯。
