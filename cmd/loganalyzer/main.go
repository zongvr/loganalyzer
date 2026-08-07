package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	file := flag.String("file", "", "日志文件路径（必填）")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "错误：缺少必填参数 --file <path>，请指定日志文件路径。")
		os.Exit(1)
	}

	total, empty, err := countLines(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：读取文件失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total lines: %d\n", total)
	fmt.Printf("Empty lines: %d\n", empty)
}

// 不能用默认 bufio.Scanner：其单行上限为 bufio.MaxScanTokenSize（64KB），
// 日志中的长 JSON 或异常堆栈会触发 "token too long" 错误。
// 改用 bufio.Reader 逐行读取，天然无长度上限且保持流式（内存不随文件总大小增长）。
func countLines(path string) (total, empty int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			total++
			if strings.TrimSpace(line) == "" {
				empty++
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return 0, 0, readErr
		}
	}
	return total, empty, nil
}
