package parser

import "testing"

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
		if got := ClassifyLevel(tt.in); got != tt.want {
			t.Errorf("ClassifyLevel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// 前 3 个字段之外出现 ERROR 不得被命中。
func TestClassifyLevelOnlyFirstThreeFields(t *testing.T) {
	in := "plain plain plain ERROR"
	if got := ClassifyLevel(in); got != "UNKNOWN" {
		t.Errorf("ClassifyLevel(%q) = %q, want UNKNOWN", in, got)
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
		if got := MessageKey(tt.in); got != tt.want {
			t.Errorf("MessageKey(%q) = %q, want %q", tt.in, got, tt.want)
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
		if _, ok := ParseTime(s); !ok {
			t.Errorf("ParseTime(%q) ok=false, want true", s)
		}
	}
	invalid := []string{"2026-13-99", "hello", "", "2026-08-07 99:99:99"}
	for _, s := range invalid {
		if _, ok := ParseTime(s); ok {
			t.Errorf("ParseTime(%q) ok=true, want false", s)
		}
	}
}

func TestLineTime(t *testing.T) {
	if _, ok := LineTime("2026-08-07 09:00:01 ERROR"); !ok {
		t.Errorf("LineTime should parse datetime line")
	}
	if _, ok := LineTime("no timestamp"); ok {
		t.Errorf("LineTime should fail on non-timestamp line")
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
		if got := FirstToken(tt.in); got != tt.want {
			t.Errorf("FirstToken(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
