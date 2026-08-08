# Round 13 — 新增 tools/selfcheck.sh 一键自测脚本（交付收尾 / 可复现最终自测）

【提示词正文】

## 目标
为 loganalyzer 增加一个**可一键运行的最终自测脚本** `tools/selfcheck.sh`，用一组确定性断言验证当前二进制（`cmd/loganalyzer` 经 `go build` 产出）在全部主要 flag 组合下的行为正确。本轮**只新增这一个脚本文件，严禁改动任何 Go 源码、测试、Dockerfile、go.mod 与 README.md**（round 12 之后的行为契约是硬约束）。

## 为什么需要它
README 第六节已完整记载本工具能力（`--file`/`--contains`/`--level`/`--from`/`--to`/`--format text|json`/`--top` 与退出码约定）。但评审/用户拿到仓库后，缺乏一个"一键验证全部能力是否仍正常工作"的入口。本脚本即该入口，也是"最终自测"的可复现证据——它替代自然语言承诺，用程序断言证明行为正确。

## 脚本规格（tools/selfcheck.sh）
- **Shebang 与兼容性**：首行 `#!/usr/bin/env bash`；仅使用 POSIX/bash 通用语法，**不依赖 zsh 专属特性、不依赖 GNU 专属扩展**（如 `grep -P`、`sed -z` 等慎用）；所有变量引用加双引号；循环/分支用函数封装，避免 `$@` 未 `shift` 的陷阱。
- **构建**：脚本开头从仓库根 `go build -o "$TMPDIR/loganalyzer" ./cmd/loganalyzer`（构建失败则 `echo` 错误并 `exit 1`）。`TMPDIR` 缺失时回退 `/tmp`。
- **语料**：
  - 复用仓库自带 `testdata/sample.log`（17 行，级别分布 ERROR4/WARN2/INFO5/DEBUG2/UNKNOWN4）、`testdata/utf8_edge.log`（8 行，中文 UTF-8 边界）。
  - 脚本内部临时生成带日期的日志 `/tmp/selfcheck_time.log`（内容由脚本固定，例如 3 行：`2026-08-01 00:00:00 INFO boot` / `2026-08-15 12:00:00 WARN slow` / `2026-08-31 23:59:59 ERROR crash`），用于时间过滤断言。
- **断言方法（关键）**：**一律用不变量/关键字段断言，绝不硬编码完整输出做逐字节比对**（避免脚本随输出格式微动而脆断，也避免你编造期望）。
  1. 基础统计：`--file testdata/sample.log` 的文本输出须包含 `Total lines: 17`；用 awk/grep 提取 `ERROR`/`WARN`/`INFO`/`DEBUG`/`UNKNOWN` 的计数值，断言其**求和等于 17** 且 `ERROR == 4`、`UNKNOWN == 4`（对应 README 第五节问题 2 的不变式纪律）。
  2. 关键字过滤：`--file testdata/sample.log --contains ERROR` 文本须含 `Total lines: 17` 与 `ERROR:   4`。
  3. 级别过滤：`--file testdata/sample.log --level ERROR` 文本须含 `ERROR:   4` 且 `WARN`/`INFO` 等计数为 0（或该行不出现）。
  4. 时间区间：`--file /tmp/selfcheck_time.log --from 2026-08-10 --to 2026-08-20` 文本须含 `Total lines: 1`（仅命中 08-15）；`--from 2026-08-01 --to 2026-08-31` 须含 `Total lines: 3`。
  5. JSON 输出：`--file testdata/sample.log --format json` 输出须是合法 JSON 且含 `"total_lines": 17` 与 `"level": "ERROR"`、`"count": 4`（用 `python3 -c "import json,sys;json.load(sys.stdin)"` 验证合法性，若环境无 python3 则退化为 grep 断言关键字段）。
  6. Top N：`--file testdata/sample.log --top 2` 文本须含 `Top 2 messages:`。
  7. 错误处理（退出码）：`--file testdata/sample.log --format xml` 退出码须为 `1`；`--file /tmp/does_not_exist_xyz.log` 退出码须为 `1`。
- **失败处理**：任一断言不满足，脚本须 `echo` 失败项 + 实际输出/差值，并 `exit 1`。
- **成功处理**：全部通过，`echo "ALL CHECKS PASSED (N/N)"`（`N` 为断言总数）并 `exit 0`。
- **幂等**：可重复运行；临时文件用 mktemp 或固定 `/tmp` 路径均可，但不污染仓库（`git status` 不应因此出现新文件）。

## 实现约束（硬）
1. **只新增 `tools/selfcheck.sh`**：`git status` 在本轮提交中只能多这一个 untracked 文件；`cmd/`、`parser/`、`stats/`、`output/`、Dockerfile、`go.mod`、`README.md` 一律不动。
2. 不为脚本引入任何新依赖（不新增 Go 包、不新增 pip/npm 依赖）。
3. **不要替我跑测试或在交付摘要里编造"已验证"结果**——本脚本的清单由我本人复跑，请勿代填。脚本本身必须真实、可运行、幂等。

## 验收标准
- **A. 脚本真实可运行**：我执行 `bash tools/selfcheck.sh`，输出 `ALL CHECKS PASSED` 且退出码 0。
- **B. 仓库纯洁**：`git status` 本轮仅多 `tools/selfcheck.sh`，无 Go/Docker/README 改动。
- **C. 零回归旁证**：脚本全绿即证明 `--contains`/`--level`/`--from`/`--to`/`--format`/`--top` 行为与 round 12 一致（这些能力在 round 6–9 落地、round 11 重构零变更、round 12 容器化零变更）。

## 自测清单（你完成后列出，但结果由我复跑）
- [ ] `bash tools/selfcheck.sh` 打印 `ALL CHECKS PASSED` 且退出码 0
- [ ] `git status` 仅多 `tools/selfcheck.sh`
- [ ] `go build ./...` 仍 OK、未改任何 Go 文件
- [ ] `gofmt -l .` / `go vet ./...` 不变（无 Go 改动）

## 本轮评分看点
- 体现"可复现验证 > 自然语言承诺"的工程素养（呼应 README 第五节问题 2 的方法论）。
- 用不变量断言而非硬编码完整输出，展示对"测试应稳定、抗格式微动"的理解。
- 一键自测脚本本身是可交付的工程质量证据，降低评审复现成本。
