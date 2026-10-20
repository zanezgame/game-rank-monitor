// Package analyzer 根据排名变化生成上榜理由。
package analyzer

import (
	"fmt"
	"strings"
	"time"

	"gamerank/internal/models"
)

// Analyze 根据上一次快照与当前快照生成上榜理由码与中文描述。
// events 为附加事件（如 "Steam 促销 -30%"），会追加到描述末尾。
// 排名变化方向：RankChange = 上次排名 - 本次排名（正数=上升）。
func Analyze(prev *models.RankSnapshot, cur models.RankSnapshot, events []string) (code, text string) {
	switch {
	case prev == nil:
		code, text = "new_entry", "首次上榜"
	default:
		// 计算缺席天数
		days := 0
		if pd, err := time.Parse("2006-01-02", prev.Date); err == nil {
			if cd, err := time.Parse("2006-01-02", cur.Date); err == nil {
				days = int(cd.Sub(pd).Hours() / 24)
			}
		}
		switch {
		case days > 7:
			code, text = "re_entry", fmt.Sprintf("重回榜单（缺席 %d 天）", days)
		default:
			change := prev.Rank - cur.Rank // 正数=上升
			switch {
			case change >= 10:
				code, text = "surge", fmt.Sprintf("排名大幅上升 ↑%d", change)
			case change <= -10:
				code, text = "drop", fmt.Sprintf("排名明显下跌 ↓%d", -change)
			default:
				code, text = "stable", "排名稳定"
			}
		}
	}
	if len(events) > 0 {
		text += "｜" + strings.Join(events, "｜")
	}
	return code, text
}

// DiscountEvent 为 Steam 折扣生成事件文本，discount<=0 时返回空字符串。
func DiscountEvent(discount int) string {
	if discount > 0 {
		return fmt.Sprintf("Steam 促销 -%d%%", discount)
	}
	return ""
}
