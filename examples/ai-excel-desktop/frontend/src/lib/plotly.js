// Plotly.js 辅助函数：trace 和 layout 构建（共享给 ChartView 和 ChartBlock）
import Plotly from "plotly.js-dist-min";

export const DARK_BG = "rgba(0,0,0,0)";
export const FG = "rgba(226,232,240,0.85)";
export const FG_MUTED = "rgba(148,163,184,0.7)";
export const GRID = "rgba(59,63,111,0.4)";
export const ZERO = "rgba(99,102,241,0.3)";
export const COLORS = ["#6366f1","#a855f7","#ec4899","#22c55e","#f59e0b","#3b82f6","#14b8a6","#ef4444"];
export const PIE_COLORS = ["#6366f1","#a855f7","#ec4899","#22c55e","#f59e0b","#ef4444","#3b82f6","#14b8a6"];

export function buildChartTraces(config, data) {
  if (!config || !data?.length) return [];
  const { type, xAxis, yAxes } = config;
  const labels = data.map((r) => r[xAxis]);
  const traces = [];

  if (type === "scatter3d") {
    const xCol = xAxis;
    const yCol = yAxes?.[0] || "";
    const zCol = yAxes?.[1] || config.zAxis || "";
    const xs = []; const ys = []; const zs = []; const hts = [];
    for (const row of data) {
      const x = parseFloat(row[xCol]);
      const y = parseFloat(row[yCol]);
      const z = parseFloat(row[zCol]);
      if (!isNaN(x) && !isNaN(y) && !isNaN(z)) {
        xs.push(x); ys.push(y); zs.push(z);
        hts.push(`${xCol}=${x.toFixed(2)}<br />${yCol}=${y.toFixed(2)}<br />${zCol}=${z.toFixed(2)}`);
      }
    }
    traces.push({
      type: "scatter3d", mode: "markers",
      x: xs, y: ys, z: zs, text: hts, hoverinfo: "text",
      marker: {
        size: Math.max(2, Math.min(8, 40 / Math.sqrt(xs.length))),
        color: ys,
        colorscale: [[0,"#6366f1"],[0.5,"#a855f7"],[1,"#ec4899"]],
        colorbar: { title: { text: yCol, font: { size: 10, color: FG_MUTED } }, tickfont: { size: 9, color: FG_MUTED } },
        opacity: 0.8,
        line: { color: "rgba(255,255,255,0.1)", width: 0.5 },
      },
    });
    return traces;
  }

  if (type === "histogram") {
    const values = data.map((r) => parseFloat(r[yAxes[0]]) || 0);
    traces.push({
      type: "histogram", x: values, nbinsx: config.bin || 20,
      name: (yAxes[0] || "") + " 分布",
      marker: { color: "#6366f1", line: { color: "#4f46e5", width: 1 } },
      autobinx: false,
      xbins: {
        start: Math.min(...values),
        end: Math.max(...values),
        size: (Math.max(...values) - Math.min(...values)) / (config.bin || 20) || 1,
      },
    });
    return traces;
  }

  if (type === "pie") {
    const groups = {};
    data.forEach((r) => {
      const k = r[xAxis];
      const v = parseFloat(r[yAxes[0]]) || 0;
      groups[k] = (groups[k] || 0) + v;
    });
    const keys = Object.keys(groups).slice(0, 12);
    traces.push({
      type: "pie", labels: keys, values: keys.map((k) => groups[k]),
      marker: { colors: PIE_COLORS, line: { color: "rgba(0,0,0,0.3)", width: 1 } },
      textinfo: "label+percent", textfont: { size: 11, color: FG },
      hoverinfo: "label+value+percent", automargin: true,
    });
    return traces;
  }

  // bar / line / area / scatter
  yAxes.forEach((yCol, i) => {
    const c = COLORS[i % COLORS.length];
    const y = data.map((r) => parseFloat(r[yCol]) || 0);
    if (type === "bar") {
      traces.push({ type: "bar", x: labels, y, name: yCol, marker: { color: c, line: { color: "rgba(255,255,255,0.1)", width: 0.5 } } });
    } else if (type === "area") {
      traces.push({ type: "scatter", mode: "lines+markers", x: labels, y, name: yCol, fill: "tozeroy", line: { color: c, width: 2, shape: "spline" }, marker: { color: c, size: 4 } });
    } else if (type === "line") {
      traces.push({ type: "scatter", mode: "lines+markers", x: labels, y, name: yCol, line: { color: c, width: 2, shape: "spline" }, marker: { color: c, size: 4 } });
    } else {
      traces.push({ type: "scatter", mode: "markers", x: labels, y, name: yCol, marker: { color: c, size: 6, opacity: 0.7, line: { color: "rgba(255,255,255,0.15)", width: 0.5 } } });
    }
  });
  return traces;
}

export function buildChartLayout(config, isFull, height) {
  if (config?.type === "scatter3d") {
    const xCol = config.xAxis;
    const yCol = config.yAxes?.[0] || "";
    const zCol = config.yAxes?.[1] || config.zAxis || "";
    return {
      paper_bgcolor: DARK_BG, plot_bgcolor: DARK_BG,
      font: { color: FG, family: "Inter, sans-serif", size: 11 },
      margin: { l: 10, r: 10, b: 10, t: config.title ? 40 : 10, pad: 4 },
      title: config.title ? { text: config.title, font: { size: 13, color: FG } } : undefined,
      height: isFull ? undefined : height,
      scene: {
        bgcolor: DARK_BG,
        xaxis: { title: xCol, gridcolor: GRID, zerolinecolor: ZERO, color: FG_MUTED },
        yaxis: { title: yCol, gridcolor: GRID, zerolinecolor: ZERO, color: FG_MUTED },
        zaxis: { title: zCol, gridcolor: GRID, zerolinecolor: ZERO, color: FG_MUTED },
        camera: { eye: { x: 1.5, y: 1.5, z: 1.2 } },
      },
    };
  }

  const isPie = config?.type === "pie";
  return {
    paper_bgcolor: DARK_BG, plot_bgcolor: DARK_BG,
    font: { color: FG, family: "Inter, sans-serif", size: 11 },
    margin: isPie ? { l: 10, r: 10, b: 10, t: config.title ? 40 : 10, pad: 4 }
                  : { l: 50, r: 20, b: 45, t: 40, pad: 4 },
    title: config?.title ? { text: config.title, font: { size: 13, color: FG } } : undefined,
    showlegend: config?.yAxes?.length > 1 || isPie,
    xaxis: isPie ? undefined : { gridcolor: GRID, zerolinecolor: ZERO, tickfont: { size: 10, color: FG_MUTED }, title: { font: { size: 11, color: FG } } },
    yaxis: isPie ? undefined : { gridcolor: GRID, zerolinecolor: ZERO, tickfont: { size: 10, color: FG_MUTED }, title: { font: { size: 11, color: FG } } },
    height: isFull ? undefined : height,
  };
}
