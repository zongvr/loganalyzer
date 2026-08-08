package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout 重定向 os.Stdout 捕获 output 的输出，不改动其签名。
func captureStdout(fn func() error) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), err
}

func writeTempLog(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "log-*.log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatalf("write temp log: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp log: %v", err)
	}
	return f.Name()
}

func TestClassifyLevel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2026-08-07 09:00:01 ERROR db connection failed", "ERROR"},
		{"WARN  cache miss", "WARN"},
		{"INFO  ok", "INFO"},
		{"DEBUG trace x", "DEBUG"},
		{"ERROR something WARN", "ERROR"},
		{"WARN then ERROR", "ERROR"},
		{"error lowercase", "ERROR"},
		{"warn lowercase", "WARN"},
		{"info", "INFO"},
		{"debug", "DEBUG"},
		{"信息 启动完成", "UNKNOWN"},
		{"no keyword here", "UNKNOWN"},
		{"", "UNKNOWN"},
		{"  ", "UNKNOWN"},
		{"2026-08-07 堀 ERROR 模块名含字节0xA0(位于中间)", "ERROR"},
		{"2026-08-07 习 WARN 模块名含字节0xA0(位于末尾)", "WARN"},
		{"堀堀堀 堀堀 INFO 前两字段全为特殊汉字", "INFO"},
	}
	for _, tt := range tests {
		if got := classifyLevel(tt.in); got != tt.want {
			t.Errorf("classifyLevel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// 前 3 个字段之外出现 ERROR 不得被命中。
func TestClassifyLevelOnlyFirstThreeFields(t *testing.T) {
	in := "plain plain plain ERROR"
	if got := classifyLevel(in); got != "UNKNOWN" {
		t.Errorf("classifyLevel(%q) = %q, want UNKNOWN", in, got)
	}
}

func TestMessageKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2026-08-07 09:00:01 ERROR db connection failed", "ERROR db connection failed"},
		{"2026-08-07 ERROR foo", "ERROR foo"},
		{"2026-08-07T09:00:01 INFO x", "INFO x"},
		{"no timestamp here", "no timestamp here"},
		{"2026-08-07", "2026-08-07"},
		{"ERROR order 2026-08-07 created", "ERROR order 2026-08-07 created"},
		{"2026-08-07 09:00:03 WARN  cache miss", "WARN  cache miss"},
		{"堀堀堀 堀堀 INFO 前两字段全为特殊汉字", "堀堀堀 堀堀 INFO 前两字段全为特殊汉字"},
		{"2026/08/01 srv1 ERROR 启动", "srv1 ERROR 启动"},
	}
	for _, tt := range tests {
		if got := messageKey(tt.in); got != tt.want {
			t.Errorf("messageKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseTime(t *testing.T) {
	valid := []string{
		"2026-08-07 09:00:01",
		"2026-08-07T09:00:01",
		"2026-08-07",
		"2026/08/01 09:00:01",
		"2026/08/01",
	}
	for _, s := range valid {
		if _, ok := parseTime(s); !ok {
			t.Errorf("parseTime(%q) ok=false, want true", s)
		}
	}
	invalid := []string{"2026-13-99", "hello", "", "2026-08-07 99:99:99"}
	for _, s := range invalid {
		if _, ok := parseTime(s); ok {
			t.Errorf("parseTime(%q) ok=true, want false", s)
		}
	}
}

func TestLineTime(t *testing.T) {
	if _, ok := lineTime("2026-08-07 09:00:01 ERROR"); !ok {
		t.Errorf("lineTime should parse datetime line")
	}
	if _, ok := lineTime("no timestamp"); ok {
		t.Errorf("lineTime should fail on non-timestamp line")
	}
}

func TestFirstToken(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"  hello world", "hello"},
		{"hello", "hello"},
		{"", ""},
		{"   堀   x", "堀"},
	}
	for _, tt := range tests {
		if got := firstToken(tt.in); got != tt.want {
			t.Errorf("firstToken(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAnalyzeCounts(t *testing.T) {
	path := writeTempLog(t, "2026-08-07 09:00:01 ERROR a\n2026-08-07 09:00:02 WARN b\n\n2026-08-07 09:00:03 INFO c\nlast-no-newline")
	stats, top, err := analyze(path, "", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if stats.Total != 5 {
		t.Errorf("Total = %d, want 5", stats.Total)
	}
	if stats.Empty != 1 {
		t.Errorf("Empty = %d, want 1", stats.Empty)
	}
	wantLevels := map[string]int{"ERROR": 1, "WARN": 1, "INFO": 1, "DEBUG": 0, "UNKNOWN": 2}
	for lv, want := range wantLevels {
		if got := stats.Levels[lv]; got != want {
			t.Errorf("Levels[%s] = %d, want %d", lv, got, want)
		}
	}
	if top != nil {
		t.Errorf("topN<=0 should return nil top, got %v", top)
	}
}

func TestAnalyzeLastLineNoNewline(t *testing.T) {
	path := writeTempLog(t, "a\nb\nc")
	stats, _, err := analyze(path, "", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
}

func TestAnalyzeContainsFilter(t *testing.T) {
	path := writeTempLog(t, "ERROR db connection failed\nWARN cache miss\nERROR db timeout")
	stats, _, err := analyze(path, "ERROR", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if stats.Total != 2 {
		t.Errorf("Total = %d, want 2", stats.Total)
	}
	if stats.Levels["ERROR"] != 2 || stats.Levels["WARN"] != 0 {
		t.Errorf("levels wrong: %v", stats.Levels)
	}
}

func TestAnalyzeLevelFilter(t *testing.T) {
	path := writeTempLog(t, "ERROR a\nwarn b\nWARN c\nINFO d")
	stats, _, err := analyze(path, "", "WARN", nil, nil, 0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	// EqualFold：warn / WARN 都命中。
	if stats.Total != 2 || stats.Levels["WARN"] != 2 {
		t.Errorf("level filter wrong: Total=%d Levels=%v", stats.Total, stats.Levels)
	}
}

func TestAnalyzeTimeFilter(t *testing.T) {
	path := writeTempLog(t, "2026-08-01 srv1 ERROR 启动\n2026-08-15 srv2 WARN 警告\n2026-09-01 srv3 INFO 正常\nnotime ERROR")
	from, _ := parseTime("2026-08-01")
	to, _ := parseTime("2026-08-31")
	stats, _, err := analyze(path, "", "", &from, &to, 0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	// 2026-08-01（边界含）与 2026-08-15 在区间内；09-01 超界；notime 被排除。
	if stats.Total != 2 {
		t.Errorf("Total = %d, want 2", stats.Total)
	}
	if stats.Levels["ERROR"] != 1 || stats.Levels["WARN"] != 1 {
		t.Errorf("levels wrong: %v", stats.Levels)
	}
}

func TestAnalyzeTopN(t *testing.T) {
	path := writeTempLog(t, "2026-08-07 a\n2026-08-08 b\n2026-08-09 a\n2026-08-10 c\n2026-08-11 b\n2026-08-12 c")
	_, top, err := analyze(path, "", "", nil, nil, 3)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(top) != 3 {
		t.Fatalf("len(top) = %d, want 3", len(top))
	}
	// 频率降序、同频首次出现序：a,b,c。
	wantMsgs := []string{"a", "b", "c"}
	wantCounts := []int{2, 2, 2}
	for i := range wantMsgs {
		if top[i].Message != wantMsgs[i] || top[i].Count != wantCounts[i] {
			t.Errorf("top[%d] = %+v, want %s/%d", i, top[i], wantMsgs[i], wantCounts[i])
		}
	}
}

func TestAnalyzeTopNTruncate(t *testing.T) {
	path := writeTempLog(t, "2026-08-07 09:00:01 ERROR db connection failed\n2026-08-07 09:00:02 ERROR db connection failed\n2026-08-07 09:00:03 WARN  cache miss")
	_, top, err := analyze(path, "", "", nil, nil, 1)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(top) != 1 {
		t.Fatalf("len(top) = %d, want 1", len(top))
	}
	if top[0].Message != "ERROR db connection failed" || top[0].Count != 2 {
		t.Errorf("top[0] = %+v", top[0])
	}
}

func TestAnalyzeTopNDisabled(t *testing.T) {
	path := writeTempLog(t, "2026-08-07 a\n2026-08-08 a")
	_, top, err := analyze(path, "", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if top != nil {
		t.Errorf("topN=0 should return nil, got %v", top)
	}
}

func TestOutputText(t *testing.T) {
	stats := Stats{Total: 2, Empty: 0, Levels: map[string]int{
		"ERROR": 1, "WARN": 0, "INFO": 1, "DEBUG": 0, "UNKNOWN": 0,
	}}
	want := "" +
		"Total lines: 2\n" +
		"Empty lines: 0\n" +
		"\n" +
		"Level statistics:\n" +
		"  ERROR:   1\n" +
		"  WARN:    0\n" +
		"  INFO:    1\n" +
		"  DEBUG:   0\n" +
		"  UNKNOWN: 0\n"
	out, err := captureStdout(func() error { return output(stats, "text", "", nil, 0) })
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	if out != want {
		t.Errorf("text output mismatch:\ngot:\n%q\nwant:\n%q", out, want)
	}
}

func TestOutputTextFilterLine(t *testing.T) {
	stats := Stats{Total: 1, Empty: 0, Levels: map[string]int{
		"ERROR": 1, "WARN": 0, "INFO": 0, "DEBUG": 0, "UNKNOWN": 0,
	}}
	out, err := captureStdout(func() error { return output(stats, "text", `contains="ERROR"`, nil, 0) })
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	if !strings.HasPrefix(out, "Filter: contains=\"ERROR\"\n") {
		t.Errorf("missing Filter line: got %q", out)
	}
}

func TestOutputTextTop(t *testing.T) {
	stats := Stats{Total: 6, Empty: 0, Levels: map[string]int{
		"ERROR": 3, "WARN": 2, "INFO": 1, "DEBUG": 0, "UNKNOWN": 0,
	}}
	top := []msgCount{
		{Message: "ERROR db connection failed", Count: 3},
		{Message: "WARN  cache miss", Count: 2},
	}
	out, err := captureStdout(func() error { return output(stats, "text", "", top, 2) })
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	if !strings.Contains(out, "Top 2 messages:\n") {
		t.Errorf("missing Top header: %q", out)
	}
	if !strings.Contains(out, "  3  ERROR db connection failed\n") {
		t.Errorf("missing top row 1: %q", out)
	}
	if !strings.Contains(out, "  2  WARN  cache miss\n") {
		t.Errorf("missing top row 2: %q", out)
	}
}

func TestOutputTextTopDisabledNoBlock(t *testing.T) {
	stats := Stats{Total: 1, Empty: 0, Levels: map[string]int{
		"ERROR": 1, "WARN": 0, "INFO": 0, "DEBUG": 0, "UNKNOWN": 0,
	}}
	out, _ := captureStdout(func() error { return output(stats, "text", "", nil, 0) })
	if strings.Contains(out, "Top") {
		t.Errorf("topN=0 should not print Top block: %q", out)
	}
}

func TestOutputJSON(t *testing.T) {
	stats := Stats{Total: 17, Empty: 1, Levels: map[string]int{
		"ERROR": 4, "WARN": 2, "INFO": 5, "DEBUG": 2, "UNKNOWN": 4,
	}}
	out, err := captureStdout(func() error { return output(stats, "json", "", nil, 0) })
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if _, ok := got["top"]; ok {
		t.Errorf("topN=0 must not contain top field: %v", got)
	}
	if got["total_lines"].(float64) != 17 || got["empty_lines"].(float64) != 1 {
		t.Errorf("json fields wrong: %v", got)
	}
	if got["filter"] != "" {
		t.Errorf("filter should be empty: %v", got)
	}
	levels := got["levels"].([]any)
	if len(levels) != 5 {
		t.Errorf("levels len = %d, want 5", len(levels))
	}
	// 级别顺序固定。
	order := []string{"ERROR", "WARN", "INFO", "DEBUG", "UNKNOWN"}
	for i, lv := range order {
		el := levels[i].(map[string]any)
		if el["level"] != lv {
			t.Errorf("levels[%d].level = %v, want %s", i, el["level"], lv)
		}
	}
}

func TestOutputJSONTop(t *testing.T) {
	stats := Stats{Total: 6, Empty: 0, Levels: map[string]int{
		"ERROR": 3, "WARN": 2, "INFO": 1, "DEBUG": 0, "UNKNOWN": 0,
	}}
	top := []msgCount{
		{Message: "ERROR db connection failed", Count: 3},
		{Message: "WARN  cache miss", Count: 2},
	}
	out, err := captureStdout(func() error { return output(stats, "json", "", top, 2) })
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	topArr, ok := got["top"].([]any)
	if !ok {
		t.Fatalf("top field missing/not array: %v", got)
	}
	if len(topArr) != 2 {
		t.Fatalf("top len = %d, want 2", len(topArr))
	}
	el0 := topArr[0].(map[string]any)
	if el0["message"] != "ERROR db connection failed" || el0["count"].(float64) != 3 {
		t.Errorf("top[0] = %v", el0)
	}
}

func TestOutputJSONTopEmptyIsArrayNotNil(t *testing.T) {
	stats := Stats{Total: 0, Empty: 0, Levels: map[string]int{
		"ERROR": 0, "WARN": 0, "INFO": 0, "DEBUG": 0, "UNKNOWN": 0,
	}}
	out, err := captureStdout(func() error { return output(stats, "json", "", nil, 3) })
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	top, ok := got["top"].([]any)
	if !ok {
		t.Fatalf("top should be present and [] when topN>=1 empty, got %v", got["top"])
	}
	if len(top) != 0 {
		t.Errorf("top should be empty array, got %v", top)
	}
}
