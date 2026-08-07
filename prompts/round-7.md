# Round 7 提示词 — `--level` 级别过滤 + 时间区间过滤

> 目标：在现有 `loganalyzer` 上新增两类**可选**的条件过滤——按日志级别过滤（`--level`）、按时间区间过滤（`--from` / `--to`），
> 且**不得**改动既有的行统计 / 级别分类逻辑、不得引入任何回归。
>
> 背景：前 6 轮已落定「流式读取 → 行级统计 → UTF-8 正确分级 → 关键字过滤」。本轮是**第二个新功能增量轮**，
> 重点是「在一个已经能跑的流式循环里叠加多个可选的 AND 过滤条件，且默认路径零差异」。这也是评审最看重的能力点。

---

【提示词正文】

请在 `loganalyzer` 上新增以下**可选**命令行参数，实现条件过滤：

- `--level <LEVEL>`：仅统计级别为 `<LEVEL>` 的行（级别比较**大小写不敏感**，即 `error` / `Error` / `ERROR` 等价）。
- `--from <TIME>`：仅统计时间戳 **≥ `<TIME>`** 的行。
- `--to <TIME>`：仅统计时间戳 **≤ `<TIME>`** 的行。
- `--from` 与 `--to` 可单独或同时出现，二者构成闭区间 `[from, to]`（含端点）。

## 行为语义（必须严格遵守）

1. 三个新参数全部未提供时：**不启用任何过滤**，程序行为与 round 6（commit `39898e3`）**逐字节完全一致**——
   包括输出内容的每一行、每一个空格。这一条是硬约束，用下面的差分复测验证。
2. `--contains` 仍保持 round 6 语义；当 `--contains` / `--level` / `--from` / `--to` 中**任意一个**生效时，
   多个条件之间为 **AND**（一行须同时满足所有生效条件才计入统计）。被任一条件过滤掉的行**不计入** `Total` / `Empty` / 任何级别计数。
3. `--level` 语义：用 `classifyLevel(line)` 得到该行级别，与 `<LEVEL>` 做 `strings.EqualFold` 比较；不匹配则该行不计入。
   若 `<LEVEL>` 不属于已知级别集合（`ERROR/WARN/INFO/DEBUG/UNKNOWN`，大小写不敏感），则没有任何行能匹配 → 全 0、退出码 0（不报错）。
4. 时间解析：取每行**第一个空白分隔的 token**（用与 `classifyLevel` 相同的 rune 感知扫描提取，**不要用 `strings.Fields`**），
   用以下布局依次尝试解析（命中第一个即可）：
   - `"2006-01-02 15:04:05"`
   - `"2006-01-02T15:04:05"`
   - `"2006-01-02"`
   - `"2006/01/02 15:04:05"`
   - `"2006/01/02"`
   行首 token 无法解析为时间时，该行在时间过滤生效时**被排除**（视为不在区间内）。
   `--from` / `--to` 的值用相同布局解析；若提供的值**无法解析**，向 stderr 打印错误并以**退出码 1** 退出（这是用户输入错误，应显式报错而非静默忽略）。
5. 空行不可能匹配非空 `--contains`、也不可能匹配 `--level`（空行级别是 `UNKNOWN`，除非 `--level UNKNOWN`），自然被相应条件过滤掉，这是预期行为。

## 输出格式

- 默认无过滤：输出格式与 round 6 一致，**不要新增 / 删减任何输出行**。
- 任意过滤生效时：在 `Total lines:` 这一行**上方**新增且仅新增一行 `Filter: ...`，内容为各生效条件的拼接（空格分隔），顺序固定为
  `contains="..." level="..." from="..." to="..."`（未生效的条件不出现）。
  - 仅 `--contains ERROR` 时，必须输出与 round 6 **完全相同**的一行：`Filter: contains="ERROR"`（**不得改变既有格式**）。
  - 例：同时 `--contains err --level ERROR --from 2026-08-01` → `Filter: contains="err" level="ERROR" from="2026-08-01"`。
  - 例：仅 `--level WARN` → `Filter: level="WARN"`。
  - 例：仅 `--from 2026-08-01 --to 2026-08-31` → `Filter: from="2026-08-01" to="2026-08-31"`。
  其余输出行格式与默认路径相同。

