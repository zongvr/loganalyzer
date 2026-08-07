# Round 5 提示词 — 修复 round 4 性能优化引入的 UTF-8 分词回归

> 目标：修复 `classifyLevel` 按字节判定空白导致的中文日志级别误判，且**不得**退回 `strings.Fields`（会重新引入 round 4 已修掉的内存退化）。
>
> 背景：round 4 为消除内存退化，把 `strings.Fields` 换成了手动逐字符扫描。功能用例与内存指标都达标，
> 但引入了一个**只在中文日志下才暴露**的正确性回归——这是典型的「性能优化改坏语义」。

---

【提示词正文】

上一轮（round 4）你把 `classifyLevel` 改成手动扫描，成功消除了内存退化（我已复测：4MB 单行文件，1 个字段 11.7MB / 200 万字段 11.5MB，差距已消失），输出对齐也修好了。但这次改动**引入了一个正确性回归**，请修复。

## 缺陷复现

测试语料 `testdata/utf8_edge.log`（8 行，级别均位于前 3 字段内）：

```
2026-08-07 堀 ERROR 模块名含字节0xA0(位于中间) 应判ERROR
2026-08-07 习 WARN 模块名含字节0xA0(位于末尾) 应判WARN
2026-08-07 丅 INFO 模块名含字节0x85即NEL(位于末尾) 应判INFO
2026-08-07 塀 DEBUG 另一个含0xA0的汉字 应判DEBUG
2026-08-07 正常 ERROR 纯汉字无特殊字节 对照组 应判ERROR
2026-08-07 module ERROR 纯ASCII 对照组 应判ERROR
2026-08-07	堀	ERROR	制表符分隔+特殊汉字 应判ERROR
堀堀堀 堀堀 INFO 前两字段全为特殊汉字 应判INFO
```

| | ERROR | WARN | INFO | DEBUG | UNKNOWN |
|---|---|---|---|---|---|
| **正确期望** | 4 | 1 | 2 | 1 | **0** |
| round 3（`strings.Fields`） | 4 | 1 | 2 | 1 | 0 ✅ |
| **round 4（当前代码）** | **2** | 1 | **1** | 1 | **3** ❌ |

8 行里错了 3 行，错误率 37.5%。**全部错在含汉字 `堀` 的行上。**

## 根因

```go
for i < len(line) && unicode.IsSpace(rune(line[i])) {
```

`line[i]` 是一个 **byte**。`rune(line[i])` 只是把 0~255 的字节值当成码点，**这不是 UTF-8 解码**。

UTF-8 多字节序列的后续字节落在 `0x80~0xBF`。而 `unicode.IsSpace` 在 Latin-1 区间里认定 **`U+0085`（NEL）和 `U+00A0`（NBSP）是空白字符**。这两个值恰好都在续字节范围内：

- `堀` = `E5 A0 80` —— 中间的 `A0` 被当成空白 → 一个汉字被劈成 `\xE5` 和 `\x80` **两个字段**，把真正的级别 token 挤出了前 3 字段 → 判为 UNKNOWN。
- `习` = `E4 B9 A0` —— `A0` 在末尾，恰好只起到"分隔符"作用，字段数不变，所以**侥幸判对了**。

也就是说：**是否出错取决于 `0xA0`/`0x85` 落在多字节序列的哪个位置**，具有隐蔽的偶发性。中文日志在国内是常态，这个缺陷必须修。

## 修复要求

请在**保留 round 4 内存优化思路**（只扫描到第 3 个字段就停、不为整行分配字段切片）的前提下，修正空白判定。两个候选方案，请择一实现并说明理由：

- **方案 A（仅认 ASCII 空白）**：按字节扫描，但只把 `' '  '\t'  '\n'  '\v'  '\f'  '\r'` 视为分隔符。
  UTF-8 多字节序列的所有字节均 ≥ `0x80`，与 ASCII 空白不可能冲突，因此按字节扫描是安全的。
  代价：全角空格 `U+3000`、`U+00A0` 等 Unicode 空白不再被当作分隔符，与 `strings.Fields` 语义有细微差异。
- **方案 B（正确解码后判定）**：用 `utf8.DecodeRuneInString(line[i:])` 逐个解码出真正的 rune，再交给 `unicode.IsSpace`，
  按返回的 `size` 步进。语义与 `strings.Fields` **完全等价**，同样不为整行分配切片。

