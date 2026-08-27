import { useState, useRef, useCallback, useEffect, useId } from 'react';
import { X, ChevronRight, Play, Grid3X3, Sigma, Box, Eye, Download, Copy, Check, Image as ImageIcon } from 'lucide-react';

type PanelTab = 'html' | 'ggb' | 'svg';

interface GeogebraRunnerProps {
  html?: string;
  ggbCode?: string;
  svg?: string;
  activeTab?: PanelTab;
  onClose: () => void;
}

type GGBView = 'geometry' | '3d' | 'classic';

const GGB_PARAMS: Record<GGBView, Record<string, string | boolean>> = {
  geometry: {
    appName: 'geometry',
    showToolBar: true,
    showAlgebraInput: true,
    showMenuBar: false,
  },
  '3d': {
    appName: '3d',
    showToolBar: true,
    showAlgebraInput: true,
    showMenuBar: false,
  },
  classic: {
    appName: 'classic',
    showToolBar: true,
    showAlgebraInput: true,
    showMenuBar: false,
  },
};

function saveFile(content: string, ext: string) {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `geogebra-${Date.now()}.${ext}`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

interface GGBAppletInstance {
  evalCommand?(cmd: string): boolean;
  evalXML?(xml: string): void;
  getXML?(): string;
  setValue?(key: string, val: number): void;
  getValue?(key: string): string;
  setSize?(width: number, height: number): void;
  setWidth?(width: number): void;
  setHeight?(height: number): void;
}

interface GGBAppletConstructor {
  new(params: Record<string, unknown>): GGBAppletInstance & {
    inject(containerId: string): void;
  };
}

declare global {
  interface Window {
    GGBApplet?: GGBAppletConstructor;
    ggbApplet?: GGBAppletInstance;
    __ggb_applets?: Record<string, GGBAppletInstance>;
  }
}

export default function GeogebraRunner({ html, ggbCode, svg, activeTab = 'html', onClose }: GeogebraRunnerProps) {
  const [tab, setTab] = useState<PanelTab>(activeTab);
  const [copied, setCopied] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const appletRef = useRef<GGBAppletInstance | null>(null);
  const [ggbView, setGgbView] = useState<GGBView>('geometry');
  const [ggbLoaded, setGgbLoaded] = useState(false);
  const ggbCodeRef = useRef(ggbCode);
  ggbCodeRef.current = ggbCode;
  const [panelWidth, setPanelWidth] = useState(520);
  const draggingRef = useRef(false);
  const startXRef = useRef(0);
  const startWidthRef = useRef(0);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);
  const ggbInjectedRef = useRef(false);
  const appletId = useId();
  const resizeTimeoutRef = useRef<number | null>(null);

  // Sync activeTab prop
  useEffect(() => {
    setTab(activeTab);
  }, [activeTab]);

  // Auto-detect GGB view
  useEffect(() => {
    if (!ggbCode) return;
    const lower = ggbCode.toLowerCase();
    const is3D = /\b(surface|sphere|cube|prism|pyramid|net|volume|z\s*=|z\(|xAxis|yAxis|zAxis|view3d|setactiveview\["3d"\])\b/i.test(lower);
    setGgbView(is3D ? '3d' : 'geometry');
  }, [ggbCode]);

  // ─── Drag handlers ───

  const handleDividerMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    draggingRef.current = true;
    startXRef.current = e.clientX;
    startWidthRef.current = panelWidth;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }, [panelWidth]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!draggingRef.current) return;
    const delta = startXRef.current - e.clientX;
    const newWidth = Math.max(360, Math.min(1000, startWidthRef.current + delta));
    setPanelWidth(newWidth);
  }, []);

  // ─── Resize handler using ResizeObserver ───

  const resizeApplet = useCallback((width: number, height: number) => {
    if (!appletRef.current) {
      console.log('[GGB] resizeApplet: no applet');
      return;
    }

    const w = Math.round(width);
    const h = Math.round(height);
    console.log('[GGB] resizeApplet:', w, h);

    // Try all available resize methods
    if (typeof appletRef.current.setSize === 'function') {
      appletRef.current.setSize(w, h);
    }
    if (typeof appletRef.current.setWidth === 'function') {
      appletRef.current.setWidth(w);
    }
    if (typeof appletRef.current.setHeight === 'function') {
      appletRef.current.setHeight(h);
    }

    // Fallback: directly resize GeoGebra's internal elements
    // Clear previous pending resize to avoid stale callbacks
    if (resizeTimeoutRef.current !== null) {
      window.clearTimeout(resizeTimeoutRef.current);
      resizeTimeoutRef.current = null;
    }
    
    const timeoutId = window.setTimeout(() => {
      if (!containerRef.current) return;
      
      // Try to find iframe(s) inside container
      const iframes = containerRef.current.querySelectorAll('iframe');
      iframes.forEach((iframe, i) => {
        iframe.style.width = w + 'px';
        iframe.style.height = h + 'px';
        iframe.style.minWidth = w + 'px';
        iframe.style.minHeight = h + 'px';
        console.log(`[GGB] fallback iframe[${i}] resize:`, w, h);
      });
      
      // Also try to resize the container itself
      containerRef.current.style.width = w + 'px';
      containerRef.current.style.height = h + 'px';
      containerRef.current.style.minHeight = h + 'px';
      console.log('[GGB] fallback container resize:', w, h);
      
      resizeTimeoutRef.current = null;
    }, 50);
    
    resizeTimeoutRef.current = timeoutId;
  }, []);

  const handleMouseUp = useCallback(() => {
    if (draggingRef.current) {
      draggingRef.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';

      // 拖动结束后强制重新调整 GeoGebra 尺寸
      requestAnimationFrame(() => {
        if (!containerRef.current || !appletRef.current) return;
        const parent = containerRef.current.parentElement;
        if (!parent) return;

        const rect = parent.getBoundingClientRect();
        resizeApplet(rect.width, rect.height);
      });
    }
  }, [resizeApplet]);

  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [handleMouseMove, handleMouseUp]);

  // ─── Inject GeoGebra applet ───

  const injectGGB = useCallback(() => {
    if (!containerRef.current) return;

    const container = containerRef.current;
    
    // Check if already injected
    if (container.getAttribute('data-injected') === 'yes') {
      return;
    }

    container.innerHTML = '';
    container.setAttribute('data-injected', 'yes');

    const params = {
      ...GGB_PARAMS[ggbView],
      id: `ggb-applet-${ggbView}-${Date.now()}`,
      width: container.clientWidth,
      height: container.clientHeight,
      scaleContainerClass: 'ggb-runner-iframe',
      borderColor: '#ddd',
      showLogging: false,
      allowStyleBar: true,
      appletOnLoad: (api: any) => {
        appletRef.current = api;
        setGgbLoaded(true);
      }
    };

    const script = document.createElement('script');
    script.src = '/GeoGebra/deployggb.js';
    script.onload = () => {
      if (typeof window.GGBApplet !== 'undefined') {
        // @ts-ignore - second argument enables legacy API with setSize
        const applet = new window.GGBApplet(params, true);
        applet.inject(appletId);
      }
    };
    
    // Prevent duplicate loading
    const existingScript = document.querySelector('script[src="/GeoGebra/deployggb.js"]') || document.querySelector('script[src="/deployggb.js"]') || document.querySelector('script[src="https://cdn.geogebra.org/apps/deployggb.js"]');
    if (!existingScript) {
      document.head.appendChild(script);
    } else {
      // Script already loaded, just inject
      if (typeof window.GGBApplet !== 'undefined') {
        // @ts-ignore - second argument enables legacy API with setSize
        const applet = new window.GGBApplet(params, true);
        applet.inject(appletId);
      }
    }
  }, [ggbView, appletId]);

  // Setup ResizeObserver when applet is loaded
  useEffect(() => {
    if (!ggbLoaded || !containerRef.current) return;

    const parent = containerRef.current.parentElement;
    if (!parent) return;

    const resizeObserver = new ResizeObserver((entries) => {
      if (!appletRef.current) return;

      for (const entry of entries) {
        const rect = parent.getBoundingClientRect();
        const width = rect.width;
        const height = rect.height;
        
        if (width > 0 && height > 0) {
          resizeApplet(width, height);
        }
      }
    });

    resizeObserver.observe(parent);
    resizeObserverRef.current = resizeObserver;

    return () => {
      resizeObserver.disconnect();
      resizeObserverRef.current = null;
    };
  }, [ggbLoaded, resizeApplet]);

  // Listen to window resize as fallback
  useEffect(() => {
    if (!ggbLoaded) return;

    const handleResize = () => {
      if (!containerRef.current || !appletRef.current) return;
      const parent = containerRef.current.parentElement;
      if (!parent) return;

      requestAnimationFrame(() => {
        const rect = parent.getBoundingClientRect();
        const cw = parent.clientWidth;
        const ch = parent.clientHeight;
        console.log('[GGB] window resize:', { rectW: rect.width, rectH: rect.height, clientW: cw, clientH: ch });
        if (rect.width > 0 && rect.height > 0) {
          resizeApplet(rect.width, rect.height);
        }
      });
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [ggbLoaded, resizeApplet]);

  // Cleanup resize timeout on unmount
  useEffect(() => {
    return () => {
      if (resizeTimeoutRef.current !== null) {
        window.clearTimeout(resizeTimeoutRef.current);
        resizeTimeoutRef.current = null;
      }
    };
  }, []);

  // ─── Execute GGB commands ───

  const executeGGBCommands = useCallback(() => {
    const code = ggbCodeRef.current;
    if (!code) return;

    const getApplet = (): GGBAppletInstance | null => {
      if (appletRef.current && typeof appletRef.current.evalCommand === 'function') {
        return appletRef.current;
      }
      return null;
    };

    let attempts = 0;
    const maxAttempts = 30;
    const interval = setInterval(() => {
      attempts++;
      const applet = getApplet();
      if (applet) {
        const cleaned = code.split('\n')
          .map((l) => l.trim())
          .filter((l) => l && !l.startsWith('#') && !l.startsWith('//'))
          .join('\n');
        if (cleaned) {
          try {
            if (typeof applet.evalCommand === 'function') {
              applet.evalCommand(cleaned);
            }
          } catch (e) {
            console.warn('[GGB] evalCommand failed:', e);
          }
        }
        clearInterval(interval);
        return;
      }
      if (attempts >= maxAttempts) {
        clearInterval(interval);
      }
    }, 500);
  }, []);

  // Inject when switching to ggb tab
  useEffect(() => {
    if (tab === 'ggb') {
      // Reset injection state when switching to ggb tab
      if (containerRef.current) {
        containerRef.current.setAttribute('data-injected', 'no');
      }
      ggbInjectedRef.current = false;
      setGgbLoaded(false);
      const timer = setTimeout(() => injectGGB(), 200);
      return () => clearTimeout(timer);
    }
  }, [tab, ggbView, injectGGB]);

  // Execute commands after applet is loaded
  useEffect(() => {
    if (!ggbLoaded || !ggbCode) return;
    const timer = setTimeout(() => executeGGBCommands(), 500);
    return () => clearTimeout(timer);
  }, [ggbCode, ggbLoaded, executeGGBCommands]);

  const showGGB = tab === 'ggb' && ggbCode;
  const showGGBPanel = tab === 'ggb';
  const showHTML = tab === 'html' && html;
  const showSVG = tab === 'svg' && svg;

  const currentContent = tab === 'html' ? html : tab === 'svg' ? svg : '';

  return (
    <div className="ggb-runner" style={{ width: panelWidth, minWidth: panelWidth }}>
      {/* Draggable divider */}
      <div className="ggb-divider" onMouseDown={handleDividerMouseDown}>
        <ChevronRight size={12} />
      </div>

      {/* Header */}
      <div className="ggb-runner-header">
        <div className="ggb-runner-header-left">
          {tab === 'html' ? <Eye size={14} /> : tab === 'svg' ? <ImageIcon size={14} /> : <Play size={14} />}
          <span>{tab === 'html' ? 'HTML 预览' : tab === 'svg' ? 'SVG 预览' : 'GeoGebra 运行器'}</span>
          {tab === 'ggb' && (
            ggbLoaded
              ? <span className="ggb-runner-status ready">就绪</span>
              : <span className="ggb-runner-status loading">初始化中...</span>
          )}
        </div>
        <div className="ggb-runner-header-actions">
          {(tab === 'html' || tab === 'svg') && currentContent && (
            <>
              <button
                className="ggb-runner-action-btn"
                onClick={() => saveFile(currentContent, tab === 'svg' ? 'svg' : 'html')}
                title="保存文件"
              >
                <Download size={14} />
              </button>
              <button
                className="ggb-runner-action-btn"
                onClick={() => { navigator.clipboard.writeText(currentContent); setCopied(true); setTimeout(() => setCopied(false), 1500); }}
                title="复制"
              >
                {copied ? <Check size={14} /> : <Copy size={14} />}
              </button>
            </>
          )}
          <button className="ggb-runner-close" onClick={onClose}>
            <X size={16} />
          </button>
        </div>
      </div>

      {/* Tab switcher */}
      <div className="ggb-runner-tabs">
        <button
          className={`ggb-runner-tab ${tab === 'svg' ? 'active' : ''}`}
          onClick={() => setTab('svg')}
          disabled={!svg}
        >
          <ImageIcon size={14} />
          <span>SVG</span>
        </button>
        <button
          className={`ggb-runner-tab ${tab === 'html' ? 'active' : ''}`}
          onClick={() => setTab('html')}
          disabled={!html}
        >
          <Eye size={14} />
          <span>HTML 预览</span>
        </button>
        <button
          className={`ggb-runner-tab ${tab === 'ggb' ? 'active' : ''}`}
          onClick={() => setTab('ggb')}
        >
          <Play size={14} />
          <span>GeoGebra</span>
        </button>
      </div>

      {/* SVG Preview Tab */}
      {showSVG && (
        <div className="ggb-runner-body">
          <div className="ggb-runner-svg-preview"
            dangerouslySetInnerHTML={{ __html: svg }}
          />
        </div>
      )}

      {/* HTML Preview Tab */}
      {showHTML && (
        <div className="ggb-runner-body">
          <iframe
            className="ggb-runner-iframe"
            srcDoc={html}
            title="HTML Preview"
            sandbox="allow-scripts allow-same-origin allow-forms"
          />
        </div>
      )}

      {/* GGB Runner Tab */}
      {showGGBPanel && (
        <>
          <div className="ggb-runner-subtabs">
            {(Object.keys(GGB_PARAMS) as GGBView[]).map((key) => (
              <button
                key={key}
                className={`ggb-runner-subtab ${ggbView === key ? 'active' : ''}`}
                onClick={() => setGgbView(key)}
              >
                {key === 'geometry' ? <Grid3X3 size={14} /> : key === '3d' ? <Box size={14} /> : <Sigma size={14} />}
                <span>{key === 'geometry' ? '2D 几何' : key === '3d' ? '3D 绘图' : 'Classic'}</span>
              </button>
            ))}
          </div>
          <div className="ggb-runner-body" style={{ position: 'relative', overflow: 'hidden' }}>
            <div
              ref={containerRef}
              id={appletId}
              className="ggb-runner-iframe"
            />
          </div>
        </>
      )}

      {!showGGBPanel && !showHTML && !showSVG && (
        <div className="ggb-runner-body ggb-runner-empty">
          <span className="ggb-runner-empty-text">暂无内容</span>
        </div>
      )}
    </div>
  );
}
