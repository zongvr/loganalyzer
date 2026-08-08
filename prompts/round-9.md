# Round 9 提示词 — `--top N` 高频日志模式 Top-N 展示

> 目标：在现有 `loganalyzer` 上新增 `--top N` 参数，在已有「流式读取 → 行级统计 → 级别分类 → 关键字/级别/时间过滤 → 多格式输出」能力之上，额外给出「出现频率最高的 N 条日志消息」榜单。
> 且**不得**改动既有的总行数 / 空行 / 级别统计、过滤逻辑与 text/json 输出格式（除非显式启用 `--top`）——零回归是硬约束。
>
> 背景：前 8 轮已落定基础统计、UTF-8 正确分级、关键字/级别/时间过滤、`text`/`json` 双格式输出。本轮是**第四个新功能增量轮**，重点是「在流式循环内顺带维护一张高频消息表，并把『Top-N 榜单』作为可选附加输出」——典型的「边读边聚合 + 可选附加视图」练习，也是评审看重的工程素养（在不破坏既有契约的前提下扩展信息维度）。

---

【提示词正文】

请在 `loganalyzer` 上新增一个命令行参数，实现**高频日志消息 Top-N 展示**：

- `--top <N>`：整数。`N >= 1` 时启用，程序在原有统计之后，额外输出「出现频率最高的 N 条日志消息」榜单（按频率降序）。`N <= 0`（含默认不提供）时**不启用**，程序行为与 round 8（commit `386c5c3`）**逐字节完全一致**——这一条是硬约束，用下方「零回归判据」差分复测验证。

## 行为语义（必须严格遵守）

1. **零回归硬约束**：`--top` 不提供、或 `--top 0` / 负数时，程序的**全部输出**（text 与 json 两种格式、含 `Filter:` 行、级别表格、每一个空格）必须与 round 8（`386c5c3`）**逐字节完全一致**。
   即：不启用 `--top` 时，**不得**新增任何输出行、不得改动既有任何输出行、json 不得新增字段。这条用下方「零回归判据」的差分复测锁死。
2. `--top N`（N>=1）与既有的 `--contains` / `--level` / `--from` / `--to` **正交**：Top-N 榜单统计的是「通过了所有生效过滤条件的行」中的高频消息；过滤逻辑完全不变，只是把「过滤后的结果」再额外聚合出一张榜单。
3. 空行（`strings.TrimSpace` 后为空）**不计入** Top-N 榜单（避免空行霸榜），但仍正常计入 `Empty lines` / 级别统计（既有行为不变）。

## 消息模板（高频聚合的 key 如何定义）

为让「同一类日志在不同时间重复出现」能聚到一起，需要**去掉行首的时间戳前缀**再作为聚合 key。定义如下（复用现有的 `parseTime` / `timeLayouts` 与 rune 感知的空白扫描，口径与 `classifyLevel` / `firstToken` 一致）：

- 取行首空白分隔的 token（rune 感知，不要用 `strings.Fields` 整体切分）。
- **贪婪判定**时间戳前缀：
  1. 若**前两个** token 用空格拼接后能经 `parseTime` 解析（即命中 `timeLayouts` 中「日期+时间」类布局，如 `2006-01-02 15:04:05` / `2006/01/02 15:04:05`），则时间戳 = 这两个 token，**去掉它们**；
  2. 否则若**第一个** token 能经 `parseTime` 解析（命中「日期」或「ISO 日期时间」类布局，如 `2006-01-02` / `2006/01/02` / `2006-01-02T15:04:05`），则时间戳 = 该 token，**去掉它**；
  3. 否则该行无可识别时间戳，**整行（去除首尾空白）即消息 key**。
- 去掉时间戳后若剩余为空串（例如一行只有时间戳），则用**整行（去除首尾空白）**作为 key，保证每一非空行都有唯一 key。
- 关键点：只在**行首**去掉时间戳，绝不去掉消息体内部的时间/数字；消息体（含其中可能存在的日期、数字）原样保留，作为 key 的一部分。

语义对齐：本榜单等价于对「去时间戳后的消息」做 Unix 经典管道 `... | sort | uniq -c | sort -rn | head -N`，但要求流式、不把整文件读进内存（沿用 round 2 的 `bufio.Reader` 流式模型）。

## 排序与截取规则

- 频率（出现次数）相同者，按**首次出现顺序**排列（先出现的排前面）——用「首次出现序号」作为次级排序键，保证结果**确定可复现**（不要依赖 map 遍历顺序）。
- 最终取频率最高的前 `N` 条；若去重后的消息总数 `< N`，则只返回实际存在的条数（榜单如实变短，不补空）。

## 输出格式

### text 模式（默认 / `--format text`）

在现有「Level statistics:」表格**之后**，新增且仅新增一段：

```
Top N messages:
   <count1>  <message1>
   <count2>  <message2>
   ...
```

