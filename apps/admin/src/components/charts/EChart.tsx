import { useEffect, useRef } from "react";
import * as echarts from "echarts/core";
import { BarChart, LineChart, PieChart } from "echarts/charts";
import { GridComponent, LegendComponent, TitleComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import type { ComposeOption } from "echarts/core";
import type { BarSeriesOption, LineSeriesOption, PieSeriesOption } from "echarts/charts";
import type {
  GridComponentOption,
  LegendComponentOption,
  TitleComponentOption,
  TooltipComponentOption,
} from "echarts/components";

echarts.use([
  BarChart,
  LineChart,
  PieChart,
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
  CanvasRenderer,
]);

export type EChartOption = ComposeOption<
  | BarSeriesOption
  | LineSeriesOption
  | PieSeriesOption
  | GridComponentOption
  | LegendComponentOption
  | TitleComponentOption
  | TooltipComponentOption
>;

/** 与 Arco 品牌色对齐的默认系列色板,跨图表保持一致。 */
export const CHART_COLORS = ["#165dff", "#14c9c9", "#f7ba1e", "#722ed1", "#00b42a", "#f53f3f", "#ff7d00"];

export const CHART_TEXT = "#4e5969";
export const CHART_LINE = "#e5e6eb";

type Props = {
  option: EChartOption;
  height?: number;
  className?: string;
};

/**
 * 轻量 ECharts 容器:自动初始化/销毁,容器尺寸变化时自适应。
 * 与业务无关,任何项目可直接复用。
 */
export function EChart({ option, height = 280, className }: Props) {
  const hostRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<echarts.ECharts | null>(null);

  useEffect(() => {
    if (!hostRef.current) return;
    const chart = echarts.init(hostRef.current);
    chartRef.current = chart;
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(hostRef.current);
    return () => {
      observer.disconnect();
      chart.dispose();
      chartRef.current = null;
    };
  }, []);

  useEffect(() => {
    chartRef.current?.setOption({ color: CHART_COLORS, ...option }, true);
  }, [option]);

  return <div ref={hostRef} className={className} style={{ width: "100%", height }} />;
}
