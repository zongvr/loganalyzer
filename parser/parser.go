// Package parser 提供日志行的解析与聚类纯函数（无副作用、无 IO）。
package parser

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// TimeLayouts 为兼容的日志时间戳布局，按序尝试，命中第一个即可。
var TimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02",
}

// ClassifyLevel 按前 3 个字段判定日志级别，优先级 ERROR > WARN > INFO > DEBUG。
// 以连续空白为分隔逐字段扫描，取到 3 个字段后立即停止，避免像 strings.Fields
// 那样为整行分配完整字段切片（超长多字段行会导致内存退化）。
// 空白判定通过 utf8.DecodeRuneInString 正确解码后交给 unicode.IsSpace，
// 与 strings.Fields 语义完全等价——不能按字节强转 rune(line[i])，
// 否则多字节 UTF-8 的续字节（0x80~0xBF，含 NEL=0x85、NBSP=0xA0）会被误判为空白。
func ClassifyLevel(line string) string {
	levelOrder := []string{"ERROR", "WARN", "INFO", "DEBUG", "UNKNOWN"}
	// 收集前 3 个字段（最多）。
	fields := make([]string, 0, 3)
	i := 0
	for len(fields) < 3 && i < len(line) {
		// 跳过分隔空白（正确解码 rune 后判定）。
		for i < len(line) {
			r, size := utf8.DecodeRuneInString(line[i:])
			if !unicode.IsSpace(r) {
				break
			}
			i += size
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) {
			r, size := utf8.DecodeRuneInString(line[i:])
			if unicode.IsSpace(r) {
				break
			}
			i += size
		}
		fields = append(fields, line[start:i])
	}
	// 按级别优先级匹配，外层遍历级别、内层遍历字段。
	for _, lv := range levelOrder[:4] {
		for _, f := range fields {
			if strings.ToUpper(f) == lv {
				return lv
			}
		}
	}
	return "UNKNOWN"
}

// FirstToken 提取行首第一个空白分隔的 token（rune 感知扫描，口径与 ClassifyLevel 一致）。
func FirstToken(line string) string {
	i := 0
	for i < len(line) {
		r, size := utf8.DecodeRuneInString(line[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	start := i
	for i < len(line) {
		r, size := utf8.DecodeRuneInString(line[i:])
		if unicode.IsSpace(r) {
			break
		}
		i += size
	}
	return line[start:i]
}

// ParseTime 用多布局解析时间字符串，返回是否成功。
func ParseTime(s string) (time.Time, bool) {
	for _, l := range TimeLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// LineTime 提取行首 token 并解析为时间；无法解析时返回 ok=false。
func LineTime(line string) (time.Time, bool) {
	return ParseTime(FirstToken(line))
}

// MessageKey 去掉行首时间戳前缀，得到用于高频聚合的消息 key。
// 规则：1) 前两个 token 拼成「日期+时间」可解析则去掉；2) 否则第一个 token
// 按日期/ISO 时间可解析则去掉；3) 否则整行（去除首尾空白）即 key。
// 去掉后若为空则退回整行（去除首尾空白），保证非空行必有唯一 key。
func MessageKey(line string) string {
	type span struct{ start, end int }
	spans := make([]span, 0, 2)
	i := 0
	for len(spans) < 2 && i < len(line) {
		for i < len(line) {
			r, size := utf8.DecodeRuneInString(line[i:])
			if !unicode.IsSpace(r) {
				break
			}
			i += size
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) {
			r, size := utf8.DecodeRuneInString(line[i:])
			if unicode.IsSpace(r) {
				break
			}
			i += size
		}
		spans = append(spans, span{start, i})
	}
	trimmed := strings.TrimSpace(line)
	if len(spans) == 0 {
		return trimmed
	}
	// 1) 前两个 token 用空格拼接成「日期+时间」。
	if len(spans) >= 2 {
		joined := line[spans[0].start:spans[0].end] + " " + line[spans[1].start:spans[1].end]
		if _, ok := ParseTime(joined); ok {
			if rest := strings.TrimSpace(line[spans[1].end:]); rest != "" {
				return rest
			}
			return trimmed
		}
	}
	// 2) 第一个 token 为日期或 ISO 时间。
	if _, ok := ParseTime(line[spans[0].start:spans[0].end]); ok {
		if rest := strings.TrimSpace(line[spans[0].end:]); rest != "" {
			return rest
		}
		return trimmed
	}
	// 3) 无可识别时间戳。
	return trimmed
}
