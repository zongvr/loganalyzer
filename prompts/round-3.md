# Round 3 提示词 —— 日志级别分级统计

- **轮次目标**：在一次遍历内，除总行数/空行数外，额外按日志级别（ERROR / WARN / INFO / DEBUG / UNKNOWN）分级计数。
- **对应文件**：`cmd/loganalyzer/main.go`
- **测试数据**：`testdata/sample.log`（含小写级别、Java 堆栈续行、无级别自由文本、空行等边界）
- **评分看点**：需求拆解是否给出**无歧义的判定规则**；是否避免二次读文件；是否用结构体承载统计结果而非返回一长串裸值。

---

【提示词正文】

本轮为 `loganalyzer` 新增**日志级别分级统计**，**只做这一件事**，不要顺带增加过滤、输出格式等其他功能。

**一、重构要求**

当前 `countLines(path) (total, empty int, err error)` 的返回值已不够用。请重构为：

```go
type Stats struct {
    Total  int            // 总行数
    Empty  int            // 空行数（strings.TrimSpace 后为空）
    Levels map[string]int // 各级别行数
}

func analyze(path string) (Stats, error)
```

**硬性约束：只允许遍历文件一次**，在同一个循环里同时完成行数统计与级别统计，不得为了统计级别再读一遍文件。

**二、级别判定规则（必须严格按此实现，不要自行发挥）**

对每一行：

1. 用 `strings.Fields` 按空白切分，得到字段切片。
2. 取**前 3 个字段**（不足 3 个就取全部），逐个用 `strings.ToUpper` 转大写。
3. 若其中任一字段**精确等于** `ERROR`、`WARN`、`INFO`、`DEBUG` 之一，则该行判定为对应级别；按 `ERROR > WARN > INFO > DEBUG` 的优先级取第一个命中的。
4. 若无任何命中（包括空行、堆栈续行、纯文本行），判定为 `UNKNOWN`。

注意：判定**大小写不敏感**（样本中存在小写 `error`、`warn`）；只精确匹配 `WARN`，不要额外兼容 `WARNING`。

**三、输出格式**

在原有两行输出之后追加分级统计，级别顺序固定为 `ERROR / WARN / INFO / DEBUG / UNKNOWN`（**不要用 map 遍历顺序**，Go 的 map 遍历是随机的，必须用固定顺序的切片来控制输出）：

```
Total lines: 17
Empty lines: 1

Level statistics:
  ERROR:   4
  WARN:    2
  INFO:    5
  DEBUG:   2
  UNKNOWN: 4
```

数字左对齐、级别名后补空格对齐即可，不必追求复杂排版。

**四、保持不变的既有行为**

- 仍用 `bufio.Reader` + `ReadString('\n')` 流式逐行读取，**不得退回 `bufio.Scanner`**（上一轮刚修复的超长行缺陷不能回归）。
- 末行无换行符仍需计入总行数；空文件输出全 0 且退出码 0。
- 缺 `--file` 参数、文件不存在时，仍中文提示 + 退出码 1。
- **仅使用 Go 标准库**，禁止 `go get` 引入任何第三方依赖。

**五、正确性不变式**

`ERROR + WARN + INFO + DEBUG + UNKNOWN` 必须**恒等于** `Total lines`。请自行检查实现是否满足该等式。

**六、输出要求**

改完后只需简述改了哪些函数、关键改动点即可，**不要在对话里贴完整 diff 或完整文件内容**。

---

## 自测清单（改完后由我执行）

```bash
go vet ./... && gofmt -l . && go build -o loganalyzer ./cmd/loganalyzer

# 1) 分级统计主用例，期望 Total 17 / Empty 1 / ERROR 4 / WARN 2 / INFO 5 / DEBUG 2 / UNKNOWN 4
./loganalyzer --file testdata/sample.log

# 2) 不变式校验：各级别之和 == 总行数
# 3) 回归：超长行不得复发
python3 -c "open('/tmp/longline.log','w').write('A'*100000 + '\n' + 'short\n' + '\n')"
./loganalyzer --file /tmp/longline.log      # 期望 Total 3 / Empty 1，不报 token too long

# 4) 回归：空文件 0/0 退出码 0；末行无换行符正常计数
# 5) 回归：缺参数 / 文件不存在 → 中文提示 + 退出码 1
# 6) 输出顺序稳定性：连续运行 5 次，级别顺序必须完全一致（防 map 随机遍历）
```

## 预期得分点

- 提示词给出了**可验证的不变式**（各级别之和 == 总行数），而非只描述功能。
- 主动预判并规避 Go 的**map 遍历随机性**陷阱——这是 AI 生成此类代码的高频缺陷。
- 明确"只遍历一次"的性能约束，避免 AI 图省事读两遍文件。
- 判定规则精确到"取前 3 个字段、精确匹配、优先级顺序"，消除歧义，便于逐条验收。
