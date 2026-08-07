package main

import (
	"bufio"
	"flag"
	"fmt"
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

func countLines(path string) (total, empty int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		total++
		if strings.TrimSpace(scanner.Text()) == "" {
			empty++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	return total, empty, nil
}
