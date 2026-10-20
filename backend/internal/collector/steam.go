// Package collector 的 Steam 实现：全部使用公开接口，无需 API Key。
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"gamerank/internal/models"
)

// SteamCollector 采集 Steam 全球榜。
type SteamCollector struct{}

func (SteamCollector) Platform() string    { return "steam" }
func (SteamCollector) Regions() []string   { return []string{"global"} }
func (SteamCollector) DisplayName() string { return "Steam" }

// Collect 采集 Steam 榜单（region 固定为 global，忽略传入值）。
//   - hot+daily：ISteamChartsService/GetMostPlayedGames（最热游玩榜）
//   - hot+weekly：featuredcategories top_sellers（热销榜）
//   - new+daily / new+weekly：featuredcategories new_releases（新品榜）
//
// 注意：Steam 官方接口是 ISteamChartsService，ISteamCharts 会返回 404。
func (c SteamCollector) Collect(ctx context.Context, region, chartType, period string) ([]Entry, error) {
	switch chartType {
	case "hot":
		if period == "daily" {
			return c.collectMostPlayed(ctx)
		}
		return c.collectFeatured(ctx, "top_sellers")
	case "new":
		return c.collectFeatured(ctx, "new_releases")
	default:
		return nil, fmt.Errorf("steam: 不支持的榜单类型 %q", chartType)
	}
}

// ---- 最热游玩榜 ----

type mostPlayedResp struct {
	Response struct {
		RollupDate int `json:"rollup_date"`
		Ranks      []struct {
			Rank         int `json:"rank"`
			AppID        int `json:"appid"`
			LastWeekRank int `json:"last_week_rank"`
			PeakInGame   int `json:"peak_in_game"`
		} `json:"ranks"`
	} `json:"response"`
}

// collectMostPlayed 采集 Steam 最热游玩榜（全球日榜）。
func (c SteamCollector) collectMostPlayed(ctx context.Context) ([]Entry, error) {
	body, err := GetJSON(ctx, "https://api.steampowered.com/ISteamChartsService/GetMostPlayedGames/v1/")
	if err != nil {
		return nil, fmt.Errorf("steam: 获取最热游玩榜失败: %w", err)
	}
	var resp mostPlayedResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("steam: 解析最热游玩榜失败: %w", err)
	}
	const maxRank = 100
	var entries []Entry
	for i, r := range resp.Response.Ranks {
		if i >= maxRank {
			break
		}
		e := Entry{
			ExternalID: fmt.Sprint(r.AppID),
			Rank:       r.Rank,
		}
		if r.LastWeekRank > 0 {
			d := r.LastWeekRank - r.Rank // 正数=上升
			e.RankChange = &d
		}
		entries = append(entries, e)
	}
	// 前 40 名用 appdetails 补全名称/图标/价格（逐个请求，间隔 ≥200ms，失败跳过），
	// 随后对 40 名外仍无名称的条目再补一遍，保证整张榜都有真实名称
	c.enrichAppDetails(ctx, entries, 40)
	// 前 20 名补当前在线人数
	c.enrichPlayerCounts(ctx, entries, 20)
	return entries, nil
}

// appDetailsResp 是 store.steampowered.com/api/appdetails 的响应结构。
type appDetailsResp map[string]struct {
	Success bool `json:"success"`
	Data    struct {
		Name       string `json:"name"`
		HeaderImage string `json:"header_image"`
		IsFree     bool   `json:"is_free"`
		PriceOverview *struct {
			Currency        string `json:"currency"`
			Initial         int    `json:"initial"`
			Final           int    `json:"final"`
			DiscountPercent int    `json:"discount_percent"`
		} `json:"price_overview"`
	} `json:"data"`
}

// enrichAppDetails 为前 n 名补充商店详情；之后再对仍无名称的条目逐个补一遍，
// 保证整张榜（共 100 名）都有真实名称，而不是只覆盖前 n 名。
func (c SteamCollector) enrichAppDetails(ctx context.Context, entries []Entry, n int) {
	if n > len(entries) {
		n = len(entries)
	}
	first := true
	fetch := func(e *Entry) {
		if !first {
			time.Sleep(200 * time.Millisecond) // 间隔 ≥200ms，避免被限流
		}
		first = false
		c.fetchAppDetails(ctx, e)
	}
	for i := 0; i < n; i++ {
		fetch(&entries[i])
	}
	// 第二遍：补齐 n 名之外仍无名称的条目（如排在第 97 名的《霍格沃茨之遗》）
	for i := n; i < len(entries); i++ {
		if entries[i].Name != "" {
			continue
		}
		fetch(&entries[i])
	}
}

