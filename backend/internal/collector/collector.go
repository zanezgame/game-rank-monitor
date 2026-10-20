// Package collector 封装各平台榜单数据的采集逻辑。
// Steam 与 App Store 走官方公开接口，Google Play 走第三方镜像页抓取。
package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gamerank/internal/models"
)

// Entry 是采集器返回的标准化榜单条目。
type Entry struct {
	ExternalID  string // 平台侧原始 ID
	Name        string
	IconURL     string
	Rank        int    // 本次采集的排名（1 开始）
	RankChange  *int   // 排名变化（正数=上升）
	PlayerCount *int64 // 当前在线玩家数（仅 Steam）
	PriceText   string
	Discount    int    // 折扣百分比（Steam 促销时 >0）
	Artist      string // 开发者/发行商（App Store）
	Summary     string // 简介
}

// Collector 是各平台榜单采集器的统一接口。
type Collector interface {
	// Platform 返回平台标识：steam / appstore / googleplay。
	Platform() string
	// Regions 返回该平台支持的采集区域。
	Regions() []string
	// DisplayName 返回中文展示名。
	DisplayName() string
	// Collect 采集指定区域、榜单类型（hot/new）、周期（daily/weekly）的榜单条目。
	Collect(ctx context.Context, region, chartType, period string) ([]Entry, error)
	// CollectDiscussions 采集某游戏的玩家讨论，默认返回 nil, nil。
	CollectDiscussions(ctx context.Context, g models.Game) ([]models.Discussion, error)
}

// httpClient 是所有采集共用的 HTTP 客户端，15 秒超时。
var httpClient = &http.Client{Timeout: 15 * time.Second}

// userAgent 是对外请求的 User-Agent。
const userAgent = "GameRankMonitor/1.0 (+mvp)"

// 重试退避间隔：1s、2s、4s。
var retryBackoffs = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// GetJSON 发起 GET 请求并把响应体读取为字节，失败自动重试 3 次。
// 所有对外 HTTP 请求都应走这个助手，保证超时与重试策略一致。
func GetJSON(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= len(retryBackoffs); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryBackoffs[attempt-1]):
			}
		}
		body, err := doGet(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		log.Printf("请求失败（第 %d 次重试）%s: %v", attempt+1, url, err)
	}
	return nil, fmt.Errorf("请求 %s 失败（已重试 %d 次）: %w", url, len(retryBackoffs), lastErr)
}

func doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20)) // 上限 20MB
	if err != nil {
		return nil, err
	}
	return body, nil
}

// MakeGameID 生成 games 表主键：platform:region:externalID。
func MakeGameID(platform, region, externalID string) string {
	return fmt.Sprintf("%s:%s:%s", platform, region, externalID)
}

// SupportsRegion 检查该采集器是否支持指定区域。
func SupportsRegion(c Collector, region string) bool {
	for _, r := range c.Regions() {
		if r == region {
			return true
		}
	}
	return false
}

// browserUserAgent 是抓取网页时使用的 UA（模拟正常浏览器，避免被简单 UA 规则拦截）。
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// ErrBlocked 表示对方站点返回了 429/403 或验证码页：命中后应立即停止该平台本次采集，
// 不再重试、不硬扛，避免触发更严厉的封禁。
var ErrBlocked = errors.New("对方站点拒绝访问（429/403/验证码），已停止本次采集")

// scrapeMu / lastScrapeTime 保证所有抓取请求全局串行、间隔 ≥2 秒（礼貌爬取）。
var (
	scrapeMu       sync.Mutex
	lastScrapeTime time.Time
)

// captchaKeywords 是响应体中疑似验证码/反爬页面的关键词（小写匹配）。
var captchaKeywords = []string{"captcha", "recaptcha", "unusual traffic", "are you a robot", "人机验证"}

// GetPageHTML 抓取一个网页的 HTML（供爬虫采集器使用）：15 秒超时、最多 3 次指数退避重试；
// 命中 429/403/验证码页时立即返回 ErrBlocked（不再重试）；请求间隔 ≥2 秒、单线程。
func GetPageHTML(ctx context.Context, url string) ([]byte, error) {
	// 礼貌爬取：全局串行，距离上次抓取不足 2 秒则等待
	scrapeMu.Lock()
	if wait := 2*time.Second - time.Since(lastScrapeTime); wait > 0 {
		select {
		case <-ctx.Done():
			scrapeMu.Unlock()
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	lastScrapeTime = time.Now()
	scrapeMu.Unlock()

	var lastErr error
	for attempt := 0; attempt <= len(retryBackoffs); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryBackoffs[attempt-1]):
			}
		}
		body, status, err := doGetPage(ctx, url)
		if err == nil {
			if looksLikeCaptcha(body) {
				return nil, fmt.Errorf("%w：%s 返回了疑似验证码页面", ErrBlocked, url)
			}
			return body, nil
		}
		if errors.Is(err, ErrBlocked) || status == http.StatusTooManyRequests || status == http.StatusForbidden {
			return nil, fmt.Errorf("%w：%s", ErrBlocked, url)
		}
		lastErr = err
		log.Printf("页面抓取失败（第 %d 次重试）%s: %v", attempt+1, url, err)
	}
	return nil, fmt.Errorf("抓取页面 %s 失败（已重试 %d 次）: %w", url, len(retryBackoffs), lastErr)
}

// doGetPage 发起一次页面 GET 请求，返回（正文，状态码，错误）。
func doGetPage(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20)) // 上限 20MB
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
		return nil, resp.StatusCode, fmt.Errorf("%w（HTTP %d）", ErrBlocked, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

// looksLikeCaptcha 检查响应体是否疑似验证码/反爬页面。
func looksLikeCaptcha(body []byte) bool {
	// 仅对小页面（<2KB）做关键词检查，避免误伤正常大页面
	if len(body) > 2048 {
		return false
	}
	lower := strings.ToLower(string(body))
	for _, kw := range captchaKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
