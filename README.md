# 游戏榜单监控平台（MVP）

监控多平台游戏榜单的工具：覆盖 **Steam / App Store / Google Play**，
支持**热门榜 / 新游榜** × **每日榜 / 每周榜** × **中国区 / 美国区**，
每个上榜游戏可查看：上榜理由（首次上榜 / 重回榜单 / 大幅上升 / 下跌 / 稳定 + 促销等事件）、
玩家人数（有公开数据的平台）、玩家讨论摘要。

技术栈：**后端 Go + SQLite，前端 Vue3 + Vite + TypeScript + ECharts**。

## 架构（文字描述）

```
┌─────────────┐     ┌──────────────────────────────┐     ┌──────────┐
│  数据源      │     │  后端 (Go, :8080)             │     │ 前端      │
│             │     │                              │     │ (Vue3)   │
│ Steam Web   │────▶│ collector: 定时/手动采集       │◀────│ 仪表盘   │
│ API(公开)    │     │   ├─ steam.go (真实)          │ REST│ 游戏详情 │
│             │     │   ├─ appstore.go (真实,cn/us) │ JSON│ 关于页   │
│ iTunes RSS  │────▶│   ├─ googleplay.go (第三方镜像抓取) │     │          │
│ (cn/us)     │     │                                 │     └──────────┘
└─────────────┘     │ analyzer: 上榜理由分析        │
                    │ store: SQLite(榜单快照/人数/评论)│
                    │ scheduler: 每日02:10/周一03:00 │
                    └──────────────────────────────┘
```

- **collector**：每个平台实现 `Collector` 接口，失败互相隔离（一个平台挂了不影响其他）。
- **analyzer**：对比上期快照生成上榜理由：`new_entry` 首次上榜 / `re_entry` 重回榜单（缺席>7天）/
  `surge` 上升≥10名 / `drop` 下跌≥10名 / `stable` 稳定，并追加事件（如 Steam 促销）。
- **store**：SQLite（pure-Go 的 `modernc.org/sqlite`，无需 cgo），表结构由
  `backend/internal/store/migrations/*.sql` 管理。榜单主键维度：
  **platform × region × type × period × date**。
- **scheduler**：`robfig/cron`，Asia/Shanghai 时区；每日采集日榜，周一聚合周榜
  （周榜 = 过去 7 天日榜按平均排名聚合，适用于 Apple 这类只有日榜 RSS 的平台）。
  服务常驻时由进程内定时器执行；也可以用 `-collect` / `-weekly` 一次性模式
  接外部定时任务（cron），无需保持服务常驻。

## 目录结构

```
game-rank-monitor/
├── README.md                 # 本文件
├── backend/
│   ├── go.mod                # module gamerank
│   ├── cmd/server/main.go    # 入口：-addr / -db / -demo / -collect / -weekly
│   └── internal/
│       ├── models/           # Game / RankSnapshot / PlayerSample / Discussion
│       ├── store/            # SQLite + migrations/001_init.sql + CRUD
│       ├── collector/        # Collector 接口；steam/appstore 官方接口；googleplay 第三方镜像抓取
│       ├── analyzer/         # 上榜理由分析
│       ├── api/              # REST API (标准库 net/http)
│       └── scheduler/        # 每日/每周采集调度
└── frontend/
    ├── package.json / vite.config.ts / tsconfig.json
    └── src/
        ├── api/client.ts     # 后端 API 的 TypeScript 封装
        ├── views/Dashboard.vue   # 仪表盘：平台Tab × 地区 × 每日/每周 × 热门/新游
        ├── views/GameDetail.vue  # 详情：排名走势 / 在线人数 / 理由时间线 / 玩家讨论
        └── views/About.vue       # 数据说明页
```

## 快速启动

### 1. 后端

