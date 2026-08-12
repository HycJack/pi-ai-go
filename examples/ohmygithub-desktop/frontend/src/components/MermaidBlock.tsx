import { useEffect, useRef, useState } from 'react';
import mermaid from 'mermaid';

// 当前主题对应的 mermaid 主题名
function currentMermaidTheme(): string {
  return document.documentElement.getAttribute('data-theme') === 'light' ? 'default' : 'dark';
}

// 跟踪已初始化的主题，主题变化时重新 initialize
let initializedTheme = '';
function ensureMermaidInitialized() {
  const theme = currentMermaidTheme();
  if (initializedTheme === theme) return;
  mermaid.initialize({
    startOnLoad: false,
    theme: theme as any,
    securityLevel: 'loose',
    fontFamily: 'inherit',
  });
  initializedTheme = theme;
}

interface MermaidBlockProps {
  chart: string;
}

// MermaidBlock 渲染 mermaid 图表。接收图表源码，输出 SVG。
// 监听 data-theme 属性变化，主题切换时自动重新渲染。
export default function MermaidBlock({ chart }: MermaidBlockProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [svg, setSvg] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [themeVersion, setThemeVersion] = useState(0);

  // 监听主题变化
  useEffect(() => {
    const observer = new MutationObserver(() => {
      setThemeVersion(v => v + 1);
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    });
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    let cancelled = false;
    async function render() {
      ensureMermaidInitialized();
      const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      try {
        const { svg: rendered } = await mermaid.render(id, chart);
        if (!cancelled) {
          setSvg(rendered);
          setError('');
        }
      } catch (e: any) {
        if (!cancelled) {
          setError(e?.message || String(e));
          setSvg('');
        }
      }
    }
    render();
    return () => { cancelled = true; };
  }, [chart, themeVersion]);

  if (error) {
    return (
      <div style={{
        padding: 12, margin: '8px 0', borderRadius: 6,
        background: 'rgba(248,81,73,0.1)', border: '1px solid rgba(248,81,73,0.3)',
        fontSize: 12, color: '#f85149', fontFamily: 'var(--font-mono)',
        overflow: 'auto',
      }}>
        <div style={{ fontWeight: 600, marginBottom: 4 }}>Mermaid 渲染失败</div>
        <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{error}</pre>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="mermaid-container"
      style={{ textAlign: 'center', padding: 12, overflow: 'auto' }}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
