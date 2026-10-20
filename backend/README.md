# 游戏榜单监控平台 · 后端（MVP）

Go 后端：多平台游戏榜单采集、入库、分析与 REST API。

## 快速开始

```bash
export PATH=$HOME/workspace/.tmp-go/go/bin:$PATH
go build ./...
./server -demo        # 演示：采集一次 Steam 全球 hot/daily + Google Play 美国区 hot/daily 入库后退出
./server              # 正常启动：API :8080 + 定时任务
```

参数：`-addr :8080`、`-db ./data/gamerank.db`（相对路径按当前工作目录解析）、`-demo`。

## 定时任务（Asia/Shanghai）

- 每日 02:10：采集各真实平台 hot/daily、new/daily，分析上榜理由后入库；Steam 额外采集前 10 名评论与在线人数采样。
- 每周一 03:00：基于过去 7 天日榜聚合生成周榜。

## 数据源说明

| 平台 | 状态 | 数据来源 |
|---|---|---|
| Steam（全球榜） | ✅ 真实 | `ISteamChartsService/GetMostPlayedGames`（最热游玩榜）、`ISteamUserStats/GetNumberOfCurrentPlayers`（在线人数）、`store.steampowered.com/api/appdetails`（商店详情）、`/appreviews`（评论）、`/api/featuredcategories`（热销/新品）。全部为公开接口，无需 Key。注意官方接口是 `ISteamChartsService`，`ISteamCharts` 会 404。 |
| App Store（中国/美国） | ✅ 真实 | iTunes RSS 公开接口：`topfreeapplications`（热门免费榜）、`newapplications`（新游榜），`genre=6014` 为游戏分类。MVP 未用 `topgrossing`（畅销榜）/`toppaid`（付费榜）：畅销榜涉及收入估算、口径不透明，保持范围克制。 |
| Google Play（美国） | ✅ 爬虫（非官方） | 无官方公开榜单 API：旧版官方榜单页已下线（实测 404/500）。当前抓取 AppBrain 的 Google Play 榜单镜像页（`top_free/game/us` / `top_new_free/game/us`，每日从 Google Play 更新），字段：排名、包名、游戏名、开发者、图标、评分、累计下载。**礼貌策略**：全局串行、请求间隔 ≥2 秒、15s 超时、3 次指数退避重试；命中 429/403/验证码立即停止本次采集。**风险**：违反 Play 服务条款、可能被反爬、对方改版即失效。URL 可用 `GOOGLEPLAY_CHART_URL_HOT` / `GOOGLEPLAY_CHART_URL_NEW` 环境变量覆盖。 |

## API 一览

- `GET /api/platforms` — 平台与区域元信息
- `GET /api/charts?platform=steam&region=global&type=hot&period=daily&date=2026-10-18` — 榜单（date 缺省为最新）
- `GET /api/games/{id}` — 游戏详情（id 形如 `steam:global:730`）
- `GET /api/games/{id}/ranks?days=30` — 排名历史
- `GET /api/games/{id}/players?days=30` — 玩家人数历史
- `GET /api/games/{id}/discussions?limit=20` — 玩家讨论
- `POST /api/collect?platform=steam&region=global` — 手动触发采集

## 榜单主键维度

platform × region × type（hot/new）× period（daily/weekly）× date。Steam region 固定 `global`；Google Play 固定 `us`。
