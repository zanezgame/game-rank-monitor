// Package collector 的 App Store 实现：使用 iTunes Search RSS 公开接口，无需 Key。
// genre=6014 为游戏分类；把路径中的 cn 换成 us 即为美国区。
package collector

import (
	"context"
	"encoding/json"
	"fmt"

	"gamerank/internal/models"
)

// AppStoreCollector 采集 App Store 中国区/美国区榜单。
type AppStoreCollector struct{}

func (AppStoreCollector) Platform() string    { return "appstore" }
func (AppStoreCollector) Regions() []string   { return []string{"cn", "us"} }
func (AppStoreCollector) DisplayName() string { return "App Store" }

// Collect 采集 App Store 榜单。
//   - hot+daily：topfreeapplications（热门免费榜）
//   - new+daily：newapplications（新游榜）
//   - period=weekly 时直接返回与 daily 相同的数据；周榜由调度器调用
//     store.BuildWeeklySnapshots 基于 7 天日榜聚合生成，这里不做重复请求。
//
// MVP 范围说明：未使用 topgrossing（畅销榜）/ toppaid（付费榜），
// 因 RSS 公开接口覆盖有限且畅销榜涉及收入估算，保持范围克制。
func (c AppStoreCollector) Collect(ctx context.Context, region, chartType, period string) ([]Entry, error) {
	if !SupportsRegion(c, region) {
		return nil, fmt.Errorf("appstore: 不支持的区域 %q", region)
	}
	var feed string
	switch chartType {
	case "hot":
		feed = "topfreeapplications"
	case "new":
		feed = "newapplications"
	default:
		return nil, fmt.Errorf("appstore: 不支持的榜单类型 %q", chartType)
	}
	url := fmt.Sprintf("https://itunes.apple.com/%s/rss/%s/limit=100/genre=6014/json", region, feed)
	body, err := GetJSON(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("appstore: 获取 %s 区 %s 榜失败: %w", region, feed, err)
	}
	return parseRSSFeed(body, region)
}

// ---- RSS JSON 解析 ----

// rssFeed 是 iTunes RSS 响应的外层结构。
type rssFeed struct {
	Feed struct {
		Entry json.RawMessage `json:"entry"` // 可能是数组，也可能是单个对象
	} `json:"feed"`
}

// rssEntry 是单个榜单条目。
type rssEntry struct {
	Name struct {
		Label string `json:"label"`
	} `json:"im:name"`
	Images []struct {
		Label string `json:"label"`
	} `json:"im:image"`
	Summary struct {
		Label string `json:"label"`
	} `json:"summary"`
	Price struct {
		Label string `json:"label"`
	} `json:"im:price"`
	Artist struct {
		Label string `json:"label"`
	} `json:"im:artist"`
	ID struct {
		Attributes struct {
			AppID string `json:"im:id"`
		} `json:"attributes"`
	} `json:"id"`
}

// parseRSSFeed 解析 iTunes RSS JSON，兼容 entry 为单个对象的边界情况。
func parseRSSFeed(body []byte, region string) ([]Entry, error) {
	var f rssFeed
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("appstore: 解析 RSS 失败: %w", err)
	}
	if len(f.Feed.Entry) == 0 {
		return nil, fmt.Errorf("appstore: RSS 响应中没有 entry 数据")
	}
	var rawList []json.RawMessage
	trimmed := trimSpace(f.Feed.Entry)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(f.Feed.Entry, &rawList); err != nil {
			return nil, fmt.Errorf("appstore: 解析 entry 数组失败: %w", err)
		}
	} else {
		rawList = []json.RawMessage{f.Feed.Entry} // 单个对象时包成数组
	}
	var entries []Entry
	for i, raw := range rawList {
		var e rssEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			continue // 单条解析失败则跳过，不影响整体
		}
		if e.ID.Attributes.AppID == "" {
			continue
		}
		icon := ""
		if len(e.Images) > 0 {
			icon = e.Images[len(e.Images)-1].Label // 取最大尺寸图片
		}
		entries = append(entries, Entry{
			ExternalID: e.ID.Attributes.AppID,
			Name:       e.Name.Label,
			IconURL:    icon,
			Rank:       i + 1,
			PriceText:  e.Price.Label,
			Artist:     e.Artist.Label,
			Summary:    e.Summary.Label,
		})
		_ = region
	}
	return entries, nil
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}

// CollectDiscussions 暂未实现：App Store 无公开评论 API，返回空。
func (AppStoreCollector) CollectDiscussions(_ context.Context, _ models.Game) ([]models.Discussion, error) {
	return nil, nil
}
