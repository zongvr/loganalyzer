# Round 10 提示词 — 单元测试覆盖解析与统计

> 目标：为现有 `loganalyzer`（Go CLI）补齐**单元测试**，覆盖「行级解析 / 级别判定 / 时间戳解析 / 消息聚类」与「流式统计 / 多格式输出」两大块逻辑。
> 本轮是**测试增量轮**：只新增测试用例，**不得改动任何既有函数（含 `analyze` / `output`）的行为、签名或输出格式**——round 9（commit `4f6d8d1`）的 text/json 字节级契约是硬约束，用下方「零回归判据」锁死。
>
> 背景：前 9 轮已落定基础统计、UTF-8 正确分级、关键字/级别/时间过滤、`text`/`json` 双格式输出、高频 Top-N 榜单。这些能力的核心逻辑（`classifyLevel` / `messageKey` / `parseTime` / `analyze` / `output` 等）目前**没有任何自动化测试**，本轮要补上——这是评审重点看的「工程成熟度」（功能迭代后是否用测试守住契约），也是为 round 11 的拆包重构铺路（有测试才能安全重构）。

---

【提示词正文】

请为 `cmd/loganalyzer/main.go` 中的核心函数编写单元测试，新建文件 `cmd/loganalyzer/main_test.go`（同属 `package main`，可直接调用未导出函数）。

## 范围与硬约束

1. **只新增 `cmd/loganalyzer/main_test.go` 一个文件**。不得修改 `main.go` 中任何函数（含 `analyze` / `output` / `classifyLevel` / `messageKey` / `parseTime` / `lineTime` / `firstToken`）的**实现、签名、输出字符串**。若你认为测试需要改签名才能做，请**改为在测试里用等价手段达到目的**（见下「测试技巧」），不要动 `main.go`。
2. 测试必须能通过 `go test ./...` 且全部为绿；`go vet ./...` 与 `gofmt -l .` 必须干净。
3. 仅使用 Go 标准库（`testing` + 捕获所需的 `os` / `io` / `bytes` / `encoding/json` / `time` 等），不引入第三方依赖。
4. round 9 的输出契约（text/json 逐字节、Top-N 字段、过滤行为）**必须保持不变**。本轮结束后用下方「零回归判据」差分复测，`diff` 必须为空。

## 必须覆盖的测试点（建议表驱动 test case）

### A. 解析与聚类纯函数

- **`classifyLevel`**（级别判定）：
  - 正常：`"2026-08-07 09:00:01 ERROR db connection failed"` → `"ERROR"`；`"WARN  cache miss"` → `"WARN"`；`"INFO  ok"` → `"INFO"`；`"DEBUG trace x"` → `"DEBUG"`。
  - 优先级：`"ERROR something WARN"`（ERROR 在前）→ `"ERROR"`；`"WARN then ERROR"` → 仍应是 `"ERROR"`（因为外层按 `levelOrder` 顺序匹配，ERROR 优先于 WARN）。
  - 仅扫描前 3 个字段：超长多字段行不得引发内存退化、不得误判。
  - **UTF-8 正确性**：从 `testdata/utf8_edge.log` 取样若干行（如含中文级别别名的行）确认其归类符合 round 5 的修复预期；纯中文行（无 ERROR/WARN/INFO/DEBUG 字段，如 `"信息 启动完成"`）→ `"UNKNOWN"`。
  - 无关键字行 → `"UNKNOWN"`。
- **`messageKey`**（去行首时间戳聚类）：
  - 日期+时间：`"2026-08-07 09:00:01 ERROR db connection failed"` → `"ERROR db connection failed"`。
  - 仅日期：`"2026-08-07 ERROR foo"` → `"ERROR foo"`。
  - ISO：`"2026-08-07T09:00:01 INFO x"` → `"INFO x"`。
  - 无可识别时间戳：`"no timestamp here"` → `"no timestamp here"`。
  - 仅时间戳的行：去掉后为空 → 退回整行（TrimSpace 后）。
  - 消息体内部的日期/数字**不得**被误删：`"ERROR order 2026-08-07 created"` → 保持原样（因为时间戳只匹配行首）。
- **`parseTime` / `lineTime`**：逐一验证 `timeLayouts` 中每个布局能解析（`2006-01-02 15:04:05` / `2006-01-02T15:04:05` / `2006-01-02` / `2006/01/02 15:04:05` / `2006/01/02`）；非法串（如 `"2026-13-99"`、`"hello"`）→ 返回 `ok=false`。
- **`firstToken`**：`"  hello world"` → `"hello"`；空行 → `""`。

### B. 流式统计 `analyze`（用临时文件，不依赖仓库 testdata）

- 写一个临时 `.log`（用 `os.CreateTemp` + `t.Cleanup` 删除），内容含若干已知行：
  - 验证 `Stats.Total` / `Stats.Empty` / `Stats.Levels` 计数正确（含 ERROR/WARN/INFO/DEBUG/UNKNOWN 五档）。
  - 末行无换行：统计不能漏掉最后一行。
- **过滤条件**（直接传 `contains` / `level` / `from` / `to` 形参，不跑 main）：
  - `contains="ERROR"` → 仅含该子串的行计入。
  - `level="ERROR"`（`EqualFold`，大小写不敏感）→ 仅该级别。
  - `from`/`to` 时间区间 → 仅落在区间内（边界 `==` 也计入；无法解析时间戳的行被排除）。