需要 Go 1.21+（[go.dev/dl](https://go.dev/dl) 下载安装；本机曾无 Go，
已用官方包临时安装 go1.27.2 在 `~/workspace/.tmp-go/go`，正式环境请按官方步骤安装）。

```bash
cd backend
export PATH=$HOME/workspace/.tmp-go/go/bin:$PATH   # 若用临时安装的 Go
go mod download
go build ./...

# 最小采集演示：拉取 Steam 全球最热榜入库后退出
go run ./cmd/server -demo -db /tmp/demo.db

# 一次性采集全部平台日榜后退出（Steam 全球 / App Store 中美 / Google Play 美国区），
# 适合接外部定时任务，无需常驻服务：
go run ./cmd/server -collect -db ./data/gamerank.db

# 一次性聚合生成周榜后退出（适合每周一由定时任务调用）：
go run ./cmd/server -weekly -db ./data/gamerank.db

# 正式启动（默认 :8080，数据 ./data/gamerank.db）
go run ./cmd/server
# 手动触发采集（另开终端）：
curl -X POST "http://localhost:8080/api/collect?platform=steam&region=global"
curl -X POST "http://localhost:8080/api/collect?platform=appstore"  # cn+us 双区
```

### 2. 前端

```bash
cd frontend
npm install
npm run dev      # 开发模式（vite proxy 已配 /api -> localhost:8080）
npm run build    # 生产构建，产物在 dist/
```

浏览器打开后端 `:8080` 的 API 或前端 dev 地址即可使用。

## REST API 速览

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/platforms` | 平台列表（含 regions、region_labels、占位 note） |
| GET | `/api/charts?platform=appstore&region=cn&type=hot&period=daily&date=2026-10-18` | 榜单（date 缺省为最新） |
| GET | `/api/games/{id}` | 游戏详情（id 形如 `steam:global:730`） |
| GET | `/api/games/{id}/ranks?days=30` | 排名历史 |
| GET | `/api/games/{id}/players?days=30` | 在线人数历史（仅 Steam 有数据） |
| GET | `/api/games/{id}/discussions?limit=20` | 玩家讨论（Steam 评论摘要） |
| POST | `/api/collect?platform=steam&region=global` | 手动触发采集（region 缺省为该平台全部 region） |

## 双区策略

- **App Store**：iTunes RSS 同时采集 `cn`（中国区）与 `us`（美国区），
  前端仪表盘可切换「中国 / 美国」。
- **Steam**：全球榜，无地区概念，`region` 固定为 `global`（前端显示「全球」且不可切换）。
- **Google Play**：中国大陆不可用，`region` 固定为 `us`（美国区榜单，第三方渠道抓取，非官方）。

## 数据源说明与局限（实测结论）

| 平台 | 状态 | 说明 |
|---|---|---|
| Steam | ✅ 真实可用 | 最热榜用 `ISteamChartsService/GetMostPlayedGames/v1`（**注意不是**文档流传的 `ISteamCharts`，后者已 404）。在线人数 `GetNumberOfCurrentPlayers`、详情 `appdetails`、评论 `appreviews`、热销/新品 `featuredcategories` 均无需 key、实测可用。`IStoreService` 需 key，未采用。 |
| App Store | ✅ 真实可用 | iTunes RSS：`topfreeapplications`（热门榜）/`newapplications`（新游榜）+ `genre=6014`（游戏分类），cn/us 双区实测可用。无在线人数、无评论字段，对应接口返回空数组。MVP 未用 `topgrossing`/`toppaid`（范围取舍，后续可加）。 |
| Google Play | ✅ 爬虫可用（非官方） | **无官方公开榜单 API**：旧版官方榜单页已下线（实测 404/500），新版页面无完整排名列表。当前抓取 AppBrain 的 Google Play 榜单镜像页（`top_free/game/us` 热门免费榜 / `top_new_free/game/us` 新游免费榜，每日从 Google Play 更新），字段：排名、包名、游戏名、开发者、图标、评分、累计下载。**风险**：违反 Play 服务条款、可能被反爬、对方改版即失效；抓取遵守礼貌策略（间隔 ≥2 秒、单线程、命中 429/403/验证码即停）。URL 可用环境变量 `GOOGLEPLAY_CHART_URL_HOT` / `GOOGLEPLAY_CHART_URL_NEW` 覆盖。前端与 `/api/platforms` 如实标注"第三方渠道抓取，非官方"。 |
| 玩家讨论 | ✅ 部分可用 | MVP 讨论来源为 **Steam 公开评论 API**（中文优先、缺时回退英文）。其他平台可选：Reddit（有公开 API）、TapTap（反爬需评估），列入路线图。 |

## 游戏分类策略（IAA / 单机 / 个人游戏）

榜单抓到游戏后用**标签体系**分类，MVP 为规则初筛 + 预留人工标注：

- **平台标签**：steam / appstore / googleplay（自动）。
- **变现模式初筛**：Steam `is_free=true` →「免费」；App Store 免费榜 →「免费/IAP」；
  Steam 折扣 / `price_overview` →「付费/促销」。
- **IAA（广告变现）/ 单机 / 个人游戏**：当前规则只能初筛（如免费+无内购标识倾向 IAA），
  准确分类依赖人工标注 —— `games` 表预留 `tags` 字段，可通过管理接口/脚本覆写，
  前端标签即读此字段。路线图：接入 TapTap / 官方分类 + 人工审核流。

## 已知局限

1. Google Play 为第三方渠道抓取（非官方）：对方反爬或改版会导致采集失败，失败时返回中文说明且不影响其他平台。
2. Steam 在线人数接口对个别 appid 返回 404，已按「失败跳过」处理。
3. 上榜理由阈值（surge/drop ±10 名、re_entry 缺席 7 天）为 MVP 默认值，可在 `analyzer` 调整。
4. App Store 无版本更新时间的可靠公开字段，「新游榜」以 `newapplications` RSS 为准。
5. 所有对外 HTTP 调用均带 15s 超时 + 3 次指数退避重试；未使用任何 API key。

## 路线图

- [x] Google Play：第三方榜单镜像抓取已接入（非官方，见风险说明）；后续可评估官方数据 API / 商业数据服务
- [ ] App Store 增加畅销榜（topgrossing）、付费榜（toppaid）
- [ ] 讨论来源扩展：Reddit、TapTap（反爬评估后）
- [ ] IAA/单机标签：人工标注后台 + 规则迭代
- [ ] 榜单异常告警（如某游戏排名异动推送）
