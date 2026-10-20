/**
 * echarts 按需引入与图表实例管理。
 * 只引入折线图需要的模块，减小打包体积；提供统一的创建/销毁封装，
 * 自动处理窗口缩放。
 */
import * as echarts from "echarts/core";
import { LineChart } from "echarts/charts";
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer]);

export { echarts };
export type EChartsInstance = ReturnType<typeof echarts.init>;

/** 创建图表实例，自动监听窗口 resize；调用返回的 dispose 释放资源 */
export function createChart(el: HTMLElement): {
  chart: EChartsInstance;
  dispose: () => void;
} {
  const chart = echarts.init(el);
  const onResize = (): void => chart.resize();
  window.addEventListener("resize", onResize);
  return {
    chart,
    dispose(): void {
      window.removeEventListener("resize", onResize);
      chart.dispose();
    },
  };
}
