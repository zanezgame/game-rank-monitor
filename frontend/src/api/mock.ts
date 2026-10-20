/**
 * 本地 Mock 数据：后端尚未启动时，保证页面可渲染、可演示。
 *
 * - 所有返回形状严格遵循 src/api/client.ts 中的后端契约。
 * - 同一查询参数（平台/地区/榜单类型/日期）生成确定性数据，
 *   切换日期能看到不同的榜单，刷新页面数据保持稳定。
 * - 仅在开发演示阶段使用；后端联调完成后可直接删除本文件，
 *   并把 src/api/index.ts 改为直接透传 client。
 */
import type {
  ChartEntry,
  ChartQuery,
  ChartResponse,
  CollectResult,
  Discussion,
  GameDetail,
  Platform,
  PlayerSample,
  RankHistoryItem,
  ReasonCode,
} from "./client";
import { REASON_TEXT, addDays, todayStr } from "@/utils/format";

/* ---------- 确定性伪随机数 ---------- */

/** 字符串哈希（FNV-1a） */
function hashStr(s: string): number {
  let h = 2166136261 >>> 0;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

/** mulberry32 伪随机数发生器 */
function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** 把时间戳格式化为带 +08:00 的 ISO 字符串（与后端契约示例一致） */
function toShanghaiISO(ts: number): string {
  const d = new Date(ts + 8 * 3600_000);
  const p = (n: number): string => String(n).padStart(2, "0");
  return (
    `${d.getUTCFullYear()}-${p(d.getUTCMonth() + 1)}-${p(d.getUTCDate())}` +
    `T${p(d.getUTCHours())}:${p(d.getUTCMinutes())}:${p(d.getUTCSeconds())}+08:00`
  );
}

/* ---------- 示例游戏池 ---------- */

interface MockGame {
  appid: number;
  name: string;
  tags: string[];
  price: string;
}

const POOLS: Record<string, MockGame[]> = {
  "steam:global:hot": [
    { appid: 730, name: "Counter-Strike 2", tags: ["免费", "射击"], price: "免费游玩" },
    { appid: 570, name: "Dota 2", tags: ["免费", "MOBA"], price: "免费游玩" },
    { appid: 578080, name: "PUBG: BATTLEGROUNDS", tags: ["免费", "大逃杀"], price: "免费游玩" },
    { appid: 1172470, name: "Apex Legends", tags: ["免费", "射击"], price: "免费游玩" },
    { appid: 1203220, name: "永劫无间", tags: ["免费", "动作"], price: "免费游玩" },
    { appid: 252490, name: "Rust", tags: ["生存", "多人"], price: "¥116" },
    { appid: 440, name: "Team Fortress 2", tags: ["免费", "射击"], price: "免费游玩" },
    { appid: 271590, name: "GTA V", tags: ["开放世界", "动作"], price: "¥98" },
    { appid: 1086940, name: "Baldur's Gate 3", tags: ["RPG", "回合制"], price: "¥298" },
    { appid: 1245620, name: "ELDEN RING", tags: ["魂系", "开放世界"], price: "¥298" },
    { appid: 2358720, name: "黑神话：悟空", tags: ["动作", "单机"], price: "¥268" },
    { appid: 2246340, name: "Monster Hunter Wilds", tags: ["动作", "共斗"], price: "¥368" },
    { appid: 2379780, name: "Deadlock", tags: ["免费", "MOBA"], price: "免费游玩" },
    { appid: 230410, name: "Warframe", tags: ["免费", "科幻"], price: "免费游玩" },
    { appid: 2694490, name: "Path of Exile 2", tags: ["免费", "暗黑"], price: "免费游玩" },
  ],
  "steam:global:new": [
    { appid: 3007280, name: "Hades II", tags: ["Roguelike", "动作"], price: "¥108" },
    { appid: 3164500, name: "Schedule I", tags: ["模拟", "独立"], price: "¥70" },
    { appid: 2701500, name: "R.E.P.O.", tags: ["合作", "恐怖"], price: "¥58" },
    { appid: 2854490, name: "inZOI", tags: ["模拟", "生活"], price: "¥198" },
    { appid: 2835570, name: "Blue Prince", tags: ["解谜", "独立"], price: "¥90" },
    { appid: 3405340, name: "Buckshot Roulette", tags: ["恐怖", "独立"], price: "¥22" },
    { appid: 1966720, name: "Lethal Company", tags: ["合作", "恐怖"], price: "¥70" },
    { appid: 1623730, name: "Palworld", tags: ["生存", "开放世界"], price: "¥108" },
  ],
  "appstore:cn:hot": [
    { appid: 989673964, name: "王者荣耀", tags: ["免费", "MOBA"], price: "免费（含内购）" },
    { appid: 1321803705, name: "和平精英", tags: ["免费", "射击"], price: "免费（含内购）" },
    { appid: 1528633802, name: "金铲铲之战", tags: ["免费", "自走棋"], price: "免费（含内购）" },
    { appid: 1609716008, name: "蛋仔派对", tags: ["免费", "休闲"], price: "免费（含内购）" },
    { appid: 1467191514, name: "原神", tags: ["免费", "开放世界"], price: "免费（含内购）" },
    { appid: 1598434173, name: "崩坏：星穹铁道", tags: ["免费", "回合制"], price: "免费（含内购）" },
    { appid: 1604910159, name: "三角洲行动", tags: ["免费", "射击"], price: "免费（含内购）" },
    { appid: 1613385715, name: "鸣潮", tags: ["免费", "开放世界"], price: "免费（含内购）" },
  ],
  "appstore:cn:new": [
    { appid: 1600000001, name: "燕云十六声", tags: ["免费", "武侠"], price: "免费（含内购）" },
    { appid: 1600000002, name: "无限暖暖", tags: ["免费", "换装"], price: "免费（含内购）" },
    { appid: 1600000003, name: "漫威终极逆转", tags: ["免费", "卡牌"], price: "免费（含内购）" },
    { appid: 1600000004, name: "星落", tags: ["免费", "二次元"], price: "免费（含内购）" },
    { appid: 1600000005, name: "异环", tags: ["免费", "都市"], price: "免费（含内购）" },
  ],
  "appstore:us:hot": [
    { appid: 431946152, name: "Roblox", tags: ["免费", "沙盒"], price: "Free (IAP)" },
    { appid: 1624601164, name: "MONOPOLY GO!", tags: ["免费", "休闲"], price: "Free (IAP)" },
    { appid: 1489822082, name: "Royal Match", tags: ["免费", "三消"], price: "Free (IAP)" },
    { appid: 553834731, name: "Candy Crush Saga", tags: ["免费", "三消"], price: "Free (IAP)" },
    { appid: 1467191514, name: "Genshin Impact", tags: ["免费", "开放世界"], price: "Free (IAP)" },
    { appid: 1094591345, name: "Pokémon GO", tags: ["免费", "AR"], price: "Free (IAP)" },
    { appid: 584317847, name: "Township", tags: ["免费", "模拟"], price: "Free (IAP)" },
    { appid: 1295239051, name: "Coin Master", tags: ["免费", "休闲"], price: "Free (IAP)" },
  ],
  "appstore:us:new": [
    { appid: 1600000011, name: "Squad Busters", tags: ["免费", "动作"], price: "Free (IAP)" },
    { appid: 1600000012, name: "Zenless Zone Zero", tags: ["免费", "动作"], price: "Free (IAP)" },
    { appid: 1600000013, name: "Wuthering Waves", tags: ["免费", "开放世界"], price: "Free (IAP)" },
    { appid: 1600000014, name: "AFK Journey", tags: ["免费", "放置"], price: "Free (IAP)" },
  ],
};

/** 取示例游戏池；weekly 沿用同类型池 */
function getPool(platform: string, region: string, type: string): MockGame[] {
  return (
    POOLS[`${platform}:${region}:${type}`] ??
    POOLS[`${platform}:${region}:hot`] ?? [
      { appid: 9001, name: "示例游戏 A", tags: ["免费"], price: "免费游玩" },
      { appid: 9002, name: "示例游戏 B", tags: ["免费"], price: "免费游玩" },
    ]
  );
}

/* ---------- Mock API（签名与 client.ts 一致） ---------- */

/** GET /api/platforms */
export function mockPlatforms(): Platform[] {
  return [
    { id: "steam", name: "Steam", regions: ["global"], region_labels: { global: "全球" } },
    {
      id: "appstore",
      name: "App Store",
      regions: ["cn", "us"],
      region_labels: { cn: "中国", us: "美国" },
    },
    {
      id: "googleplay",
      name: "Google Play",
      regions: ["us"],
      region_labels: { us: "美国" },
      note: "美国区榜单（第三方渠道抓取，非官方）",
    },
  ];
}

/** GET /api/charts */
export function mockChart(q: ChartQuery): ChartResponse {
  const date = q.date ?? todayStr();
  const rnd = mulberry32(hashStr(`${q.platform}|${q.region}|${q.type}|${date}`));
  const pool = getPool(q.platform, q.region, q.type);
  const hasPlayerCount = q.platform === "steam" && q.type === "hot";

  const entries: ChartEntry[] = pool.map((g, i) => {
    const roll = rnd();
    let reason: ReasonCode;
    let change: number | null;
    if (roll < 0.12) {
      reason = "new_entry";
      change = null;
    } else if (roll < 0.18) {
      reason = "re_entry";
      change = Math.floor(rnd() * 8) + 3;
    } else {
      const delta = Math.round((rnd() - 0.5) * 24);
      change = delta;
      if (delta >= 6) reason = "surge";
      else if (delta <= -6) reason = "drop";
      else reason = "stable";
    }
    return {
      rank: i + 1,
      game_id: `${q.platform}:${q.region}:${g.appid}`,
      game_name: g.name,
      icon_url: "",
      tags: g.tags,
      rank_change: change,
      reason_code: reason,
      reason_text: REASON_TEXT[reason],
      player_count: hasPlayerCount
        ? Math.max(20_000, Math.floor((1_200_000 / (i + 1)) * (0.7 + rnd() * 0.6)))
        : null,
    };
  });

  return {
    platform: q.platform,
    region: q.region,
    type: q.type,
    period: q.period,
    date,
    entries,
  };
}

/** 从 game_id 反查示例游戏 */
function findMockGame(id: string): { platform: string; region: string; game: MockGame } {
  const [platform = "steam", region = "global", appidStr = ""] = id.split(":");
  const appid = Number(appidStr);
  const all = Object.entries(POOLS).flatMap(([key, games]) =>
    games.map((g) => ({ key, g })),
  );
  const hit = all.find((x) => x.g.appid === appid);
  const game: MockGame = hit
    ? hit.g
    : { appid: Number.isNaN(appid) ? 0 : appid, name: `游戏 ${appidStr}`, tags: ["示例"], price: "免费游玩" };
  return { platform, region, game };
}

/** GET /api/games/{id} */
export function mockGame(id: string): GameDetail {
  const { platform, region, game } = findMockGame(id);
  return {
    id,
    platform,
    region,
    name: game.name,
    icon_url: "",
    tags: game.tags,
    description: `「${game.name}」是一款${game.tags.join("、")}类游戏，深受玩家喜爱。（本地示例数据）`,
    price_text: game.price,
  };
}

/** GET /api/games/{id}/ranks */
export function mockRanks(id: string, days = 30): RankHistoryItem[] {
  const anchor = todayStr();
  const rnd = mulberry32(hashStr(`ranks|${id}`));
  const base = 3 + Math.floor(rnd() * 12);
  const items: RankHistoryItem[] = [];
  let prevRank: number | null = null;
  for (let i = days - 1; i >= 0; i--) {
    const date = addDays(anchor, -i);
    const rank = Math.max(1, Math.min(100, base + Math.round((rnd() - 0.5) * 10)));
    let reason: ReasonCode;
    let change: number | null;
    if (prevRank === null) {
      reason = i === days - 1 ? "new_entry" : "stable";
      change = null;
    } else {
      const delta = prevRank - rank;
      change = delta;
      if (delta >= 6) reason = "surge";
      else if (delta <= -6) reason = "drop";
      else reason = "stable";
    }
    items.push({
      date,
      rank,
      rank_change: change,
      reason_code: reason,
      reason_text: REASON_TEXT[reason],
    });
    prevRank = rank;
  }
  return items;
}

/** GET /api/games/{id}/players（非 Steam 返回 []，与契约一致） */
export function mockPlayers(id: string, days = 30): PlayerSample[] {
  const [platform] = id.split(":");
  if (platform !== "steam") return [];
  const rnd = mulberry32(hashStr(`players|${id}`));
  const base = 300_000 + Math.floor(rnd() * 700_000);
  // 锚定今天 0 点（+08:00），每 3 小时一个采样点
  const now = new Date();
  const anchorTs =
    Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()) - 8 * 3600_000;
  const points: PlayerSample[] = [];
  const total = days * 8;
  for (let k = total - 1; k >= 0; k--) {
    const ts = anchorTs - k * 3 * 3600_000;
    // 模拟昼夜波动：晚上 20 点左右在线人数最高
    const hour = new Date(ts + 8 * 3600_000).getUTCHours();
    const wave = 1 + 0.35 * Math.sin(((hour - 8) / 24) * Math.PI * 2);
    const count = Math.max(5_000, Math.floor(base * wave * (0.9 + rnd() * 0.2)));
    points.push({ sampled_at: toShanghaiISO(ts), count });
  }
  return points;
}

