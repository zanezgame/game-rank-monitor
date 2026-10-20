// Package scheduler 负责定时采集：每日榜采集入库、每周榜聚合，
// 并提供可被 API 手动触发的采集流水线（CollectCharts / BuildWeeklyCharts）。
package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"gamerank/internal/analyzer"
	"gamerank/internal/collector"
	"gamerank/internal/models"
	"gamerank/internal/store"

	"github.com/robfig/cron/v3"
)

// AsiaShanghai 是调度使用的时区。
var AsiaShanghai = mustLoadLocation()

func mustLoadLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		log.Printf("加载 Asia/Shanghai 时区失败，使用本地时区: %v", err)
		return time.Local
	}
	return loc
}

// ChartResult 是单次榜单采集的结果。
type ChartResult struct {
	Platform string `json:"platform"`
	Region   string `json:"region"`
	Chart    string `json:"chart"` // 如 "hot/daily"
	OK       bool   `json:"ok"`
	Count    int    `json:"count"`
	Error    string `json:"error"`
}

// todayStr 返回 Asia/Shanghai 时区的今日日期（YYYY-MM-DD）。
func todayStr() string {
	return time.Now().In(AsiaShanghai).Format("2006-01-02")
}

// isRealCollector 判断是否为真实采集器（Steam / App Store / Google Play 参与定时任务）。
func isRealCollector(c collector.Collector) bool {
	switch c.Platform() {
	case "steam", "appstore", "googleplay":
		return true
	default:
		return false
	}
}

// CollectCharts 为指定平台（为空则全部真实平台）与区域采集 hot+daily、new+daily
// 两类榜单，计算上榜理由后入库；Steam 额外采集前 10 名的评论与玩家人数采样。
// 每个平台独立捕获错误，互不影响。
func CollectCharts(ctx context.Context, st *store.Store, collectors []collector.Collector, platform string, regions []string) []ChartResult {
	var results []ChartResult
	for _, c := range collectors {
		if platform != "" && c.Platform() != platform {
			continue
		}
		if !isRealCollector(c) {
			if platform != "" {
				// 手动指定非定时平台时，返回明确的中文错误而非静默跳过
				_, err := c.Collect(ctx, "", "", "")
				msg := "该平台暂无可用数据源"
				if err != nil {
					msg = err.Error()
				}
				results = append(results, ChartResult{Platform: c.Platform(), Region: firstRegion(c), Chart: "hot/daily", OK: false, Error: msg})
			}
			continue
		}
		rs := c.Regions()
		if len(regions) > 0 {
			rs = regions
		}
		for _, region := range rs {
			if !collector.SupportsRegion(c, region) {
				results = append(results, ChartResult{Platform: c.Platform(), Region: region, OK: false, Error: fmt.Sprintf("平台 %s 不支持区域 %s", c.DisplayName(), region)})
				continue
			}
			for _, chartType := range []string{"hot", "new"} {
				results = append(results, collectOne(ctx, st, c, region, chartType))
			}
		}
	}
	return results
}

// firstRegion 返回采集器的首个区域（用于占位平台的错误提示）。
func firstRegion(c collector.Collector) string {
	if rs := c.Regions(); len(rs) > 0 {
		return rs[0]
	}
	return ""
}

// collectOne 采集单个（平台，区域，榜单类型）组合，带 panic 隔离。
func collectOne(ctx context.Context, st *store.Store, c collector.Collector, region, chartType string) (res ChartResult) {
	res = ChartResult{Platform: c.Platform(), Region: region, Chart: chartType + "/daily"}
	defer func() {
		if r := recover(); r != nil {
			res.OK = false
			res.Error = fmt.Sprintf("采集发生异常: %v", r)
			log.Printf("采集 %s %s %s panic: %v", c.Platform(), region, chartType, r)
		}
	}()

	entries, err := c.Collect(ctx, region, chartType, "daily")
	if err != nil {
		res.Error = fmt.Sprintf("采集失败: %v", err)
		log.Printf("采集 %s %s %s 失败: %v", c.DisplayName(), region, chartType, err)
		return res
	}

	if err := SaveChartEntries(st, c.Platform(), c.DisplayName(), region, chartType, "daily", todayStr(), entries); err != nil {
		res.Error = fmt.Sprintf("保存榜单失败: %v", err)
		log.Printf("保存 %s %s %s 榜单失败: %v", c.DisplayName(), region, chartType, err)
		return res
	}

	// 3. Steam 额外数据：前 10 名评论 + 在线人数采样
	if c.Platform() == "steam" {
		collectSteamExtras(ctx, st, c, entries)
	}

	res.OK = true
	res.Count = len(entries)
	log.Printf("采集完成 %s %s %s：%d 条", c.DisplayName(), region, chartType, len(entries))
	return res
}

