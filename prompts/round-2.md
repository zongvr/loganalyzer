# Round 2 提示词：修复超长行崩溃 Bug（代码审核纠错轮）

> 本轮定位：**候选人代码审核 → 发现 AI 生成代码的真实缺陷 → 下达纠错指令**。
> 这是考核说明中"代码审核纠错能力"的直接体现，含金量最高的一轮。

## 缺陷是怎么发现的（README「问题记录」可直接引用）

Round 1 交付后做边界测试，构造一个单行长度 10 万字符的日志：

```bash
python3 -c "open('/tmp/longline.log','w').write('A'*100000 + '\n' + 'short\n' + '\n')"
./loganalyzer --file /tmp/longline.log
```

期望 `Total lines: 3 / Empty lines: 1`，实际：

```
错误：读取文件失败：bufio.Scanner: token too long
exit=1
```

**根因**：`bufio.Scanner` 默认单 token 上限为 64KB（`bufio.MaxScanTokenSize`），
超长行直接报错退出。而日志场景中，序列化后的 JSON 请求体、Java 异常堆栈单行
超过 64KB 十分常见 —— 属于**生产可用性缺陷**，不是理论问题。

---

## 直接发给 Kilo Code 的指令（复制下方「【提示词正文】」整段）

【提示词正文】

上一轮你实现的 `countLines` 有一个生产级缺陷，本轮**只修这一个问题**，不要扩散到其他功能。

**缺陷复现：**

```bash
python3 -c "open('/tmp/longline.log','w').write('A'*100000 + '\n' + 'short\n' + '\n')"
./loganalyzer --file /tmp/longline.log
# 实际输出：错误：读取文件失败：bufio.Scanner: token too long（退出码 1）
# 期望输出：Total lines: 3 / Empty lines: 1（退出码 0）
```

**根因**：`bufio.Scanner` 默认单行上限 64KB（`bufio.MaxScanTokenSize`），日志中的长 JSON 或异常堆栈会超限。

**修复要求：**

1. 改造 `countLines`，使其能正确处理**任意长度**的单行，不得因行过长而报错。
2. 修复方式二选一，请说明你的选择理由：
   - 方案 A：保留 `bufio.Scanner`，用 `scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)` 提高上限；
   - 方案 B：改用 `bufio.Reader` + `ReadString('\n')`，天然无长度上限。
3. **必须保持流式读取**，内存占用不得随文件总大小线性增长（禁止把整个文件读进内存）。
4. 注意边界：文件**最后一行没有换行符**时，该行仍必须计入 `total`；空文件应输出 `Total lines: 0 / Empty lines: 0` 且退出码 0。
5. 空行判定逻辑保持不变（`strings.TrimSpace` 后为空）。
6. 仅使用 Go 标准库，不引入第三方依赖。
7. 在 `countLines` 上方加一行注释，说明为什么不能用默认 `bufio.Scanner`（给后续维护者看）。

**输出要求**：不要在对话里贴完整 diff。只说明：改了哪些文件、选了哪个方案及理由（≤3 行）、一条验证命令。

---

## 自测清单（提交前必须全绿）

```bash
go build -o loganalyzer ./cmd/loganalyzer

# 1) 超长行：期望 Total lines: 3 / Empty lines: 1，退出码 0
python3 -c "open('/tmp/longline.log','w').write('A'*100000 + '\n' + 'short\n' + '\n')"
./loganalyzer --file /tmp/longline.log; echo "exit=$?"

# 2) 无换行结尾：期望 Total lines: 2
printf 'a\nb' > /tmp/noeol.log
./loganalyzer --file /tmp/noeol.log

# 3) 空文件：期望 Total lines: 0 / Empty lines: 0，退出码 0
: > /tmp/empty.log
./loganalyzer --file /tmp/empty.log; echo "exit=$?"

# 4) 回归：原样本仍应 Total lines: 6 / Empty lines: 3
./loganalyzer --file test.log

# 5) 静态检查
go vet ./... && gofmt -l .
```

## 提交动作

```bash
git add cmd/loganalyzer/main.go
git commit -m "round 2: 修复 bufio.Scanner 64KB 超长行崩溃，支持任意长度行"
git rev-parse --short HEAD    # 记下哈希，填入 rounds.json
```

## 评分看点

- **代码审核能力**：不是被动接收 AI 代码，而是主动构造边界用例把缺陷揪出来。
- **根因定位**：能说清 `bufio.MaxScanTokenSize` 这一层机制，而非"报错就换个写法"。
- **指令精准度**：给出复现步骤 + 根因 + 两个候选方案 + 边界约束，AI 几乎不可能改错。
- **回归意识**：修 bug 的同时要求验证旧用例不被破坏。
