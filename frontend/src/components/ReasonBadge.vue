<!-- 上榜理由徽标：按 reason_code 显示不同颜色，文案优先使用后端返回的 reason_text -->
<script setup lang="ts">
import type { PropType } from "vue";
import type { ReasonCode } from "@/api";
import { REASON_TEXT } from "@/utils/format";

defineProps({
  /** 上榜理由码：new_entry / re_entry / surge / drop / stable */
  code: { type: String as PropType<ReasonCode>, required: true },
  /** 后端返回的理由文案；缺省时用本地文案兜底 */
  text: { type: String, default: "" },
});

// 理由码 -> 徽标配色
const colorClass: Record<ReasonCode, string> = {
  new_entry: "badge-blue", // 首次上榜：蓝
  re_entry: "badge-purple", // 重回榜单：紫
  surge: "badge-orange", // 大幅上升：橙
  drop: "badge-red", // 明显下跌：红
  stable: "badge-gray", // 稳定：灰
};
</script>

<template>
  <span class="badge" :class="colorClass[code]">{{ text || REASON_TEXT[code] }}</span>
</template>
