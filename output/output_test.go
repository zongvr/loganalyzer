package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"loganalyzer/stats"
)

// captureStdout 重定向 os.Stdout 捕获 Render 的输出，不改动其签名。
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

func TestRenderText(t *testing.T) {
	s := stats.Stats{Total: 2, Empty: 0, Levels: map[string]int{
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
	out, err := captureStdout(func() error { return Render(s, "text", "", nil, 0) })
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if out != want {
		t.Errorf("text output mismatch:\ngot:\n%q\nwant:\n%q", out, want)
	}
}

func TestRenderTextFilterLine(t *testing.T) {
	s := stats.Stats{Total: 1, Empty: 0, Levels: map[string]int{
		"ERROR": 1, "WARN": 0, "INFO": 0, "DEBUG": 0, "UNKNOWN": 0,
	}}
	out, err := captureStdout(func() error { return Render(s, "text", `contains="ERROR"`, nil, 0) })
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(out, "Filter: contains=\"ERROR\"\n") {
		t.Errorf("missing Filter line: got %q", out)
	}
}

func TestRenderTextTop(t *testing.T) {
	s := stats.Stats{Total: 6, Empty: 0, Levels: map[string]int{
		"ERROR": 3, "WARN": 2, "INFO": 1, "DEBUG": 0, "UNKNOWN": 0,
	}}
	top := []stats.MsgCount{
		{Message: "ERROR db connection failed", Count: 3},
		{Message: "WARN  cache miss", Count: 2},
	}
	out, err := captureStdout(func() error { return Render(s, "text", "", top, 2) })
	if err != nil {
		t.Fatalf("Render: %v", err)
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

func TestRenderTextTopDisabledNoBlock(t *testing.T) {
	s := stats.Stats{Total: 1, Empty: 0, Levels: map[string]int{
		"ERROR": 1, "WARN": 0, "INFO": 0, "DEBUG": 0, "UNKNOWN": 0,
	}}
	out, _ := captureStdout(func() error { return Render(s, "text", "", nil, 0) })
	if strings.Contains(out, "Top") {
		t.Errorf("topN=0 should not print Top block: %q", out)
	}
}

func TestRenderJSON(t *testing.T) {
	s := stats.Stats{Total: 17, Empty: 1, Levels: map[string]int{
		"ERROR": 4, "WARN": 2, "INFO": 5, "DEBUG": 2, "UNKNOWN": 4,
	}}
	out, err := captureStdout(func() error { return Render(s, "json", "", nil, 0) })
	if err != nil {
		t.Fatalf("Render: %v", err)
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

func TestRenderJSONTop(t *testing.T) {
	s := stats.Stats{Total: 6, Empty: 0, Levels: map[string]int{
		"ERROR": 3, "WARN": 2, "INFO": 1, "DEBUG": 0, "UNKNOWN": 0,
	}}
	top := []stats.MsgCount{
		{Message: "ERROR db connection failed", Count: 3},
		{Message: "WARN  cache miss", Count: 2},
	}
	out, err := captureStdout(func() error { return Render(s, "json", "", top, 2) })
	if err != nil {
		t.Fatalf("Render: %v", err)
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

func TestRenderJSONTopEmptyIsArrayNotNil(t *testing.T) {
	s := stats.Stats{Total: 0, Empty: 0, Levels: map[string]int{
		"ERROR": 0, "WARN": 0, "INFO": 0, "DEBUG": 0, "UNKNOWN": 0,
	}}
	out, err := captureStdout(func() error { return Render(s, "json", "", nil, 3) })
	if err != nil {
		t.Fatalf("Render: %v", err)
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
