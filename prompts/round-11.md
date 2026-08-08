# Round 11 提示词 — 重构：拆分 parser / stats / output 子包

> 目标：在不改变**任何外部行为与输出格式**的前提下，把 `cmd/loganalyzer/main.go`（round 10 时是「一个文件扛所有逻辑 + 421 行 `main_test.go`」）重构成职责清晰的子包结构——解析归 `parser`、统计归 `stats`、渲染归 `output`，`main` 退化为纯 CLI 装配层。
> 本轮是**结构重构轮**：核心纪律是「**行为零变化**」——重构后 `./loganalyzer` 的 text/json 输出必须与 round 9（`4f6d8d1`，与 round 10 行为一致）**逐字节完全一致**；既有单元测试必须整体迁到子包后**全部仍绿**。这是重构是否成功唯一判据，用下方「零回归判据」差分复测锁死。
>
> 背景：前 10 轮已把功能（流式统计 / UTF-8 分级 / 关键字·级别·时间过滤 / text·json 双格式 / Top-N 高频榜）与测试（round 10，74% 覆盖）做扎实，但所有逻辑挤在 `main` 包里、函数未导出、测试只能 `package main` 内测。本轮把代码按职责拆包，既提升可维护性，也为「可独立单测 / 可复用」铺路——这是评审看重的工程成熟度信号。**拆包≠改行为**，所有既有逻辑（含空行也计入 `Levels["UNKNOWN"]`、各级别对齐空格、`top` 字段有无规则）原样平移。

---

【提示词正文】

请将 `loganalyzer` 的 `cmd/loganalyzer/main.go` 按以下包结构重构。**只移动/导出代码，不准改变任何逻辑、字符串、对齐空格、json 字段顺序与 0 值处理。**

## 目标包结构（模块根 `loganalyzer`）

```
loganalyzer
├── parser/            # 解析与聚类纯函数（无副作用、无 IO）
│   ├── parser.go
│   └── parser_test.go
├── stats/            # 流式统计 + Top-N 聚合 + Stats 类型
│   ├── stats.go
│   └── stats_test.go
├── output/           # 渲染（text / json）
│   ├── output.go
│   └── output_test.go
├── cmd/loganalyzer/
│   └── main.go        # 仅 flag 解析 + 装配，逻辑全在子包
├── go.mod
└── ...（testdata / prompts / README / JSONL 不动）
```

## 各包职责与导出 API（照搬现有实现，仅首字母大写导出）

### `parser` 包（`parser/parser.go`，`package parser`）

导出以下函数（原 `main.go` 中同名未导出实现**原样平移**，rune 感知扫描逻辑一字不改）：

- `func ClassifyLevel(line string) string` ← 原 `classifyLevel`
- `func FirstToken(line string) string` ← 原 `firstToken`
- `func ParseTime(s string) (time.Time, bool)` ← 原 `parseTime`
- `func LineTime(line string) (time.Time, bool)` ← 原 `lineTime`
- `func MessageKey(line string) string` ← 原 `messageKey`
- `var TimeLayouts = []string{...}` ← 原 `timeLayouts`（5 个布局，原样）

内部扫描辅助逻辑（DecodeRune + unicode.IsSpace）随函数一起搬入，不要重写。

### `stats` 包（`stats/stats.go`，`package stats`）

- `type Stats struct { Total int; Empty int; Levels map[string]int }` ← 原 `Stats`
- `type MsgCount struct { Message string; Count int }` ← 原 `msgCount`
- `var LevelOrder = []string{"ERROR", "WARN", "INFO", "DEBUG", "UNKNOWN"}` ← 原 `levelOrder`
- `func Analyze(path, contains, level string, from, to *time.Time, topN int) (Stats, []MsgCount, error)` ← 原 `analyze`

`stats` 需要 `parser`：`Analyze` 内部用 `parser.ClassifyLevel` / `parser.LineTime` / `parser.MessageKey` 替换原未导出调用（逻辑不变）。**关键不变式**（务必保留，否则输出/计数会变）：
- `Levels` 初始化为 `make(map[string]int, len(LevelOrder))`；逐行 `stats.Levels[lv]++` —— **空行也会 `classifyLevel("")`→`UNKNOWN` 并令 `Levels["UNKNOWN"]++`**（原行为）。
- `Total` 对所有 `keep` 行（含空行）`++`；空行 `Empty++`。
- `topN >= 1` 才分配计数 map、`sort.SliceStable` 频率降序 + 同频首次出现序、截断前 N；`topN <= 0` 返回 `nil`。