## 实现约束

1. 过滤逻辑必须插入在**流式读取循环内部**，与 `--contains` 共用同一套 `if/else` 包裹结构（**不要**用裸 `continue` 跳过循环末尾的 `readErr` 检查，
   否则最后一行无换行会被漏统计）。推荐做法：在循环内先算出本行级别 `lv := classifyLevel(line)`，再用一个 `keep := true` 标志依次叠加各条件判断，
   最后 `if keep { stats.Total++ ... stats.Levels[lv]++ }`。
2. `analyze` 增加形参：`level string`、`from string`、`to string`（签名变为 `analyze(path, contains, level, from, to string)`，
   或等价地把过滤条件打包成结构体——任选其一，但**不要破坏流式读取、不要先读进切片再过滤**）。`--from` / `--to` 解析得到的 `time.Time` 应在 `analyze` 开头各解析一次，
   **不要**在每行循环里重复解析范围端点。
3. 改动范围控制在：`analyze`（加形参 + 过滤判断 + 时间解析辅助函数）、`main`（新增 `--level` / `--from` / `--to` 三个 flag 并传入）、
   以及必要的 `import`（需新增 `"time"`）。**不要**改动 `classifyLevel` 的 UTF-8 解码逻辑、不要退回 `strings.Fields`、不要改动 `levelOrder` / 输出对齐逻辑 / 测试语料。
4. 仅使用 Go 标准库。

## 验收标准（差分测试 —— 本轮唯一的正确性判据）

- **零回归判据**：不带任何新 flag、也不带 `--contains` 时，在以下语料上的输出必须与 round 6 已知正确值**逐项一致**（round 6 已验证）：
  - `testdata/sample.log` → `Total lines: 17` / `Empty lines: 1` / `ERROR: 4` `WARN: 2` `INFO: 5` `DEBUG: 2` `UNKNOWN: 4`
  - `testdata/utf8_edge.log` → `ERROR: 4` `WARN: 1` `INFO: 2` `DEBUG: 1` `UNKNOWN: 0`
  - 空文件 → 全 0、退出码 0；末行无换行文件 → `Total` 等于实际行数；长行（>64KB）→ 不报 `token too long`
  - 仅带 `--contains ERROR`（不带新 flag）：输出必须与 round 6 该用例**逐字节一致**（首行仍是 `Filter: contains="ERROR"`）。
- **级别过滤判据**（`--level`，在 `testdata/utf8_edge.log` 上）：
  - `--level ERROR` → `Filter: level="ERROR"`，`Total lines: 4` / `ERROR: 4` / 其余级别 0。
  - `--level INFO` → `Total lines: 2` / `INFO: 2` / 其余 0。
  - `--level FOO`（非法级别）→ 全 0、退出码 0。
  - `--contains ERROR --level ERROR` → 与 `--level ERROR` 一致（`Total lines: 4` / `ERROR: 4`）。
- **时间区间过滤判据**（构造临时语料 `/tmp/time.log`）：
  ```
  2026-08-01 srv1 ERROR 启动
  2026-08-15 srv2 WARN 警告
  2026-09-01 srv3 INFO 正常
  ```
  - `--from 2026-08-01 --to 2026-08-31` → `Total lines: 2` / `ERROR: 1` `WARN: 1`（第 1、2 行）。
  - `--from 2026-09-01` → `Total lines: 1` / `INFO: 1`（仅第 3 行）。
  - `--to 2026-08-10` → `Total lines: 1` / `ERROR: 1`（仅第 1 行）。
  - `--from 2026-08-01 --to 2026-08-31 --level WARN` → `Total lines: 1` / `WARN: 1`（第 2 行）。
  - `--from 2026-13-99`（非法时间）→ stderr 报错、退出码 1。

