<!-- 游戏图标：有 icon_url 则显示图片，否则按名称生成首字头像 -->
<script setup lang="ts">
import { computed } from "vue";

const props = defineProps({
  src: { type: String, default: "" },
  name: { type: String, required: true },
  /** 边长（px）；wide 时表示宽度 */
  size: { type: Number, default: 40 },
  /** 宽幅模式：按 Steam 头图 460×215 的原始比例显示，避免正方形裁切把图切碎 */
  wide: { type: Boolean, default: false },
});

// 显示尺寸：wide 时高度按头图比例（215/460）换算
const boxW = computed(() => props.size);
const boxH = computed(() => (props.wide ? Math.round((props.size * 215) / 460) : props.size));

// 外部图片一律经后端 /api/image 同源代理，避免 CDN 防盗链/跨域/网络受限时裂图
const imgSrc = computed(() => {
  const s = props.src;
  if (!s) return "";
  if (s.startsWith("http://") || s.startsWith("https://")) {
    return `/api/image?url=${encodeURIComponent(s)}`;
  }
  return s;
});

// 无图标时：取名称首字，按名称哈希选一个柔和的背景色
const initial = computed(() => (props.name || "?").trim().charAt(0) || "?");
const bgColor = computed(() => {
  let h = 0;
  for (const ch of props.name) h = (h * 31 + ch.charCodeAt(0)) % 360;
  return `hsl(${h}, 55%, 88%)`;
});
const fgColor = computed(() => {
  let h = 0;
  for (const ch of props.name) h = (h * 31 + ch.charCodeAt(0)) % 360;
  return `hsl(${h}, 45%, 32%)`;
});
const boxStyle = computed(() => ({
  width: `${boxW.value}px`,
  height: `${boxH.value}px`,
  fontSize: `${Math.round(boxH.value * 0.42)}px`,
  background: bgColor.value,
  color: fgColor.value,
}));
</script>

<template>
  <img
    v-if="imgSrc"
    :src="imgSrc"
    :alt="name"
    :width="boxW"
    :height="boxH"
    class="game-icon"
    loading="lazy"
  />
  <span v-else class="game-icon game-icon-fallback" :style="boxStyle" :title="name">
    {{ initial }}
  </span>
</template>

<style scoped>
.game-icon {
  border-radius: 8px;
  object-fit: cover;
  flex-shrink: 0;
  display: block;
}
.game-icon-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  border-radius: 8px;
  flex-shrink: 0;
}
</style>
