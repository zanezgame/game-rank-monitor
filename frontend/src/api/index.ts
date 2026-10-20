/**
 * API 统一出口：页面只从这里调用接口。
 *
 * 策略：优先请求真实后端；当后端未启动/请求失败时，自动降级为
 * src/api/mock.ts 的本地示例数据，保证页面可渲染、可演示。
 * 降级时会通过 isMockMode / mockNotice 告知页面展示提示横幅。
 *
 * 设置环境变量 VITE_USE_MOCK=true 可强制使用 Mock（联调前演示用）。
 * 后端联调完成后：删除 mock.ts，并让本文件直接透传 client 即可。
 */
import { ref } from "vue";
import * as client from "./client";
import { getErrorMessage } from "./client";
import * as mock from "./mock";
import type { ChartQuery } from "./client";

// 类型透出，方便各页面 import type
export type {
  ChartEntry,
  ChartPeriod,
  ChartQuery,
  ChartResponse,
  ChartType,
  CollectResult,
  Discussion,
  GameDetail,
  Platform,
  PlayerSample,
  RankHistoryItem,
  ReasonCode,
} from "./client";
export { getErrorMessage };

/** 当前是否处于 Mock（示例数据）模式 */
export const isMockMode = ref(false);
/** Mock 模式下的提示文案（包含失败原因） */
export const mockNotice = ref("");

const FORCE_MOCK = import.meta.env.VITE_USE_MOCK === "true";

/** 包一层降级：真实请求失败 -> 用 Mock 数据兜底 */
async function withFallback<T>(real: () => Promise<T>, fake: () => T): Promise<T> {
  if (FORCE_MOCK) {
    isMockMode.value = true;
    mockNotice.value = "已开启强制 Mock 模式，当前显示本地示例数据。";
    return fake();
  }
  try {
    return await real();
  } catch (err) {
    isMockMode.value = true;
    mockNotice.value = `未能连接后端（${getErrorMessage(err)}），当前显示本地示例数据。`;
    return fake();
  }
}

export function fetchPlatforms(): Promise<client.Platform[]> {
  return withFallback(() => client.fetchPlatforms(), mock.mockPlatforms);
}

export function fetchChart(q: ChartQuery): Promise<client.ChartResponse> {
  return withFallback(() => client.fetchChart(q), () => mock.mockChart(q));
}

export function fetchGame(id: string): Promise<client.GameDetail> {
  return withFallback(() => client.fetchGame(id), () => mock.mockGame(id));
}

export function fetchGameRanks(id: string, days = 30): Promise<client.RankHistoryItem[]> {
  return withFallback(() => client.fetchGameRanks(id, days), () => mock.mockRanks(id, days));
}

export function fetchGamePlayers(id: string, days = 30): Promise<client.PlayerSample[]> {
  return withFallback(() => client.fetchGamePlayers(id, days), () => mock.mockPlayers(id, days));
}

export function fetchGameDiscussions(id: string, limit = 20): Promise<client.Discussion[]> {
  return withFallback(() => client.fetchGameDiscussions(id, limit), () =>
    mock.mockDiscussions(limit),
  );
}

export function triggerCollect(platform: string, region: string): Promise<client.CollectResult> {
  return withFallback(
    () => client.triggerCollect(platform, region),
    () => mock.mockCollect(platform, region),
  );
}

