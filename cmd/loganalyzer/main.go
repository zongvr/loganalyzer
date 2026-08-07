package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

var levelOrder = []string{"ERROR", "WARN", "INFO", "DEBUG", "UNKNOWN"}

// classifyLevel 按前 3 个字段判定日志级别，优先级 ERROR > WARN > INFO > DEBUG。
func classifyLevel(line string) string {
	fields := strings.Fields(line)
	if len(fields) > 3 {
		fields = fields[:3]
	}
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
func analyze(path string) (Stats, error) {
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
			stats.Total++
			if strings.TrimSpace(line) == "" {
				stats.Empty++
			}
			stats.Levels[classifyLevel(line)]++
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

type Stats struct {
	Total  int            // 总行数
	Empty  int            // 空行数（strings.TrimSpace 后为空）
	Levels map[string]int // 各级别行数
}

func main() {
	file := flag.String("file", "", "日志文件路径（必填）")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "错误：缺少必填参数 --file <path>，请指定日志文件路径。")
		os.Exit(1)
	}

	stats, err := analyze(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：读取文件失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total lines: %d\n", stats.Total)
	fmt.Printf("Empty lines: %d\n", stats.Empty)
	fmt.Println()
	fmt.Println("Level statistics:")
	for _, lv := range levelOrder {
		fmt.Printf("  %-7s %d\n", lv+":", stats.Levels[lv])
	}
}
