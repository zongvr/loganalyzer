package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var levelOrder = []string{"ERROR", "WARN", "INFO", "DEBUG", "UNKNOWN"}

type Stats struct {
	Total  int            // 总行数
	Empty  int            // 空行数（strings.TrimSpace 后为空）
	Levels map[string]int // 各级别行数
}

// classifyLevel 按前 3 个字段判定日志级别，优先级 ERROR > WARN > INFO > DEBUG。
// 以连续空白为分隔逐字段扫描，取到 3 个字段后立即停止，避免像 strings.Fields
// 那样为整行分配完整字段切片（超长多字段行会导致内存退化）。
// 空白判定通过 utf8.DecodeRuneInString 正确解码后交给 unicode.IsSpace，
// 与 strings.Fields 语义完全等价——不能按字节强转 rune(line[i])，
// 否则多字节 UTF-8 的续字节（0x80~0xBF，含 NEL=0x85、NBSP=0xA0）会被误判为空白。
func classifyLevel(line string) string {
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

// firstToken 提取行首第一个空白分隔的 token（rune 感知扫描，口径与 classifyLevel 一致）。
func firstToken(line string) string {
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

// timeLayouts 为兼容的日志时间戳布局，按序尝试，命中第一个即可。
var timeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02",
}

// parseTime 用多布局解析时间字符串，返回是否成功。
func parseTime(s string) (time.Time, bool) {
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// lineTime 提取行首 token 并解析为时间；无法解析时返回 ok=false。
func lineTime(line string) (time.Time, bool) {
	return parseTime(firstToken(line))
}

// 不能用默认 bufio.Scanner：其单行上限为 bufio.MaxScanTokenSize（64KB），
// 日志中的长 JSON 或异常堆栈会触发 "token too long" 错误。
// 改用 bufio.Reader 逐行读取，天然无长度上限且保持流式（内存不随文件总大小增长）。
// from / to 为 nil 表示对应时间过滤未启用；level 为空串表示级别过滤未启用。
func analyze(path, contains, level string, from, to *time.Time) (Stats, error) {
	f, err := os.Open(path)
	if err != nil {
		return Stats{}, err
	}
	defer f.Close()

	stats := Stats{Levels: make(map[string]int, len(levelOrder))}
	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			// 用统一的 keep 标志叠加各过滤条件（AND），而非散落的 continue，
			// 确保循环末尾的 readErr / EOF 检查照常执行（末行无换行不漏统计）。
			lv := classifyLevel(line)
			keep := true
			if contains != "" && !strings.Contains(line, contains) {
				keep = false
			}
			if keep && level != "" && !strings.EqualFold(lv, level) {
				keep = false
			}
			if keep && (from != nil || to != nil) {
				ts, ok := lineTime(line)
				if !ok {
					keep = false
				} else if from != nil && ts.Before(*from) {
					keep = false
				} else if to != nil && ts.After(*to) {
					keep = false
				}
			}
			if keep {
				stats.Total++
				if strings.TrimSpace(line) == "" {
					stats.Empty++
				}
				stats.Levels[lv]++
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return Stats{}, readErr
		}
	}
	return stats, nil
}

func main() {
	file := flag.String("file", "", "日志文件路径（必填）")
	contains := flag.String("contains", "", "关键字过滤：仅统计包含该子串的行（可选）")
	level := flag.String("level", "", "级别过滤：仅统计该级别的行，大小写不敏感（可选）")
	fromStr := flag.String("from", "", "时间过滤：仅统计时间戳 ≥ 该值的行（可选）")
	toStr := flag.String("to", "", "时间过滤：仅统计时间戳 ≤ 该值的行（可选）")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "错误：缺少必填参数 --file <path>，请指定日志文件路径。")
		os.Exit(1)
	}

	var from, to *time.Time
	if *fromStr != "" {
		t, ok := parseTime(*fromStr)
		if !ok {
			fmt.Fprintf(os.Stderr, "错误：无法解析 --from 时间值：%s\n", *fromStr)
			os.Exit(1)
		}
		from = &t
	}
	if *toStr != "" {
		t, ok := parseTime(*toStr)
		if !ok {
			fmt.Fprintf(os.Stderr, "错误：无法解析 --to 时间值：%s\n", *toStr)
			os.Exit(1)
		}
		to = &t
	}

	stats, err := analyze(*file, *contains, *level, from, to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：读取文件失败：%v\n", err)
		os.Exit(1)
	}

	var filterParts []string
	if *contains != "" {
		filterParts = append(filterParts, fmt.Sprintf("contains=%q", *contains))
	}
	if *level != "" {
		filterParts = append(filterParts, fmt.Sprintf("level=%q", *level))
	}
	if *fromStr != "" {
		filterParts = append(filterParts, fmt.Sprintf("from=%q", *fromStr))
	}
	if *toStr != "" {
		filterParts = append(filterParts, fmt.Sprintf("to=%q", *toStr))
	}
	if len(filterParts) > 0 {
		fmt.Printf("Filter: %s\n", strings.Join(filterParts, " "))
	}
	fmt.Printf("Total lines: %d\n", stats.Total)
	fmt.Printf("Empty lines: %d\n", stats.Empty)
	fmt.Println()
	fmt.Println("Level statistics:")
	width := 0
	for _, lv := range levelOrder {
		if n := len(lv) + 1; n > width {
			width = n
		}
	}
	for _, lv := range levelOrder {
		fmt.Printf("  %-*s %d\n", width, lv+":", stats.Levels[lv])
	}
}
