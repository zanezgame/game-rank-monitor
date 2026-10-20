// 游戏榜单监控平台后端入口。
// 用法：
//
//	server [-addr :8080] [-db ./data/gamerank.db] [-demo] [-collect] [-weekly]
//
// -demo 模式：采集一次 Steam 全球 hot/daily 与 Google Play 美国区 hot/daily 入库后退出（用于演示验证）。
// -collect 模式：一次性采集全部平台（Steam 全球、App Store 中美、Google Play 美国区）的
// hot/new 日榜并入库后退出（供外部定时任务调用）。
// -weekly 模式：一次性基于过去 7 天日榜聚合生成周榜后退出（供每周一定时任务调用）。
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gamerank/internal/api"
	"gamerank/internal/collector"
	"gamerank/internal/scheduler"
	"gamerank/internal/store"
)

// allCollectors 返回全部平台采集器。
func allCollectors() []collector.Collector {
	return []collector.Collector{
		collector.SteamCollector{},
		collector.AppStoreCollector{},
		collector.GooglePlayCollector{},
	}
}

// resolveDBPath 把相对路径的 -db 解析为基于当前工作目录的绝对路径
// （命令行程序的通用约定）。绝对路径原样返回。
func resolveDBPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "./data/gamerank.db", "SQLite 数据库路径（相对路径按当前工作目录解析）")
	demo := flag.Bool("demo", false, "演示模式：采集一次 Steam 全球 hot/daily 榜单入库后退出")
	collect := flag.Bool("collect", false, "采集模式：采集全部真实平台的日榜入库后退出（供定时任务调用）")
	weekly := flag.Bool("weekly", false, "周榜模式：聚合生成周榜后退出（供每周定时任务调用）")
	flag.Parse()

	dbFile := resolveDBPath(*dbPath)
	if err := os.MkdirAll(filepath.Dir(dbFile), 0o755); err != nil {
		log.Fatalf("创建数据库目录失败: %v", err)
	}

	st, err := store.New(dbFile)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer st.Close()

	collectors := allCollectors()

	// 启动时打印各平台/区域状态
	log.Println("平台状态：")
	for _, c := range collectors {
		log.Printf("  - %s (%s)：区域 %s", c.DisplayName(), c.Platform(), strings.Join(c.Regions(), ","))
	}

	if *demo {
		runDemo(st, collectors, dbFile)
		return
	}

	if *collect {
		runCollectOnce(st, collectors, dbFile)
		return
	}

	if *weekly {
		runWeeklyOnce(st, collectors, dbFile)
		return
	}

	sched := scheduler.New(st, collectors)
	sched.Start()
	defer sched.Stop()
	log.Println("定时任务已启动（Asia/Shanghai）：每日 02:10 采集日榜，每周一 03:00 聚合周榜")

	srv := api.NewServer(st, collectors)
	httpSrv := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("API 服务启动：http://%s", *addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP 服务异常退出: %v", err)
	}
}

// runDemo 演示模式：采集一次 Steam 全球 hot/daily 与 Google Play 美国区 hot/daily 入库后退出。
func runDemo(st *store.Store, collectors []collector.Collector, dbFile string) {
	log.Println("演示模式：采集 Steam 全球 hot/daily 与 Google Play 美国区 hot/daily…")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	results := scheduler.CollectCharts(ctx, st, collectors, "steam", []string{"global"})
	results = append(results, scheduler.CollectCharts(ctx, st, collectors, "googleplay", []string{"us"})...)
	okCount, total := 0, 0
	for _, r := range results {
		total += r.Count
		status := "成功"
		if !r.OK {
			status = "失败：" + r.Error
		} else {
			okCount++
		}
		log.Printf("  [%s] %s %s：%s（%d 条）", r.Platform, r.Region, r.Chart, status, r.Count)
	}
	log.Printf("演示完成：%d 个榜单采集成功，共入库 %d 条快照，数据库：%s", okCount, total, dbFile)
	if total == 0 {
		log.Println("警告：未入库任何数据，请检查网络或 Steam 接口状态")
		os.Exit(1)
	}
}

// runCollectOnce 采集模式：采集全部平台的日榜（hot/new）入库后退出。
func runCollectOnce(st *store.Store, collectors []collector.Collector, dbFile string) {
	log.Println("采集模式：采集全部真实平台日榜…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	results := scheduler.CollectCharts(ctx, st, collectors, "", nil)
	logCollectResults("采集", results, dbFile)
}

// runWeeklyOnce 周榜模式：基于过去 7 天日榜聚合生成周榜后退出。
func runWeeklyOnce(st *store.Store, collectors []collector.Collector, dbFile string) {
	log.Println("周榜模式：聚合生成周榜…")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	results := scheduler.BuildWeeklyCharts(ctx, st, collectors, "", nil)
	logCollectResults("周榜聚合", results, dbFile)
}

// logCollectResults 打印一次采集/聚合的结果汇总；全部失败时以非零退出码退出，
// 便于外部定时任务感知失败。
func logCollectResults(kind string, results []scheduler.ChartResult, dbFile string) {
	okCount, total := 0, 0
	for _, r := range results {
		total += r.Count
		status := "成功"
		if !r.OK {
			status = "失败：" + r.Error
		} else {
			okCount++
		}
		log.Printf("  [%s] %s %s：%s（%d 条）", r.Platform, r.Region, r.Chart, status, r.Count)
	}
	log.Printf("%s完成：%d/%d 个榜单成功，共 %d 条快照，数据库：%s", kind, okCount, len(results), total, dbFile)
	if okCount == 0 {
		log.Printf("警告：%s未成功采集任何榜单", kind)
		os.Exit(1)
	}
}
