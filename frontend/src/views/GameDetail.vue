<!-- 游戏详情：头部信息 / 排名走势 / 在线人数 / 上榜理由时间线 / 玩家讨论 -->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import {
  fetchGame,
  fetchGameDiscussions,
  fetchGamePlayers,
  fetchGameRanks,
  getErrorMessage,
  type Discussion,
  type GameDetail,
  type PlayerSample,
  type RankHistoryItem,
} from "@/api";
import { createChart } from "@/utils/chart";
import { formatCount, formatDateShort, formatDateTime } from "@/utils/format";
import ReasonBadge from "@/components/ReasonBadge.vue";
import RankChange from "@/components/RankChange.vue";
import GameIcon from "@/components/GameIcon.vue";

const route = useRoute();
/** 路由参数即后端 game_id（如 steam:global:730） */
const gameId = computed(() => String(route.params.id ?? ""));

/* ---------- 头部 ---------- */
const game = ref<GameDetail | null>(null);
const gameLoading = ref(true);
const gameError = ref("");

/** 平台 id -> 中文名（展示用） */
const PLATFORM_NAMES: Record<string, string> = {
  steam: "Steam",
  appstore: "App Store",
  googleplay: "Google Play",
};
const REGION_NAMES: Record<string, string> = { global: "全球", cn: "中国", us: "美国" };

/* ---------- 趋势（排名 / 在线人数），days 可切 7 / 30 ---------- */
const trendDays = ref<7 | 30>(30);
const ranks = ref<RankHistoryItem[]>([]);
const trendsLoading = ref(false);
const trendsError = ref("");
const players = ref<PlayerSample[]>([]);

const rankChartEl = ref<HTMLElement | null>(null);
const playerChartEl = ref<HTMLElement | null>(null);
let disposeRankChart: (() => void) | null = null;
let disposePlayerChart: (() => void) | null = null;

/* ---------- 讨论 ---------- */
const discussions = ref<Discussion[]>([]);
const discussionsLoading = ref(true);

/** 情感 -> 徽标 */
const sentimentMeta: Record<Discussion["sentiment"], { icon: string; text: string; cls: string }> = {
  positive: { icon: "👍", text: "好评", cls: "badge-green" },
  negative: { icon: "👎", text: "差评", cls: "badge-red" },
  neutral: { icon: "😐", text: "中性", cls: "badge-gray" },
};

/** 上榜理由时间线：按日期倒序 */
const timeline = computed(() =>
  [...ranks.value].sort((a, b) => (a.date < b.date ? 1 : -1)),
);

/* ---------- 加载 ---------- */

function disposeCharts(): void {
  disposeRankChart?.();
  disposeRankChart = null;
  disposePlayerChart?.();
  disposePlayerChart = null;
}

/** 头部 + 讨论：随游戏切换加载一次 */
async function loadHeader(): Promise<void> {
  const id = gameId.value;
  gameLoading.value = true;
  gameError.value = "";
  discussionsLoading.value = true;
  try {
    const [g, d] = await Promise.all([fetchGame(id), fetchGameDiscussions(id, 20)]);
    // 防止快速切换游戏时的竞态：只接受最新 id 的结果
    if (gameId.value !== id) return;
    game.value = g;
    discussions.value = d;
  } catch (err) {
    if (gameId.value === id) gameError.value = `游戏信息加载失败：${getErrorMessage(err)}`;
  } finally {
    if (gameId.value === id) {
      gameLoading.value = false;
      discussionsLoading.value = false;
    }
  }
}

/** 趋势数据：随游戏 / 天数切换重新加载并重绘图表 */
async function loadTrends(): Promise<void> {
  const id = gameId.value;
  const days = trendDays.value;
  trendsLoading.value = true;
  trendsError.value = "";
  disposeCharts();
  try {
    const [r, p] = await Promise.all([
      fetchGameRanks(id, days),
      fetchGamePlayers(id, days),
    ]);
    if (gameId.value !== id) return;
    ranks.value = r;
    players.value = p;
    await nextTick();
    renderRankChart();
    renderPlayerChart();
  } catch (err) {
    if (gameId.value === id) trendsError.value = `趋势数据加载失败：${getErrorMessage(err)}`;
  } finally {
    if (gameId.value === id) trendsLoading.value = false;
  }
}

/* ---------- 图表渲染 ---------- */

/** 排名走势：y 轴反转，排名越小越靠上 */
function renderRankChart(): void {
  const el = rankChartEl.value;
  if (!el || ranks.value.length === 0) return;
  const data = [...ranks.value].sort((a, b) => (a.date < b.date ? -1 : a.date > b.date ? 1 : 0));
  const { chart, dispose } = createChart(el);
  disposeRankChart = dispose;
  chart.setOption({
    tooltip: {
      trigger: "axis",
      formatter: (params: unknown) => {
        const p = (params as Array<{ dataIndex: number }>)[0];
        const item = data[p.dataIndex];
        return `${item.date}<br/>排名：第 ${item.rank} 名<br/>${item.reason_text}`;
      },
    },
    grid: { left: 44, right: 16, top: 28, bottom: 30 },
    xAxis: {
      type: "category",
      boundaryGap: false,
      data: data.map((d) => formatDateShort(d.date)),
    },
    yAxis: { type: "value", inverse: true, min: 1, name: "排名" },
    series: [
      {
        name: "排名",
        type: "line",
        smooth: true,
        symbol: "circle",
        symbolSize: 6,
        data: data.map((d) => d.rank),
        lineStyle: { width: 2.5 },
        areaStyle: { opacity: 0.08 },
      },
    ],
  });
}

