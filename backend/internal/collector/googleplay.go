package collector

import (
	"context"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gamerank/internal/models"
)

// GooglePlayCollector 是 Google Play 的爬虫采集器。
//
// 数据源实测结论（2026-10-18）：
//   - Google 官方旧版榜单 collection 页（如
//     /store/apps/category/GAME/collection/topselling_free）已下线，
//     实测返回 404/500；新版 /store/games 页面为编辑精选内容，没有完整排名列表；
//   - 因此改用 AppBrain 的 Google Play 榜单镜像页作为数据源：
//     页面为规整 HTML 表格（<table id="rankings-table">），声明每日从 Google Play 更新。
//
// 风险提示（另见 README）：这是非官方渠道——违反 Google Play 服务条款、
// 可能被反爬拦截、AppBrain 页面改版即失效。前端与 /api/platforms 会如实标注
// "第三方渠道抓取，非官方"。
//
// URL 可通过环境变量覆盖（默认美国区游戏榜）：
//   - GOOGLEPLAY_CHART_URL_HOT：热门免费榜（默认 top_free/game/us）
//   - GOOGLEPLAY_CHART_URL_NEW：新游免费榜（默认 top_new_free/game/us）
type GooglePlayCollector struct{}

func (GooglePlayCollector) Platform() string    { return "googleplay" }
func (GooglePlayCollector) Regions() []string   { return []string{"us"} }
func (GooglePlayCollector) DisplayName() string { return "Google Play" }

func googlePlayHotURL() string {
	if u := os.Getenv("GOOGLEPLAY_CHART_URL_HOT"); u != "" {
		return u
	}
	return "https://www.appbrain.com/stats/google-play-rankings/top_free/game/us"
}

func googlePlayNewURL() string {
	if u := os.Getenv("GOOGLEPLAY_CHART_URL_NEW"); u != "" {
		return u
	}
	return "https://www.appbrain.com/stats/google-play-rankings/top_new_free/game/us"
}

// Collect 抓取美国区游戏榜：hot=热门免费榜，new=新游免费榜。
func (GooglePlayCollector) Collect(ctx context.Context, region, chartType, period string) ([]Entry, error) {
	if region != "us" {
		return nil, fmt.Errorf("googleplay: 榜单仅支持美国区（region=us），暂不支持 %s", region)
	}
	var url string
	switch chartType {
	case "hot":
		url = googlePlayHotURL()
	case "new":
		url = googlePlayNewURL()
	default:
		return nil, fmt.Errorf("googleplay: 不支持的榜单类型 %s（仅支持 hot/new）", chartType)
	}

	body, err := GetPageHTML(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("googleplay: 抓取榜单页失败: %w", err)
	}
	entries := parseAppBrainRanking(string(body))
	if len(entries) < 20 {
		return nil, fmt.Errorf("googleplay: 榜单页仅解析出 %d 条数据（期望 ≥20），页面结构可能已变化", len(entries))
	}
	return entries, nil
}

// CollectDiscussions 暂不支持（AppBrain 镜像页无评论数据）。
func (GooglePlayCollector) CollectDiscussions(_ context.Context, _ models.Game) ([]models.Discussion, error) {
	return nil, nil
}

// 以下正则针对 AppBrain 榜单表格结构：
// <table id="rankings-table"> … <tr>
//
//	<td class="ranking-rank">1</td> …
//	<td class="ranking-icon-cell"><a href="/app/slug/com.pkg"><img src="…"></a></td>
//	<td class="ranking-app-cell"><a href="/app/slug/com.pkg">游戏名</a>
//	  <div class="ranking-app-cell-creator">by <a>开发者</a></div></td>
//	<td class="ranking-rating-cell">…<span>4.7</span></td>
//	<td>26 M</td>（累计下载） <td>8 M</td>（近期下载）
//
// </tr>
var (
	appBrainTableRe  = regexp.MustCompile(`(?s)<table[^>]*id="rankings-table"[^>]*>(.*?)</table>`)
	appBrainRowRe    = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
	appBrainCellRe   = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	appBrainRankRe   = regexp.MustCompile(`^\s*(\d+)\s*$`)
	appBrainAppRefRe = regexp.MustCompile(`/app/[^"/]+/([A-Za-z0-9_.]+)`)
	appBrainNameRe   = regexp.MustCompile(`<a href="/app/[^"]+">([^<]+)</a>`)
	appBrainDevRe    = regexp.MustCompile(`(?s)ranking-app-cell-creator">\s*by\s*<a[^>]*>([^<]+)</a>`)
	appBrainIconRe   = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	appBrainRateRe   = regexp.MustCompile(`<span>([\d.]+)</span>`)
)

// parseAppBrainRanking 解析 AppBrain 榜单表格，返回标准化条目（按排名排序）。
func parseAppBrainRanking(page string) []Entry {
	m := appBrainTableRe.FindStringSubmatch(page)
	if m == nil {
		return nil
	}
	var entries []Entry
	for _, rowMatch := range appBrainRowRe.FindAllStringSubmatch(m[1], -1) {
		row := rowMatch[1]
		if strings.Contains(row, "<th") {
			continue // 表头行
		}
		cells := appBrainCellRe.FindAllStringSubmatch(row, -1)
		if len(cells) < 5 {
			continue
		}
		rank := 0
		if rm := appBrainRankRe.FindStringSubmatch(stripTags(cells[0][1])); rm != nil {
			rank, _ = strconv.Atoi(rm[1])
		}
		if rank <= 0 {
			continue
		}
		appCell, rateCell := cells[3][1], cells[4][1]
		pkg := ""
		if pm := appBrainAppRefRe.FindStringSubmatch(appCell); pm != nil {
			pkg = pm[1]
		}
		name := ""
		if nm := appBrainNameRe.FindStringSubmatch(appCell); nm != nil {
			name = cleanText(nm[1])
		}
		if pkg == "" || name == "" {
			continue
		}
		dev := ""
		if dm := appBrainDevRe.FindStringSubmatch(appCell); dm != nil {
			dev = cleanText(dm[1])
		}
		icon := ""
		if im := appBrainIconRe.FindStringSubmatch(cells[2][1]); im != nil {
			icon = html.UnescapeString(im[1])
		}
		rating := ""
		if rm := appBrainRateRe.FindStringSubmatch(rateCell); rm != nil {
			rating = rm[1]
		}
		installs := ""
		if len(cells) > 5 {
			installs = cleanText(stripTags(cells[5][1]))
		}
		summary := ""
		if rating != "" || installs != "" {
			summary = "评分 " + nonEmpty(rating, "暂无") + " · 累计下载 " + nonEmpty(installs, "未知")
		}
		entries = append(entries, Entry{
			ExternalID: pkg,
			Name:       name,
			IconURL:    icon,
			Rank:       rank,
			Artist:     dev,
			PriceText:  "免费",
			Summary:    summary,
		})
	}
	return entries
}

// stripTags 去掉 HTML 标签，返回纯文本。
func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
}

// cleanText 做 HTML 反转义 + 空白归一化（如把 "Nebula  Studio" 还原为 "Nebula Studio"）。
func cleanText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

// nonEmpty 为空时返回 fallback。
func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
