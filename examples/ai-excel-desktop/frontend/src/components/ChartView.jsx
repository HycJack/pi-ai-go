// Plotly.js 图表封装（统一渲染：bar/line/pie/scatter/histogram/area + scatter3d）
import { useEffect, useRef } from "react";
import Plotly from "plotly.js-dist-min";
import { BarChart3 } from "lucide-react";
import { buildChartTraces, buildChartLayout } from "../lib/plotly";

const PLOTLY_CONFIG = {
  responsive: true,
  displayModeBar: false,
  scrollZoom: false,
};

export function ChartView({ config, data }) {
  const containerRef = useRef(null);

  useEffect(() => {
    if (!containerRef.current || !config || !data?.length) return;

    const container = containerRef.current;
    const plotId = `chart-main-${Math.random().toString(36).slice(2, 8)}`;
    container.id = plotId;

    const traces = buildChartTraces(config, data);
    const layout = buildChartLayout(config, false, undefined);

    Plotly.newPlot(plotId, traces, layout, PLOTLY_CONFIG);

    return () => {
      try { Plotly.purge(plotId); } catch {}
    };
  }, [config, data]);

  if (!config) {
    return (
      <div className="flex h-full items-center justify-center text-foreground-muted">
        <div className="text-center">
          <div className="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-subtle">
            <BarChart3 className="h-6 w-6 text-primary" />
          </div>
          <p className="text-xs">图表将在此处显示</p>
          <p className="mt-1 text-[11px] text-foreground-muted/70">在右侧 AI 助手输入指令生成</p>
        </div>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="h-full w-full" style={{ minHeight: 300 }} />
  );
}