---

## 自测清单（我会逐条复跑并与 round 6 做差分比对，请勿代填结果）

```bash
go vet ./... && gofmt -l . && go build -o loganalyzer ./cmd/loganalyzer

# —— 零回归：不带新 flag，输出必须与 round 6 逐项一致 ——
./loganalyzer --file testdata/sample.log        # Total 17 / Empty 1 / E4 W2 I5 D2 U4
./loganalyzer --file testdata/utf8_edge.log      # E4 W1 I2 D1 U0
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log   # 全 0，退出码 0
printf 'a\nb\nc' > /tmp/noeol.log && ./loganalyzer --file /tmp/noeol.log  # Total 3
python3 -c "open('/tmp/longline.log','w').write('A'*100000+'\n'+'short\n'+'\n')"
./loganalyzer --file /tmp/longline.log           # Total 3 / Empty 1，不报 token too long
./loganalyzer --file testdata/utf8_edge.log --contains ERROR   # 与 round6 逐字节一致：Filter: contains="ERROR" / Total 4 / ERROR 4

# —— 级别过滤 ——
./loganalyzer --file testdata/utf8_edge.log --level ERROR   # Filter: level="ERROR" / Total 4 / ERROR 4
./loganalyzer --file testdata/utf8_edge.log --level INFO    # Filter: level="INFO" / Total 2 / INFO 2
./loganalyzer --file testdata/utf8_edge.log --level FOO     # 全 0，退出码 0
./loganalyzer --file testdata/utf8_edge.log --contains ERROR --level ERROR  # Total 4 / ERROR 4

# —— 时间区间过滤（构造语料） ——
printf '2026-08-01 srv1 ERROR 启动\n2026-08-15 srv2 WARN 警告\n2026-09-01 srv3 INFO 正常\n' > /tmp/time.log
./loganalyzer --file /tmp/time.log --from 2026-08-01 --to 2026-08-31   # Total 2 / E1 W1
./loganalyzer --file /tmp/time.log --from 2026-09-01                   # Total 1 / I1
./loganalyzer --file /tmp/time.log --to 2026-08-10                     # Total 1 / E1
./loganalyzer --file /tmp/time.log --from 2026-08-01 --to 2026-08-31 --level WARN  # Total 1 / W1
./loganalyzer --file /tmp/time.log --from 2026-13-99                   # 退出码 1（非法时间）
```

请只输出改动后的 `analyze` 函数、`main` 函数，以及 `import` 的变更说明。
不要输出完整文件，也不要在对话里贴 diff——我会用 `git diff` 自行审阅。

---

## 本轮评分看点

- **多条件 AND 过滤的结构化实现**：在已验证的流式循环里叠加 `--level` / `--from` / `--to` 三个可选条件，
  用统一的 `keep` 标志位而非散落的 `continue`，既保证逻辑清晰，又守住「末行无换行不漏统计」这条前几轮反复加固的底线。
- **默认路径零回归意识**：加两个新功能轮（round 6/7）最容易顺手改坏旧输出。用「默认路径字节级一致 + 差分复测」把回归风险锁死，
  这正是前几轮抓 bug（round 2 超长行、round 4 内存退化、round 5 UTF-8 回归）沉淀出的工程纪律。
- **时间解析的鲁棒性与边界**：用多布局 `tryParse` 兼容常见日志时间戳，区间端点只在循环外解析一次，
  非法时间显式报错退出（exit 1）而非静默吞掉——体现对 CLI 输入契约的把控。
- **输出可解释性**：所有生效的过滤条件被合并打印到一行 `Filter: ...`，让结果可被评审 / 用户直接读懂，而非默默改变数字。