/** 在线人数曲线：无数据时不渲染（由模板展示提示） */
function renderPlayerChart(): void {
  const el = playerChartEl.value;
  if (!el || players.value.length === 0) return;
  const data = [...players.value].sort((a, b) =>
    a.sampled_at < b.sampled_at ? -1 : a.sampled_at > b.sampled_at ? 1 : 0,
  );
  const { chart, dispose } = createChart(el);
  disposePlayerChart = dispose;
  chart.setOption({
    tooltip: {
      trigger: "axis",
      valueFormatter: (v: unknown) => formatCount(Number(v)),
    },
    grid: { left: 64, right: 16, top: 28, bottom: 30 },
    xAxis: {
      type: "category",
      boundaryGap: false,
      data: data.map((d) => formatDateTime(d.sampled_at)),
      axisLabel: { hideOverlap: true },
    },
    yAxis: { type: "value", name: "在线人数" },
    series: [
      {
        name: "在线人数",
        type: "line",
        smooth: true,
        showSymbol: false,
        data: data.map((d) => d.count),
        lineStyle: { width: 2, color: "#16a34a" },
        areaStyle: { opacity: 0.1, color: "#16a34a" },
      },
    ],
  });
}

/* ---------- 生命周期 ---------- */
watch(gameId, () => {
  void loadHeader();
  void loadTrends();
});
watch(trendDays, () => {
  void loadTrends();
});

onMounted(() => {
  void loadHeader();
  void loadTrends();
});

onBeforeUnmount(() => {
  disposeCharts();
});
</script>

<template>
  <div>
    <router-link to="/" class="muted">← 返回榜单</router-link>

    <!-- 头部 -->
    <div v-if="gameLoading" class="loading card"><span class="spinner"></span>游戏信息加载中…</div>
    <div v-else-if="gameError" class="error-box card">{{ gameError }}</div>
    <div v-else-if="game" class="card">
      <div class="detail-header">
        <GameIcon :src="game.icon_url" :name="game.name" :size="game.platform === 'steam' ? 300 : 72" :wide="game.platform === 'steam'" />
        <div>
          <h1 class="page-title">{{ game.name }}</h1>
          <p class="detail-meta">
            {{ PLATFORM_NAMES[game.platform] ?? game.platform }} ·
            {{ REGION_NAMES[game.region] ?? game.region }}
            <span v-for="t in game.tags ?? []" :key="t" class="tag">{{ t }}</span>
          </p>
          <p v-if="game.price_text" class="price">{{ game.price_text }}</p>
        </div>
      </div>
      <p v-if="game.description" class="desc">{{ game.description }}</p>
    </div>

    <!-- 趋势区 -->
    <div class="card">
      <div class="toolbar">
        <h2 class="page-title">排名走势</h2>
        <span class="spacer"></span>
        <div class="seg" aria-label="天数">
          <button :class="{ active: trendDays === 7 }" @click="trendDays = 7">近 7 天</button>
          <button :class="{ active: trendDays === 30 }" @click="trendDays = 30">近 30 天</button>
        </div>
      </div>

      <div v-if="trendsLoading" class="loading"><span class="spinner"></span>趋势数据加载中…</div>
      <div v-else-if="trendsError" class="error-box">{{ trendsError }}</div>
      <template v-else>
        <div v-if="ranks.length === 0" class="empty">暂无排名历史数据。</div>
        <div v-else ref="rankChartEl" class="chart-box"></div>

        <h2 class="page-title mt-8">在线人数</h2>
        <div v-if="players.length === 0" class="empty">该平台无公开人数数据。</div>
        <div v-else ref="playerChartEl" class="chart-box"></div>
      </template>
    </div>

    <!-- 上榜理由时间线 -->
    <div class="card">
      <h2 class="page-title">上榜理由时间线</h2>
      <div v-if="trendsLoading" class="loading"><span class="spinner"></span>加载中…</div>
      <div v-else-if="timeline.length === 0" class="empty">暂无上榜记录。</div>
      <ul v-else class="timeline">
        <li v-for="r in timeline" :key="r.date">
          <span class="t-date">{{ r.date }}</span>
          <span class="rank-num" style="min-width: 40px">第 {{ r.rank }} 名</span>
          <RankChange :change="r.rank_change" :reason-code="r.reason_code" />
          <ReasonBadge :code="r.reason_code" :text="r.reason_text" />
        </li>
      </ul>
    </div>

    <!-- 玩家讨论 -->
    <div class="card">
      <h2 class="page-title">玩家讨论</h2>
      <div v-if="discussionsLoading" class="loading"><span class="spinner"></span>讨论加载中…</div>
      <div v-else-if="discussions.length === 0" class="empty">暂无玩家讨论数据。</div>
      <div v-else>
        <div v-for="(d, i) in discussions" :key="i" class="discussion">
          <div class="discussion-head">
            <span class="badge" :class="sentimentMeta[d.sentiment].cls">
              {{ sentimentMeta[d.sentiment].icon }} {{ sentimentMeta[d.sentiment].text }}
            </span>
            <span>{{ d.author }}</span>
            <span>·</span>
            <span>{{ formatDateTime(d.posted_at) }}</span>
            <span class="muted">（{{ d.source }}）</span>
          </div>
          <div class="discussion-body">{{ d.content }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
