package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
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

// msgCount 表示一条去时间戳后的消息及其出现次数。
type msgCount struct {
	Message string
	Count   int
}

// messageKey 去掉行首时间戳前缀，得到用于高频聚合的消息 key。
// 规则：1) 前两个 token 拼成「日期+时间」可解析则去掉；2) 否则第一个 token
// 按日期/ISO 时间可解析则去掉；3) 否则整行（去除首尾空白）即 key。
// 去掉后若为空则退回整行（去除首尾空白），保证非空行必有唯一 key。
func messageKey(line string) string {
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
		if _, ok := parseTime(joined); ok {
			if rest := strings.TrimSpace(line[spans[1].end:]); rest != "" {
				return rest
			}
			return trimmed
		}
	}
	// 2) 第一个 token 为日期或 ISO 时间。
	if _, ok := parseTime(line[spans[0].start:spans[0].end]); ok {
		if rest := strings.TrimSpace(line[spans[0].end:]); rest != "" {
			return rest
		}
		return trimmed
	}
	// 3) 无可识别时间戳。
	return trimmed
}

// 不能用默认 bufio.Scanner：其单行上限为 bufio.MaxScanTokenSize（64KB），
// 日志中的长 JSON 或异常堆栈会触发 "token too long" 错误。
// 改用 bufio.Reader 逐行读取，天然无长度上限且保持流式（内存不随文件总大小增长）。
// from / to 为 nil 表示对应时间过滤未启用；level 为空串表示级别过滤未启用。
// topN >= 1 时聚合高频消息榜单并返回排序截取后的结果；topN <= 0 时不做聚合（返回 nil）。
func analyze(path, contains, level string, from, to *time.Time, topN int) (Stats, []msgCount, error) {
	f, err := os.Open(path)
	if err != nil {
		return Stats{}, nil, err
	}
	defer f.Close()

	stats := Stats{Levels: make(map[string]int, len(levelOrder))}
	var msgList []msgCount
	var msgIndex map[string]int
	if topN >= 1 {
		msgIndex = make(map[string]int)
	}
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
				empty := strings.TrimSpace(line) == ""
				if empty {
					stats.Empty++
				}
				stats.Levels[lv]++
				// 空行不计入 Top-N 榜单。
				if topN >= 1 && !empty {
					key := messageKey(line)
					if idx, ok := msgIndex[key]; ok {
						msgList[idx].Count++
					} else {
						msgIndex[key] = len(msgList)
						msgList = append(msgList, msgCount{Message: key, Count: 1})
					}
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return Stats{}, nil, readErr
		}
	}
	if topN >= 1 {
		// 稳定排序：同频消息保持首次出现序。
		sort.SliceStable(msgList, func(a, b int) bool {
			return msgList[a].Count > msgList[b].Count
		})
		if len(msgList) > topN {
			msgList = msgList[:topN]
		}
	}
	return stats, msgList, nil
}

// output 渲染统计结果。format 仅接受 text / json（main 已校验）。
// filter 为已拼接好的过滤字符串，未启用过滤时为空串。
// topN >= 1 时渲染 Top-N 榜单；topN <= 0 时 text 不打印榜单、json 不出现 top 字段。
func output(stats Stats, format, filter string, top []msgCount, topN int) error {
	switch format {
	case "text":
		if filter != "" {
			fmt.Printf("Filter: %s\n", filter)
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
		if topN >= 1 {
			fmt.Printf("Top %d messages:\n", topN)
			countWidth := 1
			for _, m := range top {
				if n := len(strconv.Itoa(m.Count)); n > countWidth {
					countWidth = n
				}
			}
			for _, m := range top {
				fmt.Printf("  %*d  %s\n", countWidth, m.Count, m.Message)
			}
		}
		return nil
	case "json":
		type levelCount struct {
			Level string `json:"level"`
			Count int    `json:"count"`
		}
		levels := make([]levelCount, 0, len(levelOrder))
		for _, lv := range levelOrder {
			levels = append(levels, levelCount{Level: lv, Count: stats.Levels[lv]})
		}
		type topMsg struct {
			Message string `json:"message"`
			Count   int    `json:"count"`
		}
		var topList []topMsg
		for _, m := range top {
			topList = append(topList, topMsg{Message: m.Message, Count: m.Count})
		}
		if topList == nil {
			topList = make([]topMsg, 0)
		}
		type report struct {
			Filter     string       `json:"filter"`
			TotalLines int          `json:"total_lines"`
			EmptyLines int          `json:"empty_lines"`
			Levels     []levelCount `json:"levels"`
			Top        *[]topMsg    `json:"top,omitempty"`
		}
		var topPtr *[]topMsg
		if topN >= 1 {
			topPtr = &topList
		}
		b, err := json.Marshal(report{
			Filter:     filter,
			TotalLines: stats.Total,
			EmptyLines: stats.Empty,
			Levels:     levels,
			Top:        topPtr,
		})
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	default:
		return fmt.Errorf("不支持的输出格式：%s", format)
	}
}

func main() {
	file := flag.String("file", "", "日志文件路径（必填）")
	contains := flag.String("contains", "", "关键字过滤：仅统计包含该子串的行（可选）")
	level := flag.String("level", "", "级别过滤：仅统计该级别的行，大小写不敏感（可选）")
	fromStr := flag.String("from", "", "时间过滤：仅统计时间戳 ≥ 该值的行（可选）")
	toStr := flag.String("to", "", "时间过滤：仅统计时间戳 ≤ 该值的行（可选）")
	format := flag.String("format", "text", "输出格式：text 或 json（可选，默认 text）")
	topN := flag.Int("top", 0, "输出出现频率最高的 N 条消息榜单（N>=1 启用，默认不启用）")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "错误：缺少必填参数 --file <path>，请指定日志文件路径。")
		os.Exit(1)
	}

	formatMode := strings.ToLower(*format)
	if formatMode != "text" && formatMode != "json" {
		fmt.Fprintf(os.Stderr, "错误：不支持的 --format 值：%s（仅支持 text / json）\n", *format)
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

	stats, top, err := analyze(*file, *contains, *level, from, to, *topN)
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
	filter := strings.Join(filterParts, " ")

	if err := output(stats, formatMode, filter, top, *topN); err != nil {
		fmt.Fprintf(os.Stderr, "错误：输出失败：%v\n", err)
		os.Exit(1)
	}
}
