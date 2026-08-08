// Package output 负责渲染统计结果（text / json 两种格式）。
package output

import (
	"encoding/json"
	"fmt"
	"strconv"

	"loganalyzer/stats"
)

// Render 渲染统计结果。format 仅接受 text / json（main 已校验）。
// filter 为已拼接好的过滤字符串，未启用过滤时为空串。
// topN >= 1 时渲染 Top-N 榜单；topN <= 0 时 text 不打印榜单、json 不出现 top 字段。
func Render(s stats.Stats, format, filter string, top []stats.MsgCount, topN int) error {
	switch format {
	case "text":
		if filter != "" {
			fmt.Printf("Filter: %s\n", filter)
		}
		fmt.Printf("Total lines: %d\n", s.Total)
		fmt.Printf("Empty lines: %d\n", s.Empty)
		fmt.Println()
		fmt.Println("Level statistics:")
		width := 0
		for _, lv := range stats.LevelOrder {
			if n := len(lv) + 1; n > width {
				width = n
			}
		}
		for _, lv := range stats.LevelOrder {
			fmt.Printf("  %-*s %d\n", width, lv+":", s.Levels[lv])
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
		levels := make([]levelCount, 0, len(stats.LevelOrder))
		for _, lv := range stats.LevelOrder {
			levels = append(levels, levelCount{Level: lv, Count: s.Levels[lv]})
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
			TotalLines: s.Total,
			EmptyLines: s.Empty,
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