// fetchAppDetails 拉取单个游戏的商店详情并写入 e。
// 先用国区 + 简体中文；某些 appid 在国区返回 success:false（如 AION 2），
// 此时回退到美区 + 英文，至少拿到真实名称与图标。
func (c SteamCollector) fetchAppDetails(ctx context.Context, e *Entry) {
	if c.tryAppDetails(ctx, e, "CN", "schinese") {
		return
	}
	c.tryAppDetails(ctx, e, "US", "english")
}

// tryAppDetails 用指定地区/语言请求 appdetails，成功写入 e 并返回 true。
func (c SteamCollector) tryAppDetails(ctx context.Context, e *Entry, cc, lang string) bool {
	url := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&cc=%s&l=%s", e.ExternalID, cc, lang)
	body, err := GetJSON(ctx, url)
	if err != nil {
		log.Printf("steam: appdetails %s (%s/%s) 失败: %v", e.ExternalID, cc, lang, err)
		return false
	}
	var resp appDetailsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	d, ok := resp[e.ExternalID]
	if !ok || !d.Success || d.Data.Name == "" {
		return false
	}
	e.Name = d.Data.Name
	e.IconURL = d.Data.HeaderImage
	switch {
	case d.Data.IsFree:
		e.PriceText = "免费"
	case d.Data.PriceOverview != nil:
		po := d.Data.PriceOverview
		e.PriceText = formatSteamPrice(po.Currency, po.Final)
		e.Discount = po.DiscountPercent
	}
	return true
}

// formatSteamPrice 按币种格式化价格（接口返回的金额单位为分）。
func formatSteamPrice(currency string, cents int) string {
	switch currency {
	case "CNY":
		return fmt.Sprintf("¥%.2f", float64(cents)/100)
	case "USD":
		return fmt.Sprintf("$%.2f", float64(cents)/100)
	default:
		return fmt.Sprintf("%s %.2f", currency, float64(cents)/100)
	}
}

// currentPlayersResp 是 GetNumberOfCurrentPlayers 的响应结构。
type currentPlayersResp struct {
	Response struct {
		PlayerCount int `json:"player_count"`
		Result      int `json:"result"`
	} `json:"response"`
}

// enrichPlayerCounts 为前 n 名补充当前在线人数。
func (c SteamCollector) enrichPlayerCounts(ctx context.Context, entries []Entry, n int) {
	if n > len(entries) {
		n = len(entries)
	}
	for i := 0; i < n; i++ {
		e := &entries[i]
		url := fmt.Sprintf("https://api.steampowered.com/ISteamUserStats/GetNumberOfCurrentPlayers/v0001/?appid=%s", e.ExternalID)
		body, err := GetJSON(ctx, url)
		if err != nil {
			log.Printf("steam: 在线人数 %s 失败，跳过: %v", e.ExternalID, err)
			continue
		}
		var resp currentPlayersResp
		if err := json.Unmarshal(body, &resp); err != nil || resp.Response.Result != 1 {
			continue
		}
		pc := int64(resp.Response.PlayerCount)
		e.PlayerCount = &pc
	}
}

// ---- 热销 / 新品榜 ----

type featuredResp struct {
	TopSellers  *featuredCategory `json:"top_sellers"`
	NewReleases *featuredCategory `json:"new_releases"`
}

type featuredCategory struct {
	Items []struct {
		ID              int    `json:"id"`
		Type            int    `json:"type"` // 0=游戏，其余为 DLC/捆绑包等
		Name            string `json:"name"`
		Discounted      bool   `json:"discounted"`
		DiscountPercent int    `json:"discount_percent"`
		FinalPrice      int    `json:"final_price"` // 单位：分
		Currency        string `json:"currency"`
		LargeCapsule    string `json:"large_capsule_image"`
	} `json:"items"`
}

