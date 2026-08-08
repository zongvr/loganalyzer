package stats

import (
	"os"
	"testing"

	"loganalyzer/parser"
)

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

func TestAnalyzeCounts(t *testing.T) {
	path := writeTempLog(t, "2026-08-07 09:00:01 ERROR a\n2026-08-07 09:00:02 WARN b\n\n2026-08-07 09:00:03 INFO c\nlast-no-newline")
	stats, top, err := Analyze(path, "", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
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
	stats, _, err := Analyze(path, "", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
}

func TestAnalyzeContainsFilter(t *testing.T) {
	path := writeTempLog(t, "ERROR db connection failed\nWARN cache miss\nERROR db timeout")
	stats, _, err := Analyze(path, "ERROR", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
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
	stats, _, err := Analyze(path, "", "WARN", nil, nil, 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// EqualFold：warn / WARN 都命中。
	if stats.Total != 2 || stats.Levels["WARN"] != 2 {
		t.Errorf("level filter wrong: Total=%d Levels=%v", stats.Total, stats.Levels)
	}
}

func TestAnalyzeTimeFilter(t *testing.T) {
	path := writeTempLog(t, "2026-08-01 srv1 ERROR 启动\n2026-08-15 srv2 WARN 警告\n2026-09-01 srv3 INFO 正常\nnotime ERROR")
	from, _ := parser.ParseTime("2026-08-01")
	to, _ := parser.ParseTime("2026-08-31")
	stats, _, err := Analyze(path, "", "", &from, &to, 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
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
	_, top, err := Analyze(path, "", "", nil, nil, 3)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
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
	_, top, err := Analyze(path, "", "", nil, nil, 1)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
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
	_, top, err := Analyze(path, "", "", nil, nil, 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if top != nil {
		t.Errorf("topN=0 should return nil, got %v", top)
	}
}