// SaveChartEntries 把一批榜单条目写入游戏表与榜单快照（含上榜理由分析）。
// 由采集流水线调用；date 为榜单日期（YYYY-MM-DD），period 为 daily/weekly。
func SaveChartEntries(st *store.Store, platform, displayName, region, chartType, period, date string, entries []collector.Entry) error {
	// 1. 游戏基础信息入库
	for _, e := range entries {
		g := models.Game{
			ID:          collector.MakeGameID(platform, region, e.ExternalID),
			Platform:    platform,
			Region:      region,
			ExternalID:  e.ExternalID,
			Name:        e.Name,
			IconURL:     e.IconURL,
			Description: e.Summary,
			PriceText:   e.PriceText,
			TagsJSON:    "[]",
			FirstSeen:   date,
		}
		if g.Name == "" {
			g.Name = "未知游戏 " + e.ExternalID
		}
		if err := st.UpsertGame(g); err != nil {
			log.Printf("保存游戏 %s 失败: %v", g.ID, err)
		}
	}

	// 2. 快照入库（含上榜理由分析）
	var snaps []models.RankSnapshot
	for _, e := range entries {
		gameID := collector.MakeGameID(platform, region, e.ExternalID)
		sn := models.RankSnapshot{
			GameID:      gameID,
			Platform:    platform,
			Region:      region,
			ChartType:   chartType,
			Period:      period,
			Date:        date,
			Rank:        e.Rank,
			PlayerCount: e.PlayerCount,
		}
		var events []string
		if ev := analyzer.DiscountEvent(e.Discount); ev != "" {
			events = append(events, ev)
		}
		prev, ok := st.GetPrevSnapshot(gameID, platform, region, chartType, period, date)
		if ok {
			d := prev.Rank - sn.Rank
			sn.RankChange = &d
			code, text := analyzer.Analyze(prev, sn, events)
			sn.ReasonCode, sn.ReasonText = code, text
		} else {
			// 数据库无历史时，回退使用采集器自带的变化值（如 Steam 的上周排名）
			sn.RankChange = e.RankChange
			code, text := analyzer.Analyze(nil, sn, events)
			sn.ReasonCode, sn.ReasonText = code, text
		}
		snaps = append(snaps, sn)
	}
	if err := st.SaveSnapshots(snaps); err != nil {
		return fmt.Errorf("保存 %s %s %s 榜单失败: %w", displayName, region, chartType, err)
	}
	return nil
}

// collectSteamExtras 采集 Steam 榜单前 10 名的玩家讨论，以及所有带在线人数条目的采样。
func collectSteamExtras(ctx context.Context, st *store.Store, c collector.Collector, entries []collector.Entry) {
	now := time.Now()
	// 玩家人数采样
	var samples []models.PlayerSample
	for _, e := range entries {
		if e.PlayerCount == nil {
			continue
		}
		samples = append(samples, models.PlayerSample{
			GameID:    collector.MakeGameID(c.Platform(), "global", e.ExternalID),
			SampledAt: now,
			Count:     *e.PlayerCount,
		})
	}
	if err := st.SavePlayerSamples(samples); err != nil {
		log.Printf("保存 Steam 玩家人数采样失败: %v", err)
	}
	// 前 10 名评论
	n := 10
	if len(entries) < n {
		n = len(entries)
	}
	for i := 0; i < n; i++ {
		e := entries[i]
		g, ok := st.GetGame(collector.MakeGameID(c.Platform(), "global", e.ExternalID))
		if !ok {
			continue
		}
		discs, err := c.CollectDiscussions(ctx, g)
		if err != nil {
			log.Printf("采集 %s 评论失败: %v", g.Name, err)
			continue
		}
		if err := st.SaveDiscussions(discs); err != nil {
			log.Printf("保存 %s 评论失败: %v", g.Name, err)
		} else if len(discs) > 0 {
			log.Printf("采集 %s 评论 %d 条", g.Name, len(discs))
		}
		time.Sleep(300 * time.Millisecond) // 评论接口间隔，避免限流
	}
}

// BuildWeeklyCharts 为指定平台/区域生成周榜（基于过去 7 天日榜聚合）。
func BuildWeeklyCharts(ctx context.Context, st *store.Store, collectors []collector.Collector, platform string, regions []string) []ChartResult {
	var results []ChartResult
	weekEnd := todayStr()
	for _, c := range collectors {
		if !isRealCollector(c) {
			continue
		}
		if platform != "" && c.Platform() != platform {
			continue
		}
		rs := c.Regions()
		if len(regions) > 0 {
			rs = regions
		}
		for _, region := range rs {
			for _, chartType := range []string{"hot", "new"} {
				res := ChartResult{Platform: c.Platform(), Region: region, Chart: chartType + "/weekly"}
				func() {
					defer func() {
						if r := recover(); r != nil {
							res.Error = fmt.Sprintf("生成周榜发生异常: %v", r)
						}
					}()
					snaps, err := st.BuildWeeklySnapshots(c.Platform(), region, chartType, weekEnd, analyzer.Analyze)
					if err != nil {
						res.Error = fmt.Sprintf("生成周榜失败: %v", err)
						return
					}
					res.OK = true
					res.Count = len(snaps)
				}()
				results = append(results, res)
			}
		}
	}
	return results
}

// Scheduler 封装 robfig/cron 定时任务。
type Scheduler struct {
	cron       *cron.Cron
	store      *store.Store
	collectors []collector.Collector
}

// New 创建定时调度器（Asia/Shanghai 时区）。
func New(st *store.Store, collectors []collector.Collector) *Scheduler {
	c := cron.New(cron.WithLocation(AsiaShanghai))
	s := &Scheduler{cron: c, store: st, collectors: collectors}
	// 每日 02:10：采集各真实平台 hot+daily、new+daily
	_, _ = c.AddFunc("10 2 * * *", func() {
		log.Println("定时任务启动：每日榜单采集")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		CollectCharts(ctx, s.store, s.collectors, "", nil)
	})
	// 每周一 03:00：聚合生成周榜
	_, _ = c.AddFunc("0 3 * * 1", func() {
		log.Println("定时任务启动：周榜聚合")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		BuildWeeklyCharts(ctx, s.store, s.collectors, "", nil)
	})
	return s
}

// Start 启动定时任务。
func (s *Scheduler) Start() { s.cron.Start() }

// Stop 停止定时任务。
func (s *Scheduler) Stop() { s.cron.Stop() }