### `output` 包（`output/output.go`，`package output`）

- `func Render(s stats.Stats, format, filter string, top []stats.MsgCount, topN int) error` ← 原 `output`

`output` 需要 `stats`：用 `stats.LevelOrder` 决定级别宽度与打印顺序；`top` 元素类型为 `stats.MsgCount`。json 分支的 `levelCount` / `topMsg` / `report` 结构体、`*[]topMsg json:"top,omitempty"`、`topN>=1` 时 `topPtr=&topList` 否则 `nil`（字段省略）等逻辑**原样**搬入。`filter` 为空串时不打印 `Filter:` 行、json `filter` 为空串——全部与原 `output` 一致。

### `cmd/loganalyzer/main.go`（仅装配）

- `import`：`loganalyzer/parser`、`loganalyzer/stats`、`loganalyzer/output`（外加 `bufio`/`flag`/`fmt`/`os`/`strings`/`time`/`unicode`/`unicode/utf8` 等按需，原 `main` 用到的）。
- 逻辑：`flag` 解析 → 用 `parser.ParseTime` 解析 `--from`/`--to`（非法→stderr+exit1，原错误文案不变）→ `stats.Analyze(*file, *contains, *level, from, to, *topN)` → 按原规则拼接 `filter` 字符串（contains/level/from/to 顺序、 `%q` 引号，原样）→ `output.Render(stats, formatMode, filter, top, *topN)`。
- **不得**改变任何 flag 名称、错误信息文案、退出码。

## 测试迁移（必须整体迁移，不得丢弃任何用例）

删除 `cmd/loganalyzer/main_test.go`，按以下映射把 21 个测试函数**逐一搬入对应子包测试文件**，并改用导出名调用（`captureStdout` 助手搬进 `output/output_test.go`，`writeTempLog` 助手搬进 `stats/stats_test.go`）：

| 原测试函数 | 目标文件 | 调用改为 |
|---|---|---|
| `TestClassifyLevel`、`TestClassifyLevelOnlyFirstThreeFields`、`TestMessageKey`、`TestParseTime`、`TestLineTime`、`TestFirstToken` | `parser/parser_test.go` | `parser.ClassifyLevel` / `parser.MessageKey` / `parser.ParseTime` / `parser.LineTime` / `parser.FirstToken` |
| `TestAnalyzeCounts`、`TestAnalyzeLastLineNoNewline`、`TestAnalyzeContainsFilter`、`TestAnalyzeLevelFilter`、`TestAnalyzeTimeFilter`、`TestAnalyzeTopN`、`TestAnalyzeTopNTruncate`、`TestAnalyzeTopNDisabled` | `stats/stats_test.go` | `stats.Analyze` 构建 `stats.Stats` / `[]stats.MsgCount`；其中 `TestAnalyzeTimeFilter` 用 `parser.ParseTime("2026-08-01")` 构造 `from`/`to` 指针 |
| `TestOutputText`、`TestOutputTextFilterLine`、`TestOutputTextTop`、`TestOutputTextTopDisabledNoBlock`、`TestOutputJSON`、`TestOutputJSONTop`、`TestOutputJSONTopEmptyIsArrayNotNil` | `output/output_test.go` | `output.Render(stats.Stats{...}, ...)` / `[]stats.MsgCount{{Message:..,Count:..}}`；沿用 `captureStdout` |

所有断言（期望字符串、级别映射、顺序、对齐空格、`top` 字段有无、`top` 空时为 `[]`）**保持原值不变**。迁移后 `go test ./...` 必须全绿，且覆盖的函数与 round 10 完全一致。

## 实现约束

1. **零行为变更**：除「未导出→导出（首字母大写）+ 跨包调用」外，任何函数体不得改动；对齐空格、json 字段名/顺序/0 值、`top` 字段有无规则、空行计入 `UNKNOWN`、末行无换行统计、长行不崩，全部与原实现逐字节等价。
2. **无 import 环**：依赖方向 `output → stats → parser`，`main` 依赖三者；`parser` 不依赖任何其他包。不要引入三方依赖。
3. 包名用 `parser`/`stats`/`output`（模块根下，非 `internal/`，避免导入限制）；`main` 用完整导入路径 `loganalyzer/parser` 等。
4. 不要改动 `testdata/`、`./prompts/`、`README.md`、`*.jsonl`、`rounds.json` 等；不要新增任何功能或 flag。

## 验收标准（差分测试 —— 零回归 + 可编译 + 测试绿）

