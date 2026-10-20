<!-- 仪表盘：平台 Tab / 地区 / 每日每周 / 热门新游 / 日期选择 / 榜单表格 / 手动采集 -->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import {
  fetchChart,
  fetchPlatforms,
  getErrorMessage,
  triggerCollect,
  type ChartPeriod,
  type ChartResponse,
  type ChartType,
  type Platform,
} from "@/api";
import { formatCount, todayStr } from "@/utils/format";
import ReasonBadge from "@/components/ReasonBadge.vue";
import RankChange from "@/components/RankChange.vue";
import GameIcon from "@/components/GameIcon.vue";

/* ---------- 状态 ---------- */
const platforms = ref<Platform[]>([]);
const loadingPlatforms = ref(true);
const platformError = ref("");

const platformId = ref("");
const region = ref("");
const chartType = ref<ChartType>("hot");
const period = ref<ChartPeriod>("daily");
// 日期选择：空字符串表示"最新"
const dateStr = ref("");

const chart = ref<ChartResponse | null>(null);
const loadingChart = ref(false);
const chartError = ref("");

const collecting = ref(false);
const collectMsg = ref("");

const today = todayStr();

/* ---------- 派生状态 ---------- */
const currentPlatform = computed<Platform | undefined>(() =>
  platforms.value.find((p) => p.id === platformId.value),
);

/** 是否可切换地区（多地区平台才可切换） */
const regionSwitchable = computed(() => (currentPlatform.value?.regions.length ?? 0) > 1);

const regionLabel = computed(() => {
  const p = currentPlatform.value;
  if (!p) return "";
  return p.region_labels[region.value] ?? region.value;
});

/** 在线人数列：仅 Steam 热门榜显示真实数据 */
const showPlayerCount = computed(() => platformId.value === "steam" && chartType.value === "hot");

/* ---------- 数据加载 ---------- */

/** 加载平台列表，默认选中 Steam */
async function loadPlatforms(): Promise<void> {
  loadingPlatforms.value = true;
  platformError.value = "";
  try {
    platforms.value = await fetchPlatforms();
    if (platforms.value.length > 0 && !platformId.value) {
      selectPlatform(platforms.value[0].id);
    }
  } catch (err) {
    // api/index 已做 Mock 降级，这里只兜底极端异常
    platformError.value = `平台列表加载失败：${getErrorMessage(err)}`;
  } finally {
    loadingPlatforms.value = false;
  }
}

/** 加载榜单 */
async function loadChart(): Promise<void> {
  if (!platformId.value || !region.value) return;
  loadingChart.value = true;
  chartError.value = "";
  try {
    chart.value = await fetchChart({
      platform: platformId.value,
      region: region.value,
      type: chartType.value,
      period: period.value,
      date: dateStr.value || undefined,
    });
  } catch (err) {
    chartError.value = `榜单加载失败：${getErrorMessage(err)}`;
    chart.value = null;
  } finally {
    loadingChart.value = false;
  }
}

/** 切换平台：重置地区为该平台首个地区 */
function selectPlatform(id: string): void {
  platformId.value = id;
  const p = platforms.value.find((x) => x.id === id);
  region.value = p?.regions[0] ?? "";
}

/** 手动触发采集，然后刷新榜单 */
async function handleCollect(): Promise<void> {
  if (!platformId.value || !region.value || collecting.value) return;
  collecting.value = true;
  collectMsg.value = "";
  try {
    const res = await triggerCollect(platformId.value, region.value);
    const okCount = res.results.filter((r) => r.ok).length;
    const total = res.results.length;
    const failMsgs = res.results
      .filter((r) => !r.ok && r.error)
      .map((r) => `${r.chart}: ${r.error}`);
    collectMsg.value =
      okCount === total
        ? `✅ 采集完成：${total} 个榜单全部成功。`
        : `⚠️ 采集完成：${okCount}/${total} 成功。${failMsgs.join("；")}`;
    await loadChart();
  } catch (err) {
    collectMsg.value = `❌ 采集失败：${getErrorMessage(err)}`;
  } finally {
    collecting.value = false;
  }
}

/* ---------- 监听与初始化 ---------- */
watch([platformId, region, chartType, period, dateStr], () => {
  void loadChart();
});

onMounted(() => {
  void loadPlatforms();
});
</script>

