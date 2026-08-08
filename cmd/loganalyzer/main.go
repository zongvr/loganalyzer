package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"loganalyzer/output"
	"loganalyzer/parser"
	"loganalyzer/stats"
)

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
		t, ok := parser.ParseTime(*fromStr)
		if !ok {
			fmt.Fprintf(os.Stderr, "错误：无法解析 --from 时间值：%s\n", *fromStr)
			os.Exit(1)
		}
		from = &t
	}
	if *toStr != "" {
		t, ok := parser.ParseTime(*toStr)
		if !ok {
			fmt.Fprintf(os.Stderr, "错误：无法解析 --to 时间值：%s\n", *toStr)
			os.Exit(1)
		}
		to = &t
	}

	st, top, err := stats.Analyze(*file, *contains, *level, from, to, *topN)
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

	if err := output.Render(st, formatMode, filter, top, *topN); err != nil {
		fmt.Fprintf(os.Stderr, "错误：输出失败：%v\n", err)
		os.Exit(1)
	}
}
