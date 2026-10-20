// Package api 提供 REST API（标准库 net/http，Go 1.22+ ServeMux 路由）。
package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gamerank/internal/collector"
	"gamerank/internal/scheduler"
	"gamerank/internal/store"
)

// Server 持有 API 依赖。
type Server struct {
	Store      *store.Store
	Collectors []collector.Collector
}

// platformMeta 描述平台元信息（用于 /api/platforms）。
type platformMeta struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Regions      []string          `json:"regions"`
	RegionLabels map[string]string `json:"region_labels"`
	Note         string            `json:"note,omitempty"`
}

// platformsMeta 是各平台的静态元信息。
var platformsMeta = []platformMeta{
	{ID: "steam", Name: "Steam", Regions: []string{"global"}, RegionLabels: map[string]string{"global": "全球"}, Note: "全球最热游玩榜"},
	{ID: "appstore", Name: "App Store", Regions: []string{"cn", "us"}, RegionLabels: map[string]string{"cn": "中国", "us": "美国"}},
	{ID: "googleplay", Name: "Google Play", Regions: []string{"us"}, RegionLabels: map[string]string{"us": "美国"}, Note: "美国区榜单（第三方渠道抓取，非官方）"},
}

// NewServer 创建 API 服务器。
func NewServer(st *store.Store, collectors []collector.Collector) *Server {
	return &Server{Store: st, Collectors: collectors}
}

// Handler 返回带 CORS 的 http.Handler。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/platforms", s.handlePlatforms)
	mux.HandleFunc("GET /api/charts", s.handleCharts)
	mux.HandleFunc("GET /api/games/{id}", s.handleGame)
	mux.HandleFunc("GET /api/games/{id}/ranks", s.handleGameRanks)
	mux.HandleFunc("GET /api/games/{id}/players", s.handleGamePlayers)
	mux.HandleFunc("GET /api/games/{id}/discussions", s.handleGameDiscussions)
	mux.HandleFunc("POST /api/collect", s.handleCollect)
	mux.HandleFunc("GET /api/image", s.handleImage)
	mux.HandleFunc("/", s.handleNotFound)
	return corsMiddleware(mux)
}

// corsMiddleware 允许跨域（MVP 阶段允许 *）。
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("写 JSON 响应失败: %v", err)
	}
}

// writeErr 写 {"error":"中文信息"} 错误响应。
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleNotFound 处理未知路径。
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeErr(w, http.StatusNotFound, "接口不存在")
}

// handlePlatforms 返回平台列表。
func (s *Server) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, platformsMeta)
}

// chartEntryJSON 是榜单条目的 API 表示。
type chartEntryJSON struct {
	Rank        int      `json:"rank"`
	GameID      string   `json:"game_id"`
	GameName    string   `json:"game_name"`
	IconURL     string   `json:"icon_url"`
	Tags        []string `json:"tags"`
	RankChange  *int     `json:"rank_change"`
	ReasonCode  string   `json:"reason_code"`
	ReasonText  string   `json:"reason_text"`
	PlayerCount *int64   `json:"player_count"`
}

// handleCharts 返回某日榜单。
// 参数：platform（必填）、region（必填）、type（默认 hot）、period（默认 daily）、date（默认最新）。
func (s *Server) handleCharts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	platform, region := q.Get("platform"), q.Get("region")
	chartType, period := q.Get("type"), q.Get("period")
	date := q.Get("date")
	if platform == "" || region == "" {
		writeErr(w, http.StatusBadRequest, "缺少必填参数：platform、region")
		return
	}
	if chartType == "" {
		chartType = "hot"
	}
	if period == "" {
		period = "daily"
	}
	if chartType != "hot" && chartType != "new" {
		writeErr(w, http.StatusBadRequest, "type 参数只能是 hot 或 new")
		return
	}
	if period != "daily" && period != "weekly" {
		writeErr(w, http.StatusBadRequest, "period 参数只能是 daily 或 weekly")
		return
	}
	if date == "" {
		latest, ok := s.Store.LatestChartDate(platform, region, chartType, period)
		if !ok {
			writeErr(w, http.StatusNotFound, "该榜单暂无数据，请先触发采集")
			return
		}
		date = latest
	}
	entries, err := s.Store.GetChart(platform, region, chartType, period, date)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询榜单失败")
		return
	}
	out := make([]chartEntryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, chartEntryJSON{
			Rank: e.Rank, GameID: e.GameID, GameName: e.GameName, IconURL: e.IconURL,
			Tags: e.Tags, RankChange: e.RankChange, ReasonCode: e.ReasonCode,
			ReasonText: e.ReasonText, PlayerCount: e.PlayerCount,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"platform": platform, "region": region, "type": chartType,
		"period": period, "date": date, "entries": out,
	})
}

