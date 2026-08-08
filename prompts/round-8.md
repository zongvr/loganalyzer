# Round 8 提示词 — `--format text/json` 两种输出格式

> 目标：在现有 `loganalyzer` 上新增 `--format` 开关，支持两种输出格式——`text`（默认，沿用现有表格样式）与 `json`（结构化 JSON）。
> 且**不得**改动既有的行统计 / 级别分类 / 过滤逻辑、不得引入任何回归。
>
> 背景：前 7 轮已落定「流式读取 → 行级统计 → UTF-8 正确分级 → 关键字 / 级别 / 时间过滤」。本轮是**第三个新功能增量轮**，
> 重点是「在不触动任何统计与过滤逻辑的前提下，把『输出渲染』从 `main` 里抽出来做分支」——这是关注点分离（输出层 vs 计算层）的典型练习，
> 也是评审看重的工程素养。

---

【提示词正文】

请在 `loganalyzer` 上新增一个命令行参数，实现**输出格式切换**：

- `--format <MODE>`：取值为 `text` 或 `json`，默认 `text`（即不提供该参数时等价于 `--format text`）。
- `text`：沿用当前已有的纯文本表格输出（见下方「零回归判据」逐字节比对基准）。
- `json`：输出一行紧凑 JSON，结构见「JSON 输出结构」一节。

## 行为语义（必须严格遵守）

1. **零回归硬约束**：`--format` 不提供、或 `--format text` 时，程序的**全部输出**（含每一行、每一个空格、是否打印 `Filter:` 行）必须与 round 7（commit `ee21166`）**逐字节完全一致**。
   这一条用下方「零回归判据」的差分复测验证——这是本轮唯一的正确性底线，任何 text 输出的细微改动（多一个空行、列宽变化、顺序变化）都算回归。
2. `--format` 的合法值为 `text` / `json` 两档（大小写不敏感，即 `TEXT`/`Json` 也接受）。**非法值**（如 `xml`、`csv`、`""`）→ 向 stderr 打印错误并以**退出码 1** 退出。
   注意：默认不提供 `--format` 时**不要**报错，必须按 `text` 处理。
3. `--format` 与既有的 `--contains` / `--level` / `--from` / `--to` **正交**：过滤逻辑完全不变，JSON 模式只是把「过滤后的统计结果」换一种格式渲染。
   `Filter:` 信息在 JSON 里转为 `filter` 字段（见结构定义），其余统计字段一一对应。

## JSON 输出结构

`--format json` 时，向 stdout 打印**一个 JSON 对象**（单行、紧凑，`encoding/json` 默认 marshal 即可，不要美化缩进），字段与顺序如下：

```json
{"filter":"","total_lines":17,"empty_lines":1,"levels":[{"level":"ERROR","count":4},{"level":"WARN","count":2},{"level":"INFO","count":5},{"level":"DEBUG","count":2},{"level":"UNKNOWN","count":4}]}
```

- `filter`：字符串。任意过滤生效时，值与 text 模式的 `Filter:` 行**完全一致**（即 `contains="..." level="..." from="..." to="..."` 的拼接，顺序固定为 contains / level / from / to，未生效条件不出现）；未启用任何过滤时为空串 `""`。
- `total_lines` / `empty_lines`：整数，分别对应 text 模式的 `Total lines:` / `Empty lines:`。
- `levels`：数组，**顺序固定为 `levelOrder`（`ERROR`/`WARN`/`INFO`/`DEBUG`/`UNKNOWN`）**，每个元素为 `{"level": <级别名>, "count": <行数>}`；所有 5 个级别都出现（含计数为 0 的级别），不要省略 0 值。
- 用结构体 + `json` tag 控制字段名与顺序（不要用裸 `map[string]int`，否则级别顺序不可控）；`levels` 用 `[]struct{ Level string \`json:"level"\`; Count int \`json:"count"\` }` 保证顺序固定。

## 实现约束

1. **只改 `main`，不要改 `analyze` 的签名与实现**。`analyze` 依旧返回 `Stats`，过滤照旧在 `analyze` 内完成；新增的只是「拿到 `Stats` 后如何渲染」。
2. 推荐做法：把现在 `main` 里「从 `filterParts` 打印到级别表格」的那段输出代码抽成一个函数（如 `func output(stats Stats, format string, filter string) error`，
   其中 `filter` 是已拼接好的过滤字符串，无过滤时为 `""`），在 `main` 里根据 `--format` 分支调用 text / json 两种渲染。也可以直接在原 `main` 内用 `if/else` 分支——任选其一，
   但**text 分支的 `fmt.Printf` 必须与当前完全一致，一个字符都不能差**。
3. json 渲染用标准库 `encoding/json` 的 `json.Marshal`（紧凑、无缩进）；打印时用 `fmt.Println(string(b))` 或 `os.Stdout.Write(b)`。
4. `--format` 非法值校验放在 `flag.Parse()` 之后、调用 `analyze` 之前（与现有 `--file` 必填校验同区），非法则 stderr + `os.Exit(1)`。
5. 改动范围仅限 `main`（新增 `--format` flag + 输出分支/抽函数 + 必要的 `import "encoding/json"`）。
   **不要**改动 `classifyLevel` / `firstToken` / `parseTime` / `lineTime`、不要改动 `Stats` 结构定义与 `levelOrder`、不要退回 `strings.Fields`、不要改动测试语料。
6. 仅使用 Go 标准库。