<template>
  <div>
    <h1 class="page-title">游戏榜单</h1>
    <p class="page-desc">监控 Steam / App Store / Google Play 的热门榜与新游榜。</p>

    <!-- 平台加载状态 -->
    <div v-if="loadingPlatforms" class="loading"><span class="spinner"></span>平台列表加载中…</div>
    <div v-else-if="platformError" class="error-box">{{ platformError }}</div>

    <template v-else>
      <!-- 平台 Tab（从 /api/platforms 动态渲染） -->
      <div class="toolbar">
        <div class="seg" role="tablist" aria-label="平台">
          <button
            v-for="p in platforms"
            :key="p.id"
            :class="{ active: p.id === platformId }"
            @click="selectPlatform(p.id)"
          >
            {{ p.name }}
          </button>
        </div>
        <span class="spacer"></span>
        <button class="btn btn-primary" :disabled="collecting" @click="handleCollect">
          {{ collecting ? "采集中…" : "🔄 手动采集" }}
        </button>
      </div>

      <!-- 筛选条件 -->
      <div class="toolbar">
        <!-- 地区切换 -->
        <label>地区：</label>
        <div v-if="regionSwitchable" class="seg" aria-label="地区">
          <button
            v-for="r in currentPlatform?.regions ?? []"
            :key="r"
            :class="{ active: r === region }"
            @click="region = r"
          >
            {{ currentPlatform?.region_labels[r] ?? r }}
          </button>
        </div>
        <span v-else class="badge badge-gray">{{ regionLabel }}（固定）</span>

        <!-- 榜单类型 -->
        <label>榜单：</label>
        <div class="seg" aria-label="榜单类型">
          <button :class="{ active: chartType === 'hot' }" @click="chartType = 'hot'">🔥 热门榜</button>
          <button :class="{ active: chartType === 'new' }" @click="chartType = 'new'">✨ 新游榜</button>
        </div>

        <!-- 周期 -->
        <label>周期：</label>
        <div class="seg" aria-label="周期">
          <button :class="{ active: period === 'daily' }" @click="period = 'daily'">每日</button>
          <button :class="{ active: period === 'weekly' }" @click="period = 'weekly'">每周</button>
        </div>

        <!-- 日期 -->
        <label>日期：</label>
        <input v-model="dateStr" type="date" class="input-date" :max="today" />
        <button v-if="dateStr" class="btn" @click="dateStr = ''">回到最新</button>
      </div>

      <div v-if="collectMsg" class="notice notice-info">{{ collectMsg }}</div>

      <!-- 数据来源说明横幅：如实标注非官方/人工录入渠道 -->
      <div v-if="platformId === 'googleplay'" class="notice notice-info">
        📡 Google Play
        榜单来自第三方渠道抓取（非官方），数据可能有延迟；抓取遵守礼貌策略（低频、遇限流即停）。
      </div>

      <!-- 榜单加载状态 -->
      <div v-if="loadingChart" class="loading"><span class="spinner"></span>榜单加载中…</div>
      <div v-else-if="chartError" class="error-box">{{ chartError }}</div>
      <div v-else-if="!chart || chart.entries.length === 0" class="empty">
        该条件下暂无榜单数据，换个日期或榜单类型试试。
      </div>

      <!-- 榜单表格 -->
      <div v-else class="card" style="padding: 8px 12px">
        <p class="muted" style="margin: 8px 4px">
          {{ currentPlatform?.name }} · {{ regionLabel }} ·
          {{ chartType === "hot" ? "热门榜" : "新游榜" }} ·
          {{ period === "daily" ? "每日" : "每周" }} ·
          {{ chart.date }}（共 {{ chart.entries.length }} 款）
        </p>
        <div class="table-wrap">
          <table class="chart-table">
            <thead>
              <tr>
                <th style="width: 64px">排名</th>
                <th style="width: 84px">升降</th>
                <th>游戏</th>
                <th>类型标签</th>
                <th>上榜理由</th>
                <th style="width: 110px">在线人数</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="e in chart.entries" :key="e.game_id">
                <td>
                  <span
                    class="rank-num"
                    :class="{ top1: e.rank === 1, top2: e.rank === 2, top3: e.rank === 3 }"
                  >
                    {{ e.rank }}
                  </span>
                </td>
                <td><RankChange :change="e.rank_change" :reason-code="e.reason_code" /></td>
                <td>
                  <router-link
                    :to="{ name: 'game-detail', params: { id: e.game_id } }"
                    class="game-cell"
                  >
                    <GameIcon :src="e.icon_url" :name="e.game_name" :size="platformId === 'steam' ? 96 : 40" :wide="platformId === 'steam'" />
                    <span class="game-name">{{ e.game_name }}</span>
                  </router-link>
                </td>
                <td>
                  <span v-for="t in e.tags ?? []" :key="t" class="tag">{{ t }}</span>
                  <span v-if="!(e.tags ?? []).length" class="muted">—</span>
                </td>
                <td><ReasonBadge :code="e.reason_code" :text="e.reason_text" /></td>
                <td>{{ showPlayerCount ? formatCount(e.player_count) : "—" }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>
