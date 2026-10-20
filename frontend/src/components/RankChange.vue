<!-- 排名升降指示：▲N 绿色 / ▼N 红色 / - 灰色 / NEW 蓝色徽标（首次上榜） -->
<script setup lang="ts">
import type { PropType } from "vue";
import type { ReasonCode } from "@/api";

defineProps({
  /** 正数=上升、负数=下跌、0=持平、null=无历史数据 */
  change: { type: [Number, null] as PropType<number | null>, default: null },
  reasonCode: { type: String as PropType<ReasonCode>, required: true },
});
</script>

<template>
  <span v-if="reasonCode === 'new_entry'" class="badge badge-blue">NEW</span>
  <span v-else-if="change === null || change === undefined" class="rank-flat">-</span>
  <span v-else-if="change > 0" class="rank-up">▲{{ change }}</span>
  <span v-else-if="change < 0" class="rank-down">▼{{ -change }}</span>
  <span v-else class="rank-flat">-</span>
</template>