### A. 零回归判据（重构后二进制必须与 round 9 `4f6d8d1` 逐项字节一致）

用 `go build -o loganalyzer ./cmd/loganalyzer` 构建，对比：

- `testdata/sample.log` → `Total lines: 17` / `Empty lines: 1` / `ERROR: 4` `WARN: 2` `INFO: 5` `DEBUG: 2` `UNKNOWN: 4`。
- `testdata/utf8_edge.log` → `E4 W1 I2 D1 U0`。
- 空文件 → 全 0、退出码 0；末行无换行文件 → `Total` 等于实际行数；长行（>64KB）→ 不报 `token too long`。
- `testdata/utf8_edge.log --contains ERROR` → `Filter: contains="ERROR"` / `Total lines: 4` / `ERROR: 4`。
- `testdata/sample.log --format json`（不启用 `--top`）→ 与 round 8/9 json **逐字节一致，`top` 字段不得出现**。
- `diff <(./loganalyzer --file testdata/sample.log) <(./loganalyzer --file testdata/sample.log --top 0) && echo IDENTICAL` 应通过。
- `/tmp/top.log`（round 9 用例）`--top 2` text 与 json 与 round 9 完全一致。

### B. 可编译 + 测试绿

- `go build ./...` 全包编译通过。
- `go test ./...` 全绿（21 个用例搬到 3 个子包后全部通过）。
- `go vet ./...` 干净；`gofmt -l .` 无输出。
- `go test -cover ./...` 覆盖率应不低于 round 10 的 74.0%（迁移不应掉覆盖）。

---

## 自测清单（我会逐条复跑并与 round 9 做差分比对，请勿代填结果）

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
# 上述必须全绿 / 干净

go build -o loganalyzer ./cmd/loganalyzer

# —— 零回归：重构后行为与 round 9 (4f6d8d1) 逐项一致 ——
./loganalyzer --file testdata/sample.log                  # Total 17 / Empty 1 / E4 W2 I5 D2 U4
./loganalyzer --file testdata/utf8_edge.log                # E4 W1 I2 D1 U0
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log  # 全 0，退出码 0
printf 'a\nb\nc' > /tmp/noeol.log && ./loganalyzer --file /tmp/noeol.log  # Total 3
python3 -c "open('/tmp/longline.log','w').write('A'*100000+'\n'+'short\n'+'\n')"
./loganalyzer --file /tmp/longline.log                     # Total 3 / Empty 1，不报 token too long
diff <(./loganalyzer --file testdata/sample.log) <(./loganalyzer --file testdata/sample.log --top 0) && echo "top0==default IDENTICAL"

# —— 与 round 9 json 零回归（无 top 字段）——
./loganalyzer --file testdata/sample.log --format json | python3 -c "import sys,json; d=json.load(sys.stdin); print('top' in d)"   # 应输出 False

# —— Top-N 一致性 ——
cat > /tmp/top.log <<'EOF'
2026-08-07 09:00:01 ERROR db connection failed
2026-08-07 09:00:02 ERROR db connection failed
2026-08-07 09:00:03 WARN  cache miss
2026-08-07 09:00:04 ERROR db connection failed
2026-08-07 09:00:05 WARN  cache miss
2026-08-07 09:00:06 INFO  ok
EOF
diff <(./loganalyzer --file /tmp/top.log --top 2) <(git show 4f6d8d1:cmd/loganalyzer/main.go >/dev/null; echo)  # 仅示意，请用 round9 基线二进制比对

# —— 测试覆盖确认 ——
go test -cover ./...                                      # 应 >=74.0%
```

请只输出结论（包结构、导出的 API、测试是否全部迁到子包且通过、覆盖率、重构是否引入任何行为差异）+ 若有发现的行为差异请单独列出。不要输出完整文件，也不要在对话里贴 diff——我会用 `git diff` 自行审阅。

---

## 本轮评分看点

- **重构不改行为**：把「一个 main 包」拆成 `parser`/`stats`/`output` + 薄 `main`，但 text/json 输出与 round 9 字节级一致、测试整体迁移后仍绿——展现「用测试守护契约、安全做结构演进」的成熟工程观，而非「为了好看重写导致回归」。
- **依赖方向清晰无环**：`output → stats → parser` 单向依赖，CLI 装配层不掺逻辑；为后续独立单测、复用、甚至做库铺好结构。
- **导出即契约**：函数从 unexported 升为 exported，意味着它们从此是包的公开 API——提示评审「作者开始用包边界思考」，也是把 round 10 的测试从 `package main` 内测升级为「子包自有测试」的自然结果。
