# Round 6 提示词 — 支持 `--contains` 关键字过滤

> 目标：在现有 `loganalyzer` 上新增一个可选的关键字过滤能力，且**不得**改动既有的行统计 / 级别分类逻辑、不得引入任何回归。
>
> 背景：前 5 轮已落定「流式读取 → 行级统计 → UTF-8 正确分级」。本轮是**第一个新功能增量轮**，重点不在算法，而在
> 「加功能时如何保证不把前 5 轮已验证正确的行为改坏」。这也是评审最看重的能力点。

---

【提示词正文】

请在 `loganalyzer` 上新增一个**可选**的命令行参数 `--contains <keyword>`，实现关键字过滤。

## 行为语义（必须严格遵守）

1. `--contains` 未提供、或提供**空串**时：**不启用过滤**，程序行为与 round 5（commit `08ca8f5`）**逐字节完全一致**——
   包括输出内容的每一行、每一个空格。这一条是硬约束，用下面的差分复测验证。
2. `--contains` 提供**非空**关键字 `kw` 时：只对**包含该子串的行**做统计。被过滤掉的行**不计入** `Total` / `Empty` / 任何级别计数。
3. 子串匹配用 `strings.Contains(line, kw)`。对合法 UTF-8 文本，该匹配按字节序列进行且是安全的——**中文关键字也能正确命中**，
   不会像 round 5 那样因逐字节误判出问题（可顺带对比：round 5 的坑是「把续字节当空白」，与本题无关，但同样属于「UTF-8 必须按 rune 思维」）。
4. 空行不可能 contains 非空关键字，因此自然被过滤掉（即启用过滤时空行不计入统计），这是预期行为。

## 输出格式

- 默认无过滤：输出格式与 round 5 一致，**不要新增 / 删减任何输出行**。
- 启用过滤时：在 `Total lines:` 这一行**上方**新增且仅新增一行：

  ```
  Filter: contains="<kw>"
  ```

  其余输出行格式与默认路径相同。这样「不带 flag」时输出零差异，「带 flag」时结果可解释。

## 实现约束

1. 过滤逻辑必须插入在**流式读取循环内部**：先取行，再判断是否 contains，不匹配则跳过本行统计；匹配才做 `Total++` / 空行判定 / `classifyLevel`。
   **不要**先把整文件读进切片再过滤（会破坏 round 2 确立的流式低内存模型）。
2. 改动范围控制在两处：`analyze`（新增 `contains string` 形参并加过滤判断）、`main`（新增 `flag.String("contains", "", ...)` 并传入）。
   **不要**改动 `classifyLevel` 的 UTF-8 解码逻辑，不要退回 `strings.Fields`，不要改动 `levelOrder` / 输出对齐逻辑 / 测试语料。
3. 注意循环结构：`reader.ReadString` 的 `for` 循环里，过滤「不匹配」的分支**不能**用裸 `continue` 直接跳到下一轮——
   那样会跳过循环末尾的 `readErr` 检查，导致最后一行（无末尾换行）被漏统计或漏判 EOF。请使用 `if/else` 包裹统计块，
   确保不管是否过滤，`readErr` 检查都照常执行。
4. 仅使用 Go 标准库（当前 import 已含 `strings`，无需新增）。

## 验收标准（差分测试 —— 本轮唯一的正确性判据）

- **零回归判据**：不带 `--contains` 时，在以下语料上的输出必须与 round 5 已知正确值**逐项一致**（round 5 已验证）：
  - `testdata/sample.log` → `Total lines: 17` / `Empty lines: 1` / `ERROR: 4` `WARN: 2` `INFO: 5` `DEBUG: 2` `UNKNOWN: 4`
  - `testdata/utf8_edge.log` → `ERROR: 4` `WARN: 1` `INFO: 2` `DEBUG: 1` `UNKNOWN: 0`
  - 空文件 → 全 0、退出码 0；末行无换行文件 → `Total` 等于实际行数；长行（>64KB）→ 不报 `token too long`
- **过滤正确性判据**（带 `--contains`）：
  - `testdata/utf8_edge.log --contains ERROR` → 仅含 ERROR 的 4 行被统计，期望 `Total lines: 4` / `ERROR: 4` / 其余级别 0，且首行输出 `Filter: contains="ERROR"`。
  - 临时中文语料 `--contains 汉字` → 能正确命中含「汉字」的行（验证 UTF-8 子串匹配无误判），自行核对行数。
  - `--contains 不存在的关键字zzz` → 全 0、退出码 0（不报错）。

---

## 自测清单（我会逐条复跑并与 round 5 做差分比对，请勿代填结果）

```bash
go vet ./... && gofmt -l . && go build -o loganalyzer ./cmd/loganalyzer

# —— 零回归：不带 flag，输出必须与 round 5 逐项一致 ——
./loganalyzer --file testdata/sample.log        # Total 17 / Empty 1 / E4 W2 I5 D2 U4
./loganalyzer --file testdata/utf8_edge.log      # E4 W1 I2 D1 U0
: > /tmp/empty.log && ./loganalyzer --file /tmp/empty.log   # 全 0，退出码 0
printf 'a\nb\nc' > /tmp/noeol.log && ./loganalyzer --file /tmp/noeol.log  # Total 3
python3 -c "open('/tmp/longline.log','w').write('A'*100000+'\n'+'short\n'+'\n')"
./loganalyzer --file /tmp/longline.log           # Total 3 / Empty 1，不报 token too long

# —— 过滤：基础 ——
./loganalyzer --file testdata/utf8_edge.log --contains ERROR   # Filter 行 + Total 4 / ERROR 4

# —— 过滤：中文 UTF-8 关键字，验证无字节误判 ——
printf '2026-08-07 模块汉字 INFO 正常\n2026-08-07 普通 ERROR 英文\n汉字开头 WARN 测试\n' > /tmp/cn.log
./loganalyzer --file /tmp/cn.log --contains 汉字   # 应命中第 1、3 行，自行核对

# —— 过滤：无匹配 ——
./loganalyzer --file testdata/sample.log --contains zzz_none   # 全 0，退出码 0

# —— 过滤：空关键字 == 不过滤（与默认行为一致） ——
./loganalyzer --file testdata/sample.log --contains ""   # 与不带 flag 输出逐字节相同
```

请只输出改动后的 `analyze` 函数、`main` 函数，以及 `import` 的变更说明。
不要输出完整文件，也不要在对话里贴 diff——我会用 `git diff` 自行审阅。

---

## 本轮评分看点

- **功能增量下的零回归意识**：加新功能最容易顺手改坏旧行为。用「默认路径字节级一致 + 差分复测」把回归风险锁死，
  这正是前几轮抓 bug（round 2 超长行、round 4 内存退化、round 5 UTF-8 回归）沉淀出的工程纪律。
- **流式模型不被破坏**：过滤只是一次 `strings.Contains` 的 O(行长) 扫描，不引入额外的整文件缓冲或字段切片，
  延续 round 2 的低内存设计。
- **UTF-8 子串匹配正确性**：`strings.Contains` 对合法 UTF-8 按字节序列匹配是安全的，中文关键字能正确命中；
  可与 round 5「逐字节误判空白」的坑对照，体现对 Go 字符串编码模型的整体把握。
- **输出可解释性**：启用过滤时显式打印过滤条件，让结果可被评审 / 用户直接读懂，而非默默改变数字。