/** GET /api/games/{id}/discussions */
export function mockDiscussions(limit = 20): Discussion[] {
  const all: Discussion[] = [
    { author: "Steam 玩家 88123", content: "更新之后优化好了很多，帧数稳定，强烈推荐！", sentiment: "positive", posted_at: toShanghaiISO(Date.now() - 2 * 3600_000), source: "steam_reviews" },
    { author: "Steam 玩家 45210", content: "匹配机制有点迷，经常排到代练，希望官方管一管。", sentiment: "negative", posted_at: toShanghaiISO(Date.now() - 5 * 3600_000), source: "steam_reviews" },
    { author: "Steam 玩家 77345", content: "和朋友开黑玩了几十个小时，总体体验还行，中规中矩。", sentiment: "neutral", posted_at: toShanghaiISO(Date.now() - 9 * 3600_000), source: "steam_reviews" },
    { author: "手游玩家_小北", content: "新赛季内容挺丰富的，福利也还可以，继续玩下去。", sentiment: "positive", posted_at: toShanghaiISO(Date.now() - 14 * 3600_000), source: "app_reviews" },
    { author: "Steam 玩家 20981", content: "外挂问题什么时候能解决？排位体验太差了。", sentiment: "negative", posted_at: toShanghaiISO(Date.now() - 20 * 3600_000), source: "steam_reviews" },
    { author: "Steam 玩家 65530", content: "画面和打击感都在线，单人剧情也很扎实，值回票价。", sentiment: "positive", posted_at: toShanghaiISO(Date.now() - 26 * 3600_000), source: "steam_reviews" },
    { author: "路人甲", content: "刚入坑，跟着新手引导玩还挺顺手的，观望一下后续更新。", sentiment: "neutral", posted_at: toShanghaiISO(Date.now() - 32 * 3600_000), source: "app_reviews" },
    { author: "Steam 玩家 90112", content: "服务器晚上高峰期有点卡，其他时间都挺流畅的。", sentiment: "neutral", posted_at: toShanghaiISO(Date.now() - 40 * 3600_000), source: "steam_reviews" },
  ];
  return all.slice(0, Math.max(0, limit));
}

/** POST /api/collect */
export function mockCollect(platform: string, region: string): CollectResult {
  const charts = ["hot/daily", "hot/weekly", "new/daily", "new/weekly"];
  return {
    ok: true,
    results: charts.map((chart) => ({
      platform,
      region,
      chart,
      ok: true,
      count: 50,
      error: "",
    })),
  };
}
