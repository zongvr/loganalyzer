package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
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

// 不能用默认 bufio.Scanner：其单行上限为 bufio.MaxScanTokenSize（64KB），
// 日志中的长 JSON 或异常堆栈会触发 "token too long" 错误。
// 改用 bufio.Reader 逐行读取，天然无长度上限且保持流式（内存不随文件总大小增长）。
func analyze(path string, contains string) (Stats, error) {
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
			// 过滤判断须与 readErr 检查分离（用 if/else 而非 continue），
			// 确保末尾无换行的最后一行不会被漏统计。
			if contains != "" && !strings.Contains(line, contains) {
				// 不匹配：跳过本行统计。
			} else {
				stats.Total++
				if strings.TrimSpace(line) == "" {
					stats.Empty++
				}
				stats.Levels[classifyLevel(line)]++
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
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "错误：缺少必填参数 --file <path>，请指定日志文件路径。")
		os.Exit(1)
	}

	stats, err := analyze(*file, *contains)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：读取文件失败：%v\n", err)
		os.Exit(1)
	}

	if *contains != "" {
		fmt.Printf("Filter: contains=%q\n", *contains)
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
