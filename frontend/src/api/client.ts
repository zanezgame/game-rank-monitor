/**
 * 后端 API 客户端：类型定义 + 请求封装。
 *
 * 后端 API 契约（由并行开发的 Go 后端提供）：
 * - GET /api/platforms                     平台列表
 * - GET /api/charts?platform=&region=&type=&period=&date=  榜单
 * - GET /api/games/{id}                    游戏详情
 * - GET /api/games/{id}/ranks?days=30       排名历史
 * - GET /api/games/{id}/players?days=30     在线人数历史
 * - GET /api/games/{id}/discussions?limit=20 玩家讨论
 * - POST /api/collect?platform=&region=     手动触发采集
 *
 * 注意：game_id 形如 "steam:global:730"，包含冒号；
 * 在路由/URL 中传递时必须做 encodeURIComponent 编码。
 */
import axios, { type AxiosInstance, type AxiosError } from "axios";

/* ================= 类型定义（严格对应后端契约） ================= */

/** 榜单类型：hot 热门榜 / new 新游榜 */
export type ChartType = "hot" | "new";
/** 榜单周期：daily 每日 / weekly 每周 */
export type ChartPeriod = "daily" | "weekly";

/** 上榜理由码 */
export type ReasonCode = "new_entry" | "re_entry" | "surge" | "drop" | "stable";

/** 平台定义 */
export interface Platform {
  id: string;
  name: string;
  /** 支持的地区代码，如 ["cn", "us"] */
  regions: string[];
  /** 地区代码 -> 中文名，如 { cn: "中国", us: "美国" } */
  region_labels: Record<string, string>;
  /** 数据来源备注，如 "美国区榜单（第三方渠道抓取，非官方）" */
  note?: string;
}

/** 榜单条目 */
export interface ChartEntry {
  rank: number;
  game_id: string;
  game_name: string;
  icon_url?: string;
  tags?: string[];
  /** 正数=上升、负数=下跌、null=无历史数据 */
  rank_change: number | null;
  reason_code: ReasonCode;
  reason_text: string;
  /** 在线人数（仅 Steam 热门榜有数据，其他为缺省） */
  player_count?: number | null;
}

/** 榜单响应 */
export interface ChartResponse {
  platform: string;
  region: string;
  type: ChartType;
  period: ChartPeriod;
  date: string;
  entries: ChartEntry[];
}

/** 游戏详情 */
export interface GameDetail {
  id: string;
  platform: string;
  region: string;
  name: string;
  icon_url?: string;
  tags?: string[];
  description?: string;
  price_text?: string;
}

/** 排名历史单条 */
export interface RankHistoryItem {
  date: string;
  rank: number;
  rank_change: number | null;
  reason_code: ReasonCode;
  reason_text: string;
}

/** 在线人数采样点 */
export interface PlayerSample {
  sampled_at: string;
  count: number;
}

/** 玩家讨论 */
export interface Discussion {
  author: string;
  content: string;
  /** positive 好评 / negative 差评 / neutral 中性 */
  sentiment: "positive" | "negative" | "neutral";
  posted_at: string;
  source: string;
}

/** 手动采集结果 */
export interface CollectResult {
  ok: boolean;
  results: Array<{
    platform: string;
    region: string;
    chart: string;
    ok: boolean;
    count: number;
    error: string;
  }>;
}

/* ================= axios 实例 ================= */

const api: AxiosInstance = axios.create({
  // 同源请求；开发环境下 vite.config.ts 会把 /api 代理到 localhost:8080
  baseURL: "/api",
  timeout: 15000,
});

/** 从 AxiosError 中提取可读错误信息 */
export function getErrorMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const e = err as AxiosError<{ message?: string; error?: string }>;
    if (e.response?.data?.message) return e.response.data.message;
    if (e.response?.data?.error) return e.response.data.error;
    if (e.response) return `请求失败（HTTP ${e.response.status}）`;
    if (e.code === "ECONNABORTED") return "请求超时，请稍后重试";
    return `网络错误：${e.message}`;
  }
  return err instanceof Error ? err.message : "未知错误";
}

/* ================= API 封装 ================= */

/** GET /api/platforms 平台列表 */
export function fetchPlatforms(): Promise<Platform[]> {
  return api.get<Platform[]>("/platforms").then((r) => r.data);
}

export interface ChartQuery {
  platform: string;
  region: string;
  type: ChartType;
  period: ChartPeriod;
  /** 格式 YYYY-MM-DD；缺省为最新 */
  date?: string;
}

/** GET /api/charts 榜单 */
export function fetchChart(q: ChartQuery): Promise<ChartResponse> {
  return api
    .get<ChartResponse>("/charts", {
      params: {
        platform: q.platform,
        region: q.region,
        type: q.type,
        period: q.period,
        ...(q.date ? { date: q.date } : {}),
      },
    })
    .then((r) => r.data);
}

/** GET /api/games/{id} 游戏详情（id 含冒号，调用前请先编码） */
export function fetchGame(id: string): Promise<GameDetail> {
  return api.get<GameDetail>(`/games/${encodeURIComponent(id)}`).then((r) => r.data);
}

/** GET /api/games/{id}/ranks 排名历史 */
export function fetchGameRanks(id: string, days = 30): Promise<RankHistoryItem[]> {
  return api
    .get<RankHistoryItem[]>(`/games/${encodeURIComponent(id)}/ranks`, { params: { days } })
    .then((r) => r.data);
}

/** GET /api/games/{id}/players 在线人数历史（非 Steam 返回 []） */
export function fetchGamePlayers(id: string, days = 30): Promise<PlayerSample[]> {
  return api
    .get<PlayerSample[]>(`/games/${encodeURIComponent(id)}/players`, { params: { days } })
    .then((r) => r.data);
}

/** GET /api/games/{id}/discussions 玩家讨论 */
export function fetchGameDiscussions(id: string, limit = 20): Promise<Discussion[]> {
  return api
    .get<Discussion[]>(`/games/${encodeURIComponent(id)}/discussions`, { params: { limit } })
    .then((r) => r.data);
}

/** POST /api/collect 手动触发采集 */
export function triggerCollect(platform: string, region: string): Promise<CollectResult> {
  return api
    .post<CollectResult>("/collect", null, { params: { platform, region } })
    .then((r) => r.data);
}

