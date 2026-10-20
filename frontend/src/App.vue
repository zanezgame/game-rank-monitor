<!-- 应用根组件：顶栏导航（榜单 / 关于）+ 路由出口 + 全局 Mock 提示 -->
<script setup lang="ts">
import { isMockMode, mockNotice } from "@/api";
</script>

<template>
  <header class="topbar">
    <div class="topbar-inner">
      <span class="brand">🎮 游戏榜单监控平台</span>
      <nav class="nav">
        <router-link to="/">榜单</router-link>
        <router-link to="/about">关于</router-link>
      </nav>
    </div>
  </header>

  <!-- Mock 模式全局提示：后端未连接时显示示例数据横幅 -->
  <div v-if="isMockMode" class="container" style="padding-bottom: 0">
    <div class="notice notice-warn">⚠️ {{ mockNotice }}</div>
  </div>

  <main class="container">
    <router-view />
  </main>
</template>
