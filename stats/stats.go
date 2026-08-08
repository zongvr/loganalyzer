// Package stats 提供流式统计与 Top-N 高频消息聚合。
package stats

import (
	"bufio"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"loganalyzer/parser"
)

// LevelOrder 为级别统计的固定输出顺序。
var LevelOrder = []string{"ERROR", "WARN", "INFO", "DEBUG", "UNKNOWN"}

// Stats 为一次日志分析的统计结果。
type Stats struct {
	Total  int            // 总行数
	Empty  int            // 空行数（strings.TrimSpace 后为空）
	Levels map[string]int // 各级别行数
}

// MsgCount 表示一条去时间戳后的消息及其出现次数。
type MsgCount struct {
	Message string
	Count   int
}

// 不能用默认 bufio.Scanner：其单行上限为 bufio.MaxScanTokenSize（64KB），
// 日志中的长 JSON 或异常堆栈会触发 "token too long" 错误。
// 改用 bufio.Reader 逐行读取，天然无长度上限且保持流式（内存不随文件总大小增长）。
// from / to 为 nil 表示对应时间过滤未启用；level 为空串表示级别过滤未启用。
// topN >= 1 时聚合高频消息榜单并返回排序截取后的结果；topN <= 0 时不做聚合（返回 nil）。
func Analyze(path, contains, level string, from, to *time.Time, topN int) (Stats, []MsgCount, error) {
	f, err := os.Open(path)
	if err != nil {
		return Stats{}, nil, err
	}
	defer f.Close()

	stats := Stats{Levels: make(map[string]int, len(LevelOrder))}
	var msgList []MsgCount
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
			lv := parser.ClassifyLevel(line)
			keep := true
			if contains != "" && !strings.Contains(line, contains) {
				keep = false
			}
			if keep && level != "" && !strings.EqualFold(lv, level) {
				keep = false
			}
			if keep && (from != nil || to != nil) {
				ts, ok := parser.LineTime(line)
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
					key := parser.MessageKey(line)
					if idx, ok := msgIndex[key]; ok {
						msgList[idx].Count++
					} else {
						msgIndex[key] = len(msgList)
						msgList = append(msgList, MsgCount{Message: key, Count: 1})
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
