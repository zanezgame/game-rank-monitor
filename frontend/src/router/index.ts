import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";

// 路由表：仪表盘 / 游戏详情（id 即后端 game_id，如 steam:global:730） / 关于
const routes: RouteRecordRaw[] = [
  {
    path: "/",
    name: "dashboard",
    component: () => import("@/views/Dashboard.vue"),
    meta: { title: "榜单" },
  },
  {
    path: "/games/:id",
    name: "game-detail",
    component: () => import("@/views/GameDetail.vue"),
    meta: { title: "游戏详情" },
  },
  {
    path: "/about",
    name: "about",
    component: () => import("@/views/About.vue"),
    meta: { title: "关于 / 数据说明" },
  },
  // 未知路径回到仪表盘
  { path: "/:pathMatch(.*)*", redirect: "/" },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

// 切换页面时同步浏览器标题
router.afterEach((to) => {
  const title = typeof to.meta.title === "string" ? to.meta.title : "";
  document.title = title ? `${title} - 游戏榜单监控平台` : "游戏榜单监控平台";
});

export default router;