## 验收标准（差分测试 —— 本轮唯一的正确性判据）

- **零回归判据（text 路径必须与 round 7 逐项字节一致）**：
  - `testdata/sample.log` → `Total lines: 17` / `Empty lines: 1` / `ERROR: 4` `WARN: 2` `INFO: 5` `DEBUG: 2` `UNKNOWN: 4`（与 round 7 逐字节一致）。
  - `testdata/utf8_edge.log` → `ERROR: 4` `WARN: 1` `INFO: 2` `DEBUG: 1` `UNKNOWN: 0`。
  - 空文件 → 全 0、退出码 0；末行无换行文件 → `Total` 等于实际行数；长行（>64KB）→ 不报 `token too long`。
  - `--format text --contains ERROR`（utf8_edge.log）→ 与 round 7 该用例逐字节一致：`Filter: contains="ERROR"` / `Total lines: 4` / `ERROR: 4`。
  - 不带 `--format`（默认）→ 与显式 `--format text` 逐字节一致。
- **JSON 判据**（用 `python3 -m json.tool` 或人工比对字段与顺序）：
  - `testdata/sample.log --format json` →
    `{"filter":"","total_lines":17,"empty_lines":1,"levels":[{"level":"ERROR","count":4},{"level":"WARN","count":2},{"level":"INFO","count":5},{"level":"DEBUG","count":2},{"level":"UNKNOWN","count":4}]}`
  - `testdata/utf8_edge.log --contains ERROR --format json` →
    `{"filter":"contains=\"ERROR\"","total_lines":4,"empty_lines":0,"levels":[{"level":"ERROR","count":4},{"level":"WARN","count":0},{"level":"INFO","count":0},{"level":"DEBUG","count":0},{"level":"UNKNOWN","count":0}]}`
  - `testdata/utf8_edge.log --format json` → `filter` 为 `""`，`total_lines:8`，`levels` 为 `ERROR:4/WARN:1/INFO:2/DEBUG:1/UNKNOWN:0`。
- **非法格式判据**：`--format xml` / `--format csv` → stderr 报错、退出码 1；不提供 `--format` 不报错（按 text 处理）。

---

## 自测清单（我会逐条复跑并与 round 7 做差分比对，请勿代填结果）

```bash
go vet ./... && gofmt -l . && go build -o loganalyzer ./cmd/loganalyzer

# —— 零回归：text 路径必须与 round 7 逐项一致 ——
./loganalyzer --file testdata/sample.log                      # Total 17 / Empty 1 / E4 W2 I5 D2 U4
./loganalyzer --file testdata/utf8_edge.log                    # E4 W1 I2 D1 U0
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log       # 全 0，退出码 0
printf 'a\nb\nc' > /tmp/noeol.log && ./loganalyzer --file /tmp/noeol.log  # Total 3
python3 -c "open('/tmp/longline.log','w').write('A'*100000+'\n'+'short\n'+'\n')"
./loganalyzer --file /tmp/longline.log                         # Total 3 / Empty 1，不报 token too long
./loganalyzer --file testdata/utf8_edge.log --contains ERROR   # 与 round7 逐字节一致：Filter: contains="ERROR" / Total 4 / ERROR 4
./loganalyzer --file testdata/sample.log --format text         # 必须与上一条（不带 --format）逐字节一致
diff <(./loganalyzer --file testdata/sample.log) <(./loganalyzer --file testdata/sample.log --format text) && echo "TEXT==default IDENTICAL"

# —— JSON 输出 ——
./loganalyzer --file testdata/sample.log --format json
./loganalyzer --file testdata/utf8_edge.log --format json
./loganalyzer --file testdata/utf8_edge.log --contains ERROR --format json
./loganalyzer --file testdata/utf8_edge.log --level ERROR --from 2026-01-01 --format json   # filter 应为 contains/level/from 拼接（未用 to）

# —— 非法格式 ——
./loganalyzer --file testdata/sample.log --format xml   # 退出码 1（非法格式）
./loganalyzer --file testdata/sample.log --format csv   # 退出码 1（非法格式）
```

请只输出改动后的 `main` 函数（及新增的输出辅助结构 / 函数）、以及 `import` 的变更说明。
不要输出完整文件，也不要在对话里贴 diff——我会用 `git diff` 自行审阅。

---

## 本轮评分看点

- **关注点分离（输出层 vs 计算层）**：把「统计结果如何渲染」从 `main` 里抽出来做 `text` / `json` 分支，而 `analyze` 的统计与过滤逻辑**一行不动**。
  这是 CLI 工具演进到中期的典型重构——新增交付格式不该牵连核心计算，正是前几轮「加功能不破坏已验证行为」纪律的延续。
- **默认路径零回归意识**：text 模式是 7 轮来的「基线契约」，任何空格 / 空行 / 列宽的改动都算回归。用「`--format text` 与默认路径差分零差异 + 与 round 7 历史值差分」双重锁死。
- **结构化输出的确定性**：JSON 用带 `json` tag 的结构体 + 固定顺序的 `levels` 数组（而非裸 map），保证字段名、字段顺序、级别顺序在任何环境都**可复现**，
  避免 `map` 序列化顺序不确定带来的评审困惑。
- **输入契约的健壮性**：`--format` 非法值显式报错退出（exit 1），而默认缺失按 `text` 处理——区分「用户显式给错」与「用户没给」，体现 CLI 输入校验的边界把控。