## 硬性约束

1. **禁止**退回 `strings.Fields` 或任何会为整行分配完整字段切片的写法。
2. 必须保持「取满 3 个字段即 `break`」的提前退出。
3. 级别优先级语义不变：`ERROR > WARN > INFO > DEBUG`，前 3 字段内命中优先级最高者；`INFO x ERROR` 必须判 `ERROR`。
4. 级别匹配仍为 `strings.ToUpper(field)` 后**精确相等**，不做前缀/包含匹配。
5. 不改动 `analyze` / `Stats` / 输出格式 / 对齐逻辑，**本轮只改 `classifyLevel`**。
6. 仅使用 Go 标准库。

## 验收标准（差分测试）

修复后，程序在所有语料上的**级别统计结果必须与 round 3 版本（commit `04d58ee`，`strings.Fields` 实现）逐项一致**。
这是本轮唯一的正确性判据——因为 round 3 的分词语义是已知正确的基线。

---

## 自测清单（我会逐条复跑并与 round 3 差分比对，请勿代填结果）

```bash
go vet ./... && gofmt -l . && go build -o loganalyzer ./cmd/loganalyzer

# 1) 回归本缺陷：期望 ERROR 4 / WARN 1 / INFO 2 / DEBUG 1 / UNKNOWN 0
./loganalyzer --file testdata/utf8_edge.log

# 2) 主用例不得回退：期望 Total 17 / Empty 1 / ERROR 4 / WARN 2 / INFO 5 / DEBUG 2 / UNKNOWN 4
./loganalyzer --file testdata/sample.log

# 3) 内存退化不得复发：两者峰值内存应接近（~11MB 量级）
python3 -c "open('/tmp/fewfields.log','w').write('X'*4000000+'\n')"
python3 -c "open('/tmp/manyfields.log','w').write(('x '*2000000).strip()+'\n')"
/usr/bin/time -l ./loganalyzer --file /tmp/fewfields.log  2>&1 | grep -E "Total|maximum resident"
/usr/bin/time -l ./loganalyzer --file /tmp/manyfields.log 2>&1 | grep -E "Total|maximum resident"

# 4) 优先级语义：期望 ERROR 1 / WARN 1 / INFO 1 / UNKNOWN 0
printf 'INFO x ERROR\nDEBUG y WARN\nINFO only\n' > /tmp/prio.log && ./loganalyzer --file /tmp/prio.log

# 5) 第 4 字段是级别时仍应判 UNKNOWN
printf 'a b c ERROR\n' > /tmp/f4.log && ./loganalyzer --file /tmp/f4.log

# 6) 既有回归：长行 / 空文件 / 末行无换行 / CRLF / 异常路径
python3 -c "open('/tmp/longline.log','w').write('A'*100000+'\n'+'short\n'+'\n')"
./loganalyzer --file /tmp/longline.log      # 期望 Total 3 / Empty 1，不报 token too long
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log   # 全 0，退出码 0
printf 'a\nb\nc' > /tmp/noeol.log && ./loganalyzer --file /tmp/noeol.log   # Total 3
printf 'a\r\n\r\nb\r\n' > /tmp/crlf.log && ./loganalyzer --file /tmp/crlf.log  # Total 3 / Empty 1
./loganalyzer ; ./loganalyzer --file no_such.log   # 均中文提示 + 退出码 1
```

请只输出修改后的 `classifyLevel` 函数与必要的 import 调整，并用一句话说明选了哪个方案、为什么。
不要输出完整文件，也不要在对话里贴 diff——我会用 `git diff` 自行审阅。

---

## 本轮评分看点

- **回归测试意识**：性能优化通过了全部既有功能用例与内存指标，却悄悄改坏了语义。
  能抓到它，靠的是「拿上一版做基线做差分比对」，而不是只看新用例是否通过。
- **对 Go 语言细节的掌握**：`rune(byte)` 不是 UTF-8 解码；`unicode.IsSpace` 在 Latin-1 区认 `U+0085` / `U+00A0`；
  UTF-8 续字节范围 `0x80~0xBF` 与之重叠——三者叠加才构成这个缺陷。
- **本地化敏感度**：这个 bug 只在中文日志下暴露，纯 ASCII 测试永远测不出来。