// collectFeatured 采集热销榜（top_sellers）或新品榜（new_releases）。
func (c SteamCollector) collectFeatured(ctx context.Context, category string) ([]Entry, error) {
	body, err := GetJSON(ctx, "https://store.steampowered.com/api/featuredcategories/?cc=CN")
	if err != nil {
		return nil, fmt.Errorf("steam: 获取精选分类失败: %w", err)
	}
	var resp featuredResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("steam: 解析精选分类失败: %w", err)
	}
	var cat *featuredCategory
	switch category {
	case "top_sellers":
		cat = resp.TopSellers
	case "new_releases":
		cat = resp.NewReleases
	}
	if cat == nil {
		return nil, fmt.Errorf("steam: 分类 %q 数据缺失", category)
	}
	var entries []Entry
	rank := 0
	for _, it := range cat.Items {
		if it.Type != 0 {
			continue // 只保留游戏，过滤 DLC / 捆绑包
		}
		rank++
		e := Entry{
			ExternalID: fmt.Sprint(it.ID),
			Name:       it.Name,
			IconURL:    it.LargeCapsule,
			Rank:       rank,
			Discount:   it.DiscountPercent,
		}
		if it.Discounted || it.FinalPrice == 0 {
			if it.FinalPrice == 0 {
				e.PriceText = "免费"
			} else {
				e.PriceText = fmt.Sprintf("¥%.2f", float64(it.FinalPrice)/100)
			}
		} else {
			e.PriceText = fmt.Sprintf("¥%.2f", float64(it.FinalPrice)/100)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ---- 玩家讨论 ----

type appReviewsResp struct {
	Success int `json:"success"`
	Reviews []struct {
		RecommendationID string `json:"recommendationid"`
		Author           struct {
			SteamID string `json:"steamid"`
		} `json:"author"`
		Language         string `json:"language"`
		Review           string `json:"review"`
		TimestampCreated int64  `json:"timestamp_created"`
		VotedUp          bool   `json:"voted_up"`
	} `json:"reviews"`
}

// CollectDiscussions 采集某 Steam 游戏的最新商店评论。
// 先尝试中文评论，为空则回退英文。
func (c SteamCollector) CollectDiscussions(ctx context.Context, g models.Game) ([]models.Discussion, error) {
	reviews, err := c.fetchReviews(ctx, g.ExternalID, "schinese")
	if err != nil {
		return nil, err
	}
	if len(reviews) == 0 {
		reviews, err = c.fetchReviews(ctx, g.ExternalID, "english")
		if err != nil {
			return nil, err
		}
	}
	var out []models.Discussion
	for _, r := range reviews {
		content := strings.TrimSpace(r.Review)
		if content == "" {
			continue
		}
		if utf8.RuneCountInString(content) > 2000 {
			content = string([]rune(content)[:2000])
		}
		sentiment := "negative"
		if r.VotedUp {
			sentiment = "positive"
		}
		author := "Steam玩家"
		if len(r.Author.SteamID) >= 6 {
			author += r.Author.SteamID[len(r.Author.SteamID)-6:] // 仅展示尾号，保护隐私
		}
		out = append(out, models.Discussion{
			GameID:    g.ID,
			Source:    "steam_reviews",
			Author:    author,
			Content:   content,
			Sentiment: sentiment,
			PostedAt:  time.Unix(r.TimestampCreated, 0),
			URL:       "https://steamcommunity.com/profiles/" + r.Author.SteamID,
		})
	}
	return out, nil
}

func (c SteamCollector) fetchReviews(ctx context.Context, appid, language string) ([]struct {
	RecommendationID string `json:"recommendationid"`
	Author           struct {
		SteamID string `json:"steamid"`
	} `json:"author"`
	Language         string `json:"language"`
	Review           string `json:"review"`
	TimestampCreated int64  `json:"timestamp_created"`
	VotedUp          bool   `json:"voted_up"`
}, error) {
	url := fmt.Sprintf("https://store.steampowered.com/appreviews/%s?json=1&filter=recent&num_per_page=20&language=%s",
		appid, language)
	body, err := GetJSON(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("steam: 获取游戏 %s 评论失败: %w", appid, err)
	}
	var resp appReviewsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("steam: 解析游戏 %s 评论失败: %w", appid, err)
	}
	if resp.Success != 1 {
		return nil, fmt.Errorf("steam: 游戏 %s 评论接口返回异常", appid)
	}
	return resp.Reviews, nil
}
