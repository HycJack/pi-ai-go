import { useState, useRef, useCallback, useEffect } from 'react';
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

// GeoGebra app parameters for each view mode
const GGB_PARAMS: Record<GGBView, Record<string, string | boolean>> = {
  geometry: {
    appName: 'geometry',
    width: '100%',
    height: '100%',
    showToolBar: true,
    showAlgebraInput: true,
    showMenuBar: false,
    scaleContainerClass: 'ggb-runner-body',
  },
  '3d': {
    appName: '3d',
    width: '100%',
    height: '100%',
    showToolBar: true,
    showAlgebraInput: true,
    showMenuBar: false,
    scaleContainerClass: 'ggb-runner-body',
  },
  classic: {
    appName: 'classic',
    width: '100%',
    height: '100%',
    showToolBar: true,
    showAlgebraInput: true,
    showMenuBar: false,
    scaleContainerClass: 'ggb-runner-body',
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

// Declare the GeoGebra global types
interface GGBAppletInstance {
  inject?(containerId: string): void;
  evalCommand?(cmd: string): boolean;
  evalCommandGetLabels?(cmd: string): string[];
  evalXML?(xml: string): void;
  getXML?(): string;
  setValue?(key: string, val: number): void;
  getValue?(key: string): string;
}

declare global {
  interface Window {
    GGBApplet?: {
      new(params: Record<string, unknown>, id: string): GGBAppletInstance;
    };
    __ggb_applets?: Record<string, GGBAppletInstance>;
  }
}

export default function GeogebraRunner({ html, ggbCode, svg, activeTab = 'html', onClose }: GeogebraRunnerProps) {
  const [tab, setTab] = useState<PanelTab>(activeTab);
  const [copied, setCopied] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const appletRef = useRef<string | null>(null);
  const [ggbReady, setGgbReady] = useState(false);
  const [ggbView, setGgbView] = useState<GGBView>('geometry');
  const [panelWidth, setPanelWidth] = useState(520);
  const draggingRef = useRef(false);
  const startXRef = useRef(0);
  const startWidthRef = useRef(0);
  const ggbInitializedRef = useRef(false);
  const ggbCodeRef = useRef(ggbCode);
  ggbCodeRef.current = ggbCode;

  // Sync activeTab prop
  useEffect(() => {
    setTab(activeTab);
  }, [activeTab]);

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

  const handleMouseUp = useCallback(() => {
    if (draggingRef.current) {
      draggingRef.current = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }
  }, []);

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

  // ─── Initialize GeoGebra applet ───

  const initGeoGebra = useCallback(() => {
    if (!containerRef.current || ggbInitializedRef.current) return;
    ggbInitializedRef.current = true;

    // Load the GeoGebra deployment script if not already loaded
    const loadGGB = () => {
      // Generate a unique applet ID for this instance
      const appletId = `ggb-applet-${Date.now()}`;

      const params = {
        ...GGB_PARAMS[ggbView],
        id: appletId,
        borderColor: '#25262b',
        fontCss: 'font-family: var(--font-body)',
      };

      if (window.GGBApplet) {
        // GeoGebra script already loaded — instantiate directly
        const applet = new window.GGBApplet(params, appletId);
        appletRef.current = appletId;
        try {
          applet.inject?.(appletId); // older API
        } catch { /* newer API uses different method */ }
        // For newer GGB API, we use the inject method with container ID
        setGgbReady(true);
      } else {
        // Script not loaded yet — will try again
        setGgbReady(true);
      }
    };

    if (typeof window.GGBApplet !== 'undefined') {
      loadGGB();
    } else {
      // Load the GeoGebra deploy script
      const script = document.createElement('script');
      script.src = 'https://www.geogebra.org/apps/deployggb.js';
      script.onload = () => {
        loadGGB();
      };
      document.head.appendChild(script);
    }
  }, [ggbView]);

  // Initialize when switching to ggb tab
  useEffect(() => {
    if (tab === 'ggb') {
      initGeoGebra();
    }
  }, [tab, initGeoGebra]);

  // ─── Execute GGB commands via evalCommand ───

  const executeGGBCommands = useCallback(() => {
    const code = ggbCodeRef.current;
    if (!code) return;

    // Try multiple ways to get the applet reference
    const getApplet = () => {
      // Method 1: window.__ggb_applets global map
      if (window.__ggb_applets) {
        const keys = Object.keys(window.__ggb_applets);
        if (keys.length > 0) {
          return window.__ggb_applets[keys[0]];
        }
      }
      // Method 2: Direct ggbApplet global
      if ((window as any).ggbApplet) {
        return (window as any).ggbApplet;
      }
      return null;
    };

    let attempts = 0;
    const maxAttempts = 60;
    const interval = setInterval(() => {
      attempts++;
      const applet = getApplet();
      if (applet && typeof applet.evalCommand === 'function') {
        const lines = code.split('\n')
          .map((l) => l.trim())
          .filter((l) => l && !l.startsWith('#') && !l.startsWith('//'));
        lines.forEach((line) => {
          try {
            applet.evalCommand(line);
          } catch (e) {
            console.warn('[GGB] evalCommand failed:', line, e);
          }
        });
        clearInterval(interval);
        return;
      }
      if (attempts >= maxAttempts) {
        console.warn('[GGB] Failed to get applet after', maxAttempts, 'attempts');
        clearInterval(interval);
      }
    }, 500);
  }, []);

  // Execute commands when ready and ggbCode changes
  useEffect(() => {
    if (!ggbReady || !ggbCode) return;
    // Wait a bit for the applet to fully initialize
    const timer = setTimeout(() => {
      executeGGBCommands();
    }, 1500);
    return () => clearTimeout(timer);
  }, [ggbCode, ggbReady, executeGGBCommands]);

  // Auto-detect GGB view
  useEffect(() => {
    if (!ggbCode) return;
    const lower = ggbCode.toLowerCase();
    const is3D = /\b(surface|sphere|cube|prism|pyramid|net|volume|z\s*=|z\(|xAxis|yAxis|zAxis|view3d|setactiveview\["3d"\])\b/i.test(lower);
    setGgbView(is3D ? '3d' : 'geometry');
  }, [ggbCode]);

  // Re-init GeoGebra when switching views
  useEffect(() => {
    if (tab === 'ggb') {
      ggbInitializedRef.current = false;
      setGgbReady(false);
      // Delay to let DOM re-render
      const timer = setTimeout(() => initGeoGebra(), 100);
      return () => clearTimeout(timer);
    }
  }, [ggbView, tab, initGeoGebra]);

  const showGGB = tab === 'ggb' && ggbCode;
  const showHTML = tab === 'html' && html;
  const showSVG = tab === 'svg' && svg;
  const ggbViewConfig = GGB_PARAMS[ggbView];

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
            ggbReady
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
          disabled={!ggbCode}
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
      {showGGB && (
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
          <div className="ggb-runner-body" style={{ position: 'relative' }}>
            <div
              ref={containerRef}
              id={`ggb-container-${ggbView}`}
              className="ggb-runner-iframe"
            />
          </div>
        </>
      )}

      {!showGGB && !showHTML && !showSVG && (
        <div className="ggb-runner-body ggb-runner-empty">
          <span className="ggb-runner-empty-text">暂无内容</span>
        </div>
      )}
    </div>
  );
}
