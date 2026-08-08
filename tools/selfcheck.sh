#!/usr/bin/env bash
# 华傲数据考核 - loganalyzer 最终自测脚本
# 一键验证二进制在各主要 flag 组合下的行为正确性。
# 断言一律使用不变量 / 关键字段，绝不硬编码完整输出做逐字节比对（抗格式微动）。
# 用法：bash tools/selfcheck.sh （幂等，可重复运行，不污染仓库）
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="${TMPDIR:-/tmp}"
BIN="${TMPDIR}/loganalyzer-selfcheck"
TIME_LOG="${TMPDIR}/selfcheck_time.log"

PASS=0
FAIL=0

# 记录一次断言结果
record() {
  local ok="$1" name="$2"
  if [ "$ok" = "0" ]; then
    PASS=$((PASS + 1))
    printf '  [PASS] %s\n' "$name"
  else
    FAIL=$((FAIL + 1))
    printf '  [FAIL] %s\n' "$name"
  fi
}

# 断言：字符串包含子串
assert_contains() {
  local haystack="$1" needle="$2" name="$3"
  case "$haystack" in
    *"$needle"*) record 0 "$name" ;;
    *)
      record 1 "$name"
      printf '    期望包含: %s\n    实际输出:\n%s\n' "$needle" "$haystack"
      ;;
  esac
}

# 断言：退出码等于期望值
assert_exit() {
  local actual="$1" want="$2" name="$3"
  if [ "$actual" = "$want" ]; then
    record 0 "$name"
  else
    record 1 "$name"
    printf '    期望退出码 %s，实际 %s\n' "$want" "$actual"
  fi
}

# 提取某级别的计数值（从 text 的 "Level statistics:" 段，格式如 "  ERROR:   4"）
level_count() {
  local output="$1" lv="$2"
  printf '%s\n' "$output" \
    | awk -v lvl="$lv:" '$1 == lvl { gsub(/^[[:space:]]+/, ""); split($0, a, /:[[:space:]]+/); print a[2] }'
}

# 提取 "Total lines: N" 中的 N
total_lines() {
  printf '%s\n' "$1" | awk '/^Total lines: / { print $3 }'
}

# 检查 JSON 合法性：优先 python3，退化用 grep
json_ok() {
  local output="$1"
  if command -v python3 >/dev/null 2>&1; then
    printf '%s\n' "$output" | python3 -c 'import json,sys; json.load(sys.stdin)' >/dev/null 2>&1
  else
    case "$output" in
      *'"total_lines"'*) return 0 ;;
      *) return 1 ;;
    esac
  fi
}

