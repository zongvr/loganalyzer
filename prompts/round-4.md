# Round 4 提示词 — 修正输出对齐 + 消除 classifyLevel 的内存退化

- **轮次目标**：修复上一轮遗留的两个缺陷——输出列未对齐、`classifyLevel` 对超长行存在内存退化。
- **对应 commit message**：`round 4: 修正级别统计输出对齐，classifyLevel 改为只提取前 3 个字段`
- **背景**：Round 3 交付的功能正确性没问题（17 行样例分级结果与不变式均正确），但人工复核时发现两处实现缺陷，本轮专项修复。

---

【提示词正文】

本轮修复 `cmd/loganalyzer/main.go` 中的两个缺陷，不新增任何功能。请只改这两处，不要顺带改动其他逻辑。

## 缺陷一：级别统计输出列未对齐

当前实际输出（注意 UNKNOWN 行比其他行多顶出一格，数字列没对齐）：

```
Level statistics:
  ERROR:  4
  WARN:   2
  INFO:   5
  DEBUG:  2
  UNKNOWN: 4
```

根因：输出用了 `fmt.Printf("  %-7s %d\n", lv+":", ...)`，宽度 7 不足以容纳最长的标签 `UNKNOWN:`（8 个字符），导致该行不被填充、数字列错位。

要求修正为（所有数字在同一列）：

```
Level statistics:
  ERROR:   4
  WARN:    2
  INFO:    5
  DEBUG:   2
  UNKNOWN: 4
```

请不要硬编码 5 行各自的空格数；用能容纳最长标签的字段宽度来统一控制。

## 缺陷二：`classifyLevel` 对超长行存在内存退化

现在的实现是：

```go
fields := strings.Fields(line)
if len(fields) > 3 {
    fields = fields[:3]
}
```

问题在于 `strings.Fields` 会先把**整行的所有字段**都切出来、分配一个完整的切片，然后才丢弃前 3 个之后的部分。函数实际只需要前 3 个字段，却为整行付出了代价。

实测证据（同样是 4MB 大小的单行文件，仅字段数不同）：

| 单行内容 | 字段数 | 进程峰值常驻内存 |
|---|---|---|
| 4MB 连续字符，无空格 | 1 | 11.5 MB |
| 4MB 的 `x x x ...` | 2,000,000 | 43.7 MB |

文件大小完全相同，仅因字段数暴增，内存就涨到 3.8 倍——这与 Round 2 确立的「内存不随输入规模无谓增长」的流式原则相违背。

要求：改写 `classifyLevel`，**在扫描到第 3 个字段后立即停止**，不再构造整行的字段切片。

实现约束：
1. 仅使用 Go 标准库，不引入第三方依赖。
2. 保持函数签名不变：`func classifyLevel(line string) string`。
3. 字段的切分口径必须与 `strings.Fields` 一致：以**连续空白字符**（空格、制表符等）作为分隔，忽略首尾空白与连续空白造成的空字段。可使用 `unicode.IsSpace` 判定。
4. 判定规则保持不变：取前 3 个字段，`strings.ToUpper` 后精确匹配，优先级 `ERROR > WARN > INFO > DEBUG`，都不命中则返回 `UNKNOWN`。
5. 注意：优先级是**按级别优先**而非按字段位置优先。例如某行前 3 个字段依次是 `INFO`、`x`、`ERROR`，结果必须是 `ERROR` 而不是 `INFO`。现有实现（外层遍历级别、内层遍历字段）已满足该语义，改写后请确保语义不变。
6. 顺带把 `type Stats struct` 的定义移到 `analyze` 函数**之前**——目前它被定义在使用它的函数之后，阅读时需要向下跳。

## 输出格式

改完后请用一段话说明你的实现思路即可，**不要在对话里粘贴完整代码或 diff**（我会用 `git diff` 自行审阅）。

---

## 自测清单（我会逐条复跑，请勿代填结果）

```bash
go vet ./... && gofmt -l . && go build -o loganalyzer ./cmd/loganalyzer

# 1) 对齐修正 + 分级结果回归，期望 Total 17 / Empty 1 / ERROR 4 / WARN 2 / INFO 5 / DEBUG 2 / UNKNOWN 4
#    且 5 个数字必须在同一列
./loganalyzer --file testdata/sample.log

# 2) 内存退化修复验证：两次峰值内存应接近（都在 ~11MB 量级），不再有 3.8 倍差距
python3 -c "open('/tmp/fewfields.log','w').write('X'*4000000+'\n')"
python3 -c "open('/tmp/manyfields.log','w').write(('x '*2000000).strip()+'\n')"
/usr/bin/time -l ./loganalyzer --file /tmp/fewfields.log  2>&1 | grep -E "Total|maximum resident"
/usr/bin/time -l ./loganalyzer --file /tmp/manyfields.log 2>&1 | grep -E "Total|maximum resident"

# 3) 优先级语义回归：前 3 字段为 INFO x ERROR，必须判为 ERROR
printf 'INFO x ERROR y\n' > /tmp/prio.log
./loganalyzer --file /tmp/prio.log      # 期望 ERROR: 1，其余 0

# 4) 制表符分隔回归：应与空格分隔等价
printf '2026-08-07\t09:00:01\tERROR\tmsg\n' > /tmp/tab.log
./loganalyzer --file /tmp/tab.log       # 期望 ERROR: 1

# 5) 第 4 个字段是级别时不应被误判
printf 'a b c ERROR\n' > /tmp/fourth.log
./loganalyzer --file /tmp/fourth.log    # 期望 UNKNOWN: 1

# 6) 既有回归：长行 / 空文件 / 末行无换行 / 异常路径
./loganalyzer --file test.log           # 期望 6 / 3
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log   # 期望全 0，退出码 0
./loganalyzer --file no_such.log        # 中文报错，退出码 1
```

---

## 本轮评分看点

- 能否读懂「只要前 3 个字段却切了整行」这类**非功能性**缺陷——它不会让程序报错，只会让程序在极端输入下变差，是靠压测和对照实验才暴露出来的。
- 输出对齐这种细节，体现的是对交付质量的要求，而不只是"能跑就行"。
- 修复时是否守住了原有语义（级别优先级、空白切分口径），没有为了优化而引入行为变化。