// handleGame 返回游戏详情。
func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	g, ok := s.Store.GetGame(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "游戏不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": g.ID, "platform": g.Platform, "region": g.Region,
		"name": g.Name, "icon_url": g.IconURL, "description": g.Description,
		"price_text": g.PriceText, "first_seen": g.FirstSeen,
	})
}

// handleGameRanks 返回游戏排名历史。
func (s *Server) handleGameRanks(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 30)
	hist, err := s.Store.GetRankHistory(r.PathValue("id"), days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询排名历史失败")
		return
	}
	out := make([]map[string]any, 0, len(hist))
	for _, h := range hist {
		out = append(out, map[string]any{
			"date": h.Date, "rank": h.Rank, "rank_change": h.RankChange,
			"reason_code": h.ReasonCode, "reason_text": h.ReasonText,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGamePlayers 返回游戏玩家人数历史（无数据返回空数组）。
func (s *Server) handleGamePlayers(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 30)
	hist, err := s.Store.GetPlayerHistory(r.PathValue("id"), days)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询玩家人数历史失败")
		return
	}
	out := make([]map[string]any, 0, len(hist))
	for _, h := range hist {
		out = append(out, map[string]any{
			"sampled_at": h.SampledAt.Format(time.RFC3339), "count": h.Count,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGameDiscussions 返回游戏玩家讨论。
func (s *Server) handleGameDiscussions(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	discs, err := s.Store.GetDiscussions(r.PathValue("id"), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询玩家讨论失败")
		return
	}
	out := make([]map[string]any, 0, len(discs))
	for _, d := range discs {
		out = append(out, map[string]any{
			"author": d.Author, "content": d.Content, "sentiment": d.Sentiment,
			"posted_at": d.PostedAt.Format(time.RFC3339), "source": d.Source,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCollect 手动触发采集：POST /api/collect?platform=steam&region=global。
// platform 缺省为全部真实平台；region 缺省为该平台的全部区域。
func (s *Server) handleCollect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	platform := q.Get("platform")
	var regions []string
	if reg := q.Get("region"); reg != "" {
		regions = []string{reg}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
	defer cancel()
	results := scheduler.CollectCharts(ctx, s.Store, s.Collectors, platform, regions)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": results})
}

// imageHostAllowlist 是允许代理的图片域名后缀（各平台官方 CDN）。
var imageHostAllowlist = []string{
	"steamstatic.com",       // Steam
	"mzstatic.com",          // App Store
	"googleusercontent.com", // Google Play
}

// handleImage 代理游戏图片：GET /api/image?url=<原始图片URL>。
// 前端同源加载图片，避免 CDN 防盗链、跨域或网络受限时图片裂开；
// 只允许各平台官方 CDN 域名，防止被当作开放代理滥用。
func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("url")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		writeErr(w, http.StatusBadRequest, "图片地址不合法")
		return
	}
	host := u.Hostname()
	allowed := false
	for _, suffix := range imageHostAllowlist {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			allowed = true
			break
		}
	}
	if !allowed {
		writeErr(w, http.StatusForbidden, "不允许代理该域名的图片")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "图片地址不合法")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "图片拉取失败")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeErr(w, http.StatusBadGateway, "图片拉取失败")
		return
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		writeErr(w, http.StatusBadGateway, "目标不是图片")
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 8<<20))
}

// queryInt 读取整数查询参数，非法或缺省时返回默认值。
func queryInt(r *http.Request, key string, def int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || v < 0 {
		return def
	}
	return v
}