main() {
  echo "==> 构建二进制"
  if ! (cd "$ROOT" && go build -o "$BIN" ./cmd/loganalyzer); then
    echo "错误：go build 失败" >&2
    exit 1
  fi

  # 生成时间过滤语料（固定内容，幂等）
  printf '2026-08-01 00:00:00 INFO boot\n2026-08-15 12:00:00 WARN slow\n2026-08-31 23:59:59 ERROR crash\n' > "$TIME_LOG"

  SAMPLE="$ROOT/testdata/sample.log"
  UTF8="$ROOT/testdata/utf8_edge.log"

  echo "==> 1. 基础统计（sample.log）"
  out="$("$BIN" --file "$SAMPLE")"
  assert_contains "$out" "Total lines: 17" "基础统计包含 Total lines: 17"
  assert_contains "$out" "ERROR:   4" "基础统计 ERROR 计数为 4"
  assert_contains "$out" "UNKNOWN: 4" "基础统计 UNKNOWN 计数为 4"
  # 不变式：五档级别计数之和 == Total lines
  e=$(level_count "$out" ERROR); w=$(level_count "$out" WARN)
  i=$(level_count "$out" INFO); d=$(level_count "$out" DEBUG)
  u=$(level_count "$out" UNKNOWN)
  sum=$((e + w + i + d + u))
  if [ "$sum" = "$(total_lines "$out")" ]; then
    record 0 "五档计数之和($sum) == Total lines"
  else
    record 1 "五档计数之和($sum) == Total lines"
    printf '    期望 %s，实际 %s（E=%s W=%s I=%s D=%s U=%s）\n' \
      "$(total_lines "$out")" "$sum" "$e" "$w" "$i" "$d" "$u"
  fi

  echo "==> 2. 关键字过滤（--contains ERROR）"
  out="$("$BIN" --file "$SAMPLE" --contains ERROR)"
  assert_contains "$out" 'Filter: contains="ERROR"' "过滤含 Filter 行"
  # 不变式：contains 过滤后命中行级别均为 ERROR，故 Total == ERROR 计数
  tc=$(total_lines "$out"); ec=$(level_count "$out" ERROR)
  if [ -n "$tc" ] && [ "$tc" = "$ec" ] && [ "$tc" = "3" ]; then
    record 0 "contains 过滤 Total($tc) == ERROR($ec) == 3"
  else
    record 1 "contains 过滤 Total($tc) == ERROR($ec) == 3"
    printf '    实际 Total=%s ERROR=%s\n' "$tc" "$ec"
  fi

  echo "==> 3. 级别过滤（--level ERROR）"
  out="$("$BIN" --file "$SAMPLE" --level ERROR)"
  assert_contains "$out" "ERROR:   4" "级别过滤 ERROR 为 4"
  # 不变式：级别过滤后 Total == 该级别计数
  tc=$(total_lines "$out"); ec=$(level_count "$out" ERROR)
  if [ -n "$tc" ] && [ "$tc" = "$ec" ] && [ "$tc" = "4" ]; then
    record 0 "级别过滤 Total($tc) == ERROR($ec) == 4"
  else
    record 1 "级别过滤 Total($tc) == ERROR($ec) == 4"
  fi
  wc=$(level_count "$out" WARN); ic=$(level_count "$out" INFO)
  if [ "$wc" = "0" ] && [ "$ic" = "0" ]; then
    record 0 "级别过滤后 WARN($wc)/INFO($ic) 计数为 0"
  else
    record 1 "级别过滤后 WARN/INFO 计数为 0"
    printf '    实际 WARN=%s INFO=%s\n' "$wc" "$ic"
  fi

  echo "==> 4. 时间区间过滤（/tmp/selfcheck_time.log）"
  out="$("$BIN" --file "$TIME_LOG" --from 2026-08-10 --to 2026-08-20)"
  assert_contains "$out" "Total lines: 1" "区间 [08-10,08-20] 命中 1 行"
  out="$("$BIN" --file "$TIME_LOG" --from 2026-08-01 --to 2026-08-31)"
  assert_contains "$out" "Total lines: 3" "区间 [08-01,08-31] 命中 3 行"

  echo "==> 5. JSON 输出（--format json）"
  out="$("$BIN" --file "$SAMPLE" --format json)"
  if json_ok "$out"; then
    record 0 "JSON 合法"
  else
    record 1 "JSON 合法"
    printf '    输出:\n%s\n' "$out"
  fi
  assert_contains "$out" '"total_lines":17' "JSON total_lines=17"
  assert_contains "$out" '"level":"ERROR"' "JSON 含 ERROR 级别"
  assert_contains "$out" '"count":4' "JSON ERROR count=4"

  echo "==> 6. Top-N（--top 2）"
  out="$("$BIN" --file "$SAMPLE" --top 2)"
  assert_contains "$out" "Top 2 messages:" "含 Top 2 messages 标题"

  echo "==> 7. 错误处理（退出码）"
  "$BIN" --file "$SAMPLE" --format xml >/dev/null 2>&1
  assert_exit "$?" 1 "--format xml 退出码为 1"
  "$BIN" --file "${TMPDIR}/does_not_exist_xyz.log" >/dev/null 2>&1
  assert_exit "$?" 1 "不存在的文件退出码为 1"

  echo ""
  if [ "$FAIL" = "0" ]; then
    echo "ALL CHECKS PASSED (${PASS}/${PASS})"
    exit 0
  else
    echo "SOME CHECKS FAILED (${PASS}/$((PASS + FAIL)))"
    exit 1
  fi
}

main "$@"
