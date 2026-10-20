# 游戏榜单监控平台 · 前端

Vue 3.4 + Vite 6 + TypeScript（严格模式）+ vue-router 4 + echarts 5 + axios。

监控 Steam / App Store / Google Play 的热门榜与新游榜（每日/每周），
含上榜理由、在线人数、排名走势与玩家讨论。

## 快速开始

```bash
npm install
npm run dev      # http://localhost:5173，/api 代理到 http://localhost:8080
npm run build    # 类型检查 + 生产构建，产物在 dist/
```

后端（Go）默认跑在 `http://localhost:8080`，开发时代理见 `vite.config.ts`。

## 后端未启动时

页面会自动降级为本地示例数据（`src/api/mock.ts`），顶部会显示黄色提示横幅。
如需强制使用 Mock（不经过网络请求）：

```bash
cp .env.example .env   # VITE_USE_MOCK=true
```

## 目录结构

```
src/
  api/
    client.ts   # 后端 API 契约的类型定义 + typed 封装（唯一真相来源）
    mock.ts     # 本地示例数据（后端未就绪时降级使用，联调后可删）
    index.ts    # 统一出口：真实请求失败 -> Mock 降级 + isMockMode 标记
  router/       # / 仪表盘 · /games/:id 详情 · /about 关于
  views/        # Dashboard.vue / GameDetail.vue / About.vue
  components/   # ReasonBadge.vue（上榜理由徽标）/ RankChange.vue（升降）/ GameIcon.vue
  utils/        # format.ts（Asia/Shanghai 日期·人数格式化）/ chart.ts（echarts 按需引入）
```

## 后端 API 契约（前端严格按此调用）

| 方法 | 路径 | 说明 |
| ---- | ---- | ---- |
| GET | `/api/platforms` | 平台列表（含 regions / region_labels / note） |
| GET | `/api/charts?platform=&region=&type=&period=&date=` | 榜单（type=hot\|new，period=daily\|weekly，date 缺省最新） |
| GET | `/api/games/{id}` | 游戏详情（id 如 `steam:global:730`，含冒号，调用时已编码） |
| GET | `/api/games/{id}/ranks?days=30` | 排名历史 |
| GET | `/api/games/{id}/discussions?limit=20` | 玩家讨论 |
| GET | `/api/games/{id}/players?days=30` | 在线人数（非 Steam 返回 `[]`） |
| POST | `/api/collect?platform=&region=` | 手动触发采集 |

`reason_code` 取值：`new_entry`（首次上榜）/ `re_entry`（重回榜单）/ `surge`（大幅上升）/ `drop`（明显下跌）/ `stable`（稳定）；
`rank_change` 正数=上升、负数=下跌、`null`=无历史。

## 给后端联调的说明

1. **game_id 含冒号**：如 `steam:global:730`。前端在路由中直接使用原始 id（冒号在 path segment 中合法），
   发请求时用 `encodeURIComponent` 编码，后端路由请按解码后的完整 id 匹配。
2. **数据来源标注**：前端以后端返回的 `platform.note` 为准显示渠道说明（如 Google Play 的"第三方渠道抓取，非官方"），
   不硬编码平台 id。
3. **日期缺省**：`GET /api/charts` 不传 `date` 时后端返回最新一天，前端日期选择器留空即表示最新。
4. **空数据约定**：非 Steam 平台的 `/players` 返回 `[]`（前端据此显示"该平台无公开人数数据"）；
   在线人数列仅在 `platform=steam && type=hot` 时渲染，其余显示 `—`。
5. **时区**：榜单 `date` 为 `YYYY-MM-DD`；`sampled_at` / `posted_at` 为 ISO 8601（含时区偏移），
   前端统一按 Asia/Shanghai 展示。
