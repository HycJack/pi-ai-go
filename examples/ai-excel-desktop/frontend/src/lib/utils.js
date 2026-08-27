// Wails 绑定封装
import * as App from "../../wailsjs/go/main/App.js";

export const Api = App;

// 日志（控制台 + toast）
let logCounter = 0;
export function Log(msg, ...args) {
  logCounter++;
  const prefix = `[ai-excel:${logCounter}]`;
  if (args.length > 0) {
    console.log(prefix, msg, ...args);
  } else {
    console.log(prefix, msg);
  }
}

// Toast 工具
export function toast(msg, type = "info") {
  const el = document.createElement("div");
  el.className = `toast ${type}`;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}

// 采样大数据
export function sampleData(data, n) {
  if (data.length <= n) return data;
  const step = Math.floor(data.length / n);
  const out = [];
  for (let i = 0; i < data.length; i += step) out.push(data[i]);
  return out;
}

// 直方图分箱
export function createHistogram(values, binCount) {
  if (values.length === 0) return { labels: [], data: [] };
  const min = Math.min(...values);
  const max = Math.max(...values);
  const step = (max - min) / binCount || 1;
  const bins = new Array(binCount).fill(0);
  const labels = [];
  for (let i = 0; i < binCount; i++) {
    const s = min + i * step;
    const e = min + (i + 1) * step;
    labels.push(`${s.toFixed(1)}-${e.toFixed(1)}`);
  }
  values.forEach((v) => {
    const idx = Math.min(Math.floor((v - min) / step), binCount - 1);
    bins[idx]++;
  });
  return { labels, data: bins };
}