- 标题行固定为 `Top N messages:`（N 为用户传入的 `--top` 值，即使实际条数少于 N 也照写 N）。
- 每条消息一行：前面 2 个空格，接着是**右对齐**的计数（字段宽度 = 榜单中最大计数的位数），再 2 个空格，最后是去时间戳后的消息原文。
- 若启用 `--top` 但经过滤后无非空行（榜单为空），仍打印标题行 `Top N messages:`，其下不打印任何消息行。

### json 模式（`--format json`）

当 `--top N`（N>=1）启用时，在原有 JSON 对象**末尾新增一个字段** `top`（注意：不启用 `--top` 时**不得**出现该字段，以保持 round 8 的 json 形状不变）：

```json
{"filter":"","total_lines":17,"empty_lines":1,"levels":[{"level":"ERROR","count":4},{"level":"WARN","count":2},{"level":"INFO","count":5},{"level":"DEBUG","count":2},{"level":"UNKNOWN","count":4}],"top":[{"message":"ERROR db connection failed","count":3},{"message":"WARN  cache miss","count":2}]}
```

- `top`：数组，按频率降序、同频按首次出现序；每个元素为 `{"message": <去时间戳后的消息>, "count": <次数>}`。
- 字段顺序：`filter` / `total_lines` / `empty_lines` / `levels` / `top`（与 round 8 相比仅在末尾多了 `top`；其余字段名、顺序、0 值处理完全不变）。
- 用结构体 + `json` tag 控制顺序（不要裸 `map`）。`top` 元素用 `[]struct{ Message string `json:"message"`; Count int `json:"count"` }`。
- 启用 `--top` 但榜单为空时，`top` 为 `[]`（空数组），字段仍出现。

## 实现约束

1. 高频计数必须融入**既有流式读取循环**（与 `--contains`/`--level`/时间过滤共用同一套 `if/else` 包裹结构）：仅在某行 `keep` 为真**且**非空时，才更新消息计数表。不要先把整文件读进切片再统计（破坏 round 2 流式低内存模型）。
2. **当 `--top <= 0` 时，不得分配计数 map、不得做任何额外聚合**——保证默认路径与 round 8 在性能与输出上都零差异。
3. 建议改动：
   - `analyze` 增加形参 `topN int`，签名改为 `func analyze(path, contains, level string, from, to *time.Time, topN int) (Stats, []msgCount, error)`；新增 `type msgCount struct { Message string; Count int }`（或带 json tag 的版本）。`topN <= 0` 时返回 `nil`（或空）切片、不聚合。
   - 新增辅助 `func messageKey(line string) string`（实现上文「消息模板」规则），在循环内对非空 kept 行调用。
   - `output` 增加形参 `top []msgCount, topN int`；text 分支在级别表格后按需打印榜单；json 分支在 `topN >= 1` 时追加 `top` 字段。
   - `main` 新增 `flag.Int("top", 0, "...")`，并把它与 `*topN` 传入 `analyze` / `output`。`flag.Int` 对非法值（非整数）会自动报错退出（exit 2），无需额外处理。
4. **不得**改动 `classifyLevel` 的 UTF-8 解码逻辑、不得退回 `strings.Fields`、不得改动 `Stats` 结构定义与 `levelOrder`、不得改动既有 text/json 输出内容、不得改动测试语料。
5. 仅使用 Go 标准库。

## 验收标准（差分测试 —— 零回归 + 新功能两部分）

### A. 零回归判据（`--top` 不启用时，必须与 round 8 逐项字节一致）

- `testdata/sample.log` → `Total lines: 17` / `Empty lines: 1` / `ERROR: 4` `WARN: 2` `INFO: 5` `DEBUG: 2` `UNKNOWN: 4`（与 round 8 逐字节一致）。
- `testdata/utf8_edge.log` → `ERROR: 4` `WARN: 1` `INFO: 2` `DEBUG: 1` `UNKNOWN: 0`。
- 空文件 → 全 0、退出码 0；末行无换行文件 → `Total` 等于实际行数；长行（>64KB）→ 不报 `token too long`。
- `testdata/utf8_edge.log --contains ERROR` → 与 round 8 逐字节一致：`Filter: contains="ERROR"` / `Total lines: 4` / `ERROR: 4`。
- `testdata/sample.log --format json`（不启用 `--top`）→ 与 round 8 json **逐字节一致，不得出现 `top` 字段**（这是 json 零回归的关键）。
- `testdata/sample.log --format text` 与默认（不启用 `--top`）逐字节一致。
- `diff <(./loganalyzer --file testdata/sample.log) <(./loganalyzer --file testdata/sample.log --top 0) && echo "top0==default IDENTICAL"` 应通过。

### B. 新功能判据（`--top` 启用时）

用一个重复消息明显的语料验证（自建临时文件，不要改仓库 testdata）：