- **Top-N**：`topN>=1` 时返回的高频榜长度与排序正确（频率降序、同频首次出现序）；`topN<=0` 时返回 `nil`（不聚合，性能零差异）。

### C. 输出 `output`（捕获 stdout，验证与契约一致）

- 因 `output` 直接写 `os.Stdout`，在测试中用 `os.Pipe()` 重定向 `os.Stdout` 捕获字符串（见下「测试技巧」），**不要**改 `output` 签名去接 `io.Writer`。
- text 模式：构造一个已知 `Stats`，断言输出字符串与手写的期望完全一致（含对齐空格、`Level statistics:` 标题、`Top N messages:` 段落仅在 `topN>=1` 时存在）。
- json 模式：构造已知 `Stats` + `top`，用 `encoding/json` 解析捕获到的字符串并断言字段名/顺序/0 值；`topN<1` 时 `top` 字段**不得出现**，`topN>=1` 时 `top` 为 `[]`（空时）而非 `null`。

## 测试技巧（避免改动 main.go）

- 捕获 stdout：
  ```go
  func captureStdout(fn func() error) (string, error) {
      old := os.Stdout
      r, w, _ := os.Pipe()
      os.Stdout = w
      err := fn()
      w.Close()
      os.Stdout = old
      var buf bytes.Buffer
      io.Copy(&buf, r)
      return buf.String(), err
  }
  // 用法：out, _ := captureStdout(func() error { return output(stats, "text", "", nil, 0) })
  ```
- `analyze` 读文件：用 `os.CreateTemp(t.TempDir(), "log-*.log")` 写用例，路径传给 `analyze`，无需改其内部 `os.Open`。
- 表驱动：解析类函数（classifyLevel / messageKey / parseTime / firstToken）用 `tests := []struct{ in, want string }{...}` 循环断言。

## 验收标准（差分测试 —— 零回归 + 测试绿两部分）

### A. 零回归判据（main.go 行为必须与 round 9 逐项字节一致）

- `testdata/sample.log` → `Total lines: 17` / `Empty lines: 1` / `ERROR: 4` `WARN: 2` `INFO: 5` `DEBUG: 2` `UNKNOWN: 4`。
- `testdata/utf8_edge.log` → `E4 W1 I2 D1 U0`。
- 空文件 → 全 0、退出码 0；末行无换行文件 → `Total` 等于实际行数。
- `testdata/sample.log --format json`（不启用 `--top`）→ 与 round 8/9 json 逐字节一致，`top` 字段不得出现。
- `diff <(./loganalyzer --file testdata/sample.log) <(./loganalyzer --file testdata/sample.log --top 0) && echo IDENTICAL` 应通过。

### B. 测试判据（本轮交付物）

- `go test ./...` 全部通过（解析类 + analyze 各类过滤 + output 两类格式）。
- `go vet ./...` 干净；`gofmt -l .` 无输出。
- 测试用例需覆盖：classifyLevel 的 UTF-8 与优先级、messageKey 的三种时间戳剥离分支、parseTime 各布局与非法串、analyze 的 contains/level/时间过滤与 Top-N、output 的 text 与 json 契约（含 `top` 字段的有无）。

---

## 自测清单（我会逐条复跑并与 round 9 做差分比对，请勿代填结果）

```bash
go vet ./... && gofmt -l . && go test ./...
# 上述必须全绿 / 干净

go build -o loganalyzer ./cmd/loganalyzer

# —— 零回归：行为与 round 9 (4f6d8d1) 逐项一致 ——
./loganalyzer --file testdata/sample.log                  # Total 17 / Empty 1 / E4 W2 I5 D2 U4
./loganalyzer --file testdata/utf8_edge.log                # E4 W1 I2 D1 U0
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log  # 全 0，退出码 0
printf 'a\nb\nc' > /tmp/noeol.log && ./loganalyzer --file /tmp/noeol.log  # Total 3
diff <(./loganalyzer --file testdata/sample.log) <(./loganalyzer --file testdata/sample.log --top 0) && echo "top0==default IDENTICAL"

# —— 测试覆盖确认 ——
go test -v ./cmd/loganalyzer                              # 列出各子测试名，确认覆盖解析/统计/输出
go test -cover ./cmd/loganalyzer                          # 打印覆盖率（建议解析类函数接近 100%）
```

请只输出结论（测试文件是否新增、覆盖了哪些函数、覆盖率大致多少、是否发现既有 bug）+ 若有发现的既有缺陷请单独列出。不要输出完整文件，也不要在对话里贴 diff——我会用 `git diff` 自行审阅。

---

## 本轮评分看点

- **用测试守住已验证契约**：在 9 轮功能迭代后补自动化测试，且不改动任何既有实现——展现「先有契约、再用测试固化契约」的成熟工程观，也是后续 round 11 拆包重构的安全网。
- **UTF-8 与边界不放过**：classifyLevel 的 UTF-8 解码、messageKey 的三种时间戳剥离分支、parseTime 各布局与非法串、末行无换行——这些都是历史轮次踩过的坑，测试把它们变成「回归护栏」。
- **测试技巧分寸**：用 `os.Pipe` 捕获 stdout 验证 `output`、用 `os.CreateTemp` 驱动 `analyze`，而非改签名迎合测试——体现对「测试不应倒逼私有契约变形」的理解。