```bash
cat > /tmp/top.log <<'EOF'
2026-08-07 09:00:01 ERROR db connection failed
2026-08-07 09:00:02 ERROR db connection failed
2026-08-07 09:00:03 WARN  cache miss
2026-08-07 09:00:04 ERROR db connection failed
2026-08-07 09:00:05 WARN  cache miss
2026-08-07 09:00:06 INFO  ok
EOF
```

- `./loganalyzer --file /tmp/top.log --top 2`（text）→ 榜单应为：
  ```
  Top 2 messages:
    3  ERROR db connection failed
    2  WARN  cache miss
  ```
  （`ERROR db connection failed` 出现 3 次居首，`WARN  cache miss` 出现 2 次次之；`INFO  ok` 1 次不进前 2。）
- `./loganalyzer --file /tmp/top.log --top 2 --format json` →
  `{"filter":"","total_lines":6,"empty_lines":0,"levels":[{"level":"ERROR","count":3},{"level":"WARN","count":2},{"level":"INFO","count":1},{"level":"DEBUG","count":0},{"level":"UNKNOWN","count":0}],"top":[{"message":"ERROR db connection failed","count":3},{"message":"WARN  cache miss","count":2}]}`
- 组合过滤：`/tmp/top.log --level ERROR --top 1` → 仅在 ERROR 行（3 行）里聚合，榜单 `ERROR db connection failed` count=3、top 字段仅 1 条。
- `--top 99`（超过去重条数）→ 返回全部去重消息（榜单如实变短，不报错、不补空）。
- `--top 0` / 不提供 `--top` → 不打印榜单、json 无 `top` 字段（见 A 判据）。

---

## 自测清单（我会逐条复跑并与 round 8 做差分比对，请勿代填结果）

```bash
go vet ./... && gofmt -l . && go build -o loganalyzer ./cmd/loganalyzer

# —— 零回归：不启用 --top 必须与 round 8 逐项一致 ——
./loganalyzer --file testdata/sample.log                      # Total 17 / Empty 1 / E4 W2 I5 D2 U4
./loganalyzer --file testdata/utf8_edge.log                    # E4 W1 I2 D1 U0
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log       # 全 0，退出码 0
printf 'a\nb\nc' > /tmp/noeol.log && ./loganalyzer --file /tmp/noeol.log  # Total 3
python3 -c "open('/tmp/longline.log','w').write('A'*100000+'\n'+'short\n'+'\n')"
./loganalyzer --file /tmp/longline.log                         # Total 3 / Empty 1，不报 token too long
diff <(./loganalyzer --file testdata/sample.log) <(./loganalyzer --file testdata/sample.log --top 0) && echo "top0==default IDENTICAL"

# —— 与 round 8 json 零回归（无 top 字段）——
./loganalyzer --file testdata/sample.log --format json | python3 -c "import sys,json; d=json.load(sys.stdin); print('top' in d)"   # 应输出 False

# —— 新功能：Top-N ——
cat > /tmp/top.log <<'EOF'
2026-08-07 09:00:01 ERROR db connection failed
2026-08-07 09:00:02 ERROR db connection failed
2026-08-07 09:00:03 WARN  cache miss
2026-08-07 09:00:04 ERROR db connection failed
2026-08-07 09:00:05 WARN  cache miss
2026-08-07 09:00:06 INFO  ok
EOF
./loganalyzer --file /tmp/top.log --top 2
./loganalyzer --file /tmp/top.log --top 2 --format json
./loganalyzer --file /tmp/top.log --level ERROR --top 1
./loganalyzer --file /tmp/top.log --top 99   # 返回全部去重消息
```

请只输出改动后的 `main` 函数（及新增的 `msgCount` 结构 / `messageKey` 辅助函数 / `analyze`/`output` 的签名与实现变更）、以及 `import` 的变更说明。
不要输出完整文件，也不要在对话里贴 diff——我会用 `git diff` 自行审阅。

---

## 本轮评分看点

- **可选附加视图，零侵入**：`--top` 是「在既有统计契约之上叠加一张榜单」——不启用时与 round 8 逐字节一致，启用时才多一段输出。这延续了「加功能不破坏已验证行为」的工程纪律，也是 CLI 工具做功能渐进最安全的范式。
- **流式聚合 + 低内存**：高频计数在既有 `bufio.Reader` 循环内顺带完成，不开第二遍文件、不把全文件读进内存（`--top<=0` 时甚至不分配 map），与 round 2 的流式原则一脉相承。
- **确定性排序**：同频消息按「首次出现序」定序，杜绝 `map` 遍历随机性，保证 Top-N 榜单在任意环境可复现——评审对「不确定输出」零容忍。
- **消息模板化的工程判断**：用「去行首时间戳」而非「全文正则替换」来做聚合 key，既能把「同一错误在不同时间」聚到一起，又不会误伤消息体内的日期/数字；语义对齐 Unix `sort | uniq -c | sort -rn | head -N`，展现对经典日志分析链路的理解。
