import { useRef, useEffect, useState, useImperativeHandle, forwardRef, useCallback } from 'react';

export interface GeoGebraRef {
  executeCommand: (cmd: string) => void;
  setPerspective: (p: string) => void;
  setSize: (width: number, height: number) => void;
  reset: () => void;
  getAPI: () => any;
}

interface GeoGebraProps {
  perspective?: string;
  onPerspectiveChange?: (p: string) => void;
  onReady?: (api: any) => void;
}

const GeoGebraWorkspace = forwardRef<GeoGebraRef, GeoGebraProps>(({ perspective = '2', onPerspectiveChange, onReady }, ref) => {
  // Keep a stable ref to onReady so initGeoGebra/rebuildApplet references
  // remain stable across re-renders that aren't related to perspective changes.
  const onReadyRef = useRef(onReady);
  onReadyRef.current = onReady;
  const onReadyStable = useCallback((api: any) => onReadyRef.current?.(api), []);
  const containerIdRef = useRef(`geogebra-${Math.random().toString(36).slice(2, 9)}`);
  const containerId = containerIdRef.current;
  const containerRef = useRef<HTMLDivElement>(null);
  const ggbRef = useRef<any>(null);

  const [loadError, setLoadError] = useState<string | null>(null);
  const retryCountRef = useRef(0);
  const maxRetries = 3;

  // View visibility states
  const [showMenuBar, setShowMenuBar] = useState(false);
  const [showToolBar, setShowToolBar] = useState(true);
  const [showAlgebraInput, setShowAlgebraInput] = useState(true);
  const [axesVisible, setAxesVisible] = useState(true);
  const [gridVisible, setGridVisible] = useState(true);

  const toggleMenuBar = useCallback(() => {
    setShowMenuBar(prev => {
      const next = !prev;
      const api = ggbRef.current;
      if (api?.showMenuBar) {
        try { api.showMenuBar(next); } catch {}
      }
      return next;
    });
  }, []);

  const toggleToolBar = useCallback(() => {
    setShowToolBar(prev => {
      const next = !prev;
      const api = ggbRef.current;
      if (api?.showToolBar) {
        try { api.showToolBar(next); } catch {}
      }
      return next;
    });
  }, []);

  const toggleAlgebraInput = useCallback(() => {
    setShowAlgebraInput(prev => {
      const next = !prev;
      const api = ggbRef.current;
      if (api?.showAlgebraInput) {
        try { api.showAlgebraInput(next); } catch {}
      }
      return next;
    });
  }, []);

  const toggleAxes = useCallback(() => {
    setAxesVisible(prev => {
      const next = !prev;
      const api = ggbRef.current;
      if (api?.setAxesVisible) {
        try { api.setAxesVisible(next, next, next); } catch {}
      }
      return next;
    });
  }, []);

  const toggleGrid = useCallback(() => {
    setGridVisible(prev => {
      const next = !prev;
      const api = ggbRef.current;
      if (api?.setGridVisible) {
        try { api.setGridVisible(next); } catch {}
      }
      return next;
    });
  }, []);

  const exportGGB = useCallback(() => {
    const api = ggbRef.current;
    if (!api) return;
    try {
      const base64 = api.getBase64?.();
      if (base64) {
        const link = document.createElement('a');
        link.href = 'data:application/octet-stream;base64,' + base64;
        link.download = 'geogebra-' + Date.now() + '.ggb';
        link.click();
      }
    } catch (e) {
      console.warn('[GeoGebra] exportGGB failed:', e);
    }
  }, []);

  const exportPNG = useCallback(() => {
    const api = ggbRef.current;
    if (!api) return;
    try {
      const base64 = api.getPNGBase64?.(1, false, 72);
      if (base64) {
        const link = document.createElement('a');
        link.href = 'data:image/png;base64,' + base64;
        link.download = 'geogebra-' + Date.now() + '.png';
        link.click();
      }
    } catch (e) {
      console.warn('[GeoGebra] exportPNG failed:', e);
    }
  }, []);

  // Use a global flag to prevent duplicate script loading across Strict Mode double-mount
  const getScriptLoaded = () => (window as any).__geogebraScriptLoaded === true;
  const setScriptLoaded = () => { (window as any).__geogebraScriptLoaded = true; };
  const getAppletInited = () => (window as any).__geogebraAppletInited === true;
  const setAppletInited = () => { (window as any).__geogebraAppletInited = true; };

  useImperativeHandle(ref, () => ({
    executeCommand: (cmd: string) => {
      if (!ggbRef.current) {
        console.warn('[GeoGebra] executeCommand skipped: applet not ready');
        return;
      }
      try {
        if (typeof ggbRef.current.evalCommand === 'function') {
          ggbRef.current.evalCommand(cmd);
        } else {
          // Fallback: try common legacy API names
          const fn = ggbRef.current.evalCommandCAS || ggbRef.current.evalCmd;
          if (typeof fn === 'function') {
            fn.call(ggbRef.current, cmd);
          } else {
            console.warn('[GeoGebra] executeCommand: no evalCommand on applet', Object.keys(ggbRef.current).slice(0, 20));
          }
        }
      } catch (e) {
        console.warn('[GeoGebra] evalCommand failed:', e);
      }
    },
    setPerspective: (p: string) => {
      if (ggbRef.current?.setPerspective) {
        try {
          ggbRef.current.setPerspective(p);
        } catch (e) {
          console.warn('[GeoGebra] setPerspective failed:', e);
        }
      }
    },
    setSize: (width: number, height: number) => {
      if (ggbRef.current?.setSize) {
        try {
          ggbRef.current.setSize(width, height);
        } catch (e) {
          console.warn('[GeoGebra] setSize failed:', e);
        }
      }
    },
    reset: () => {
      if (ggbRef.current) {
        try {
          // Delete all objects by iterating over their names
          if (typeof ggbRef.current.getAllObjectNames === 'function') {
            const names = ggbRef.current.getAllObjectNames();
            if (names && names.length > 0) {
              for (const name of names) {
                if (typeof ggbRef.current.deleteObject === 'function') {
                  ggbRef.current.deleteObject(name);
                } else if (typeof ggbRef.current.evalCommand === 'function') {
                  ggbRef.current.evalCommand(`Delete(${name})`);
                }
              }
            }
          }
        } catch (e) {
          console.warn('[GeoGebra] reset failed:', e);
        }
      }
    },
    getAPI: () => ggbRef.current,
  }));

  const initGeoGebra = useCallback(() => {
    if (!containerRef.current || getAppletInited()) return;

    try {
      const width = containerRef.current.clientWidth || 800;
      const height = containerRef.current.clientHeight || 600;

      // Pick appName based on perspective
      const appName = perspective === '5' ? '3d' : perspective === '2' ? 'geometry' : 'graphing';

      const applet = new (window as any).GGBApplet({
        appletId: containerId,
        width,
        height,
        showToolBar: true,
        showAlgebraInput: true,
        showMenuBar: false,
        enableRightClick: true,
        enableShiftDragZoom: true,
        useBrowserForJS: false,
        allowResizing: true,
        scaleContainerClass: 'ggb-runner-iframe',
        appName,
        perspective,
        appletOnLoad: (api: any) => {
          // Store the actual on-load API which exposes evalCommand reliably
          ggbRef.current = api;
          // Sync view states after load
          try {
            if (api.showMenuBar) api.showMenuBar(showMenuBar);
            if (api.showToolBar) api.showToolBar(showToolBar);
            if (api.showAlgebraInput) api.showAlgebraInput(showAlgebraInput);
            if (api.setAxesVisible) api.setAxesVisible(axesVisible, axesVisible, axesVisible);
            if (api.setGridVisible) api.setGridVisible(gridVisible);
          } catch {}
          try {
            onReadyStable(api);
          } catch (e) {
            console.warn('[GeoGebra] onReady callback failed:', e);
          }
          // Force an initial resize after the applet is fully loaded
          const tryResize = (attempt: number) => {
            const el = containerRef.current;
            if (!el) return;
            const target = el.parentElement || el;
            const rect = target.getBoundingClientRect();
            if (rect.width < 10 || rect.height < 10) {
              if (attempt < 10) {
                setTimeout(() => tryResize(attempt + 1), 100);
              }
              return;
            }
            try {
              if (typeof api.setSize === 'function') {
                api.setSize(Math.floor(rect.width), Math.floor(rect.height));
              } else if (typeof api.setWidth === 'function' && typeof api.setHeight === 'function') {
                api.setWidth(Math.floor(rect.width));
                api.setHeight(Math.floor(rect.height));
              }
            } catch {
              // ignore
            }
          };
          tryResize(0);
        },
      }, true);
      // Track applet immediately so ref exposes methods right after inject
      ggbRef.current = applet;
      applet.inject(containerId);
      setAppletInited();
      setLoadError(null);

      // Initial resize after inject
      requestAnimationFrame(() => {
        const el = containerRef.current;
        if (el) {
          const rect = el.getBoundingClientRect();
          if (rect.width > 0 && rect.height > 0) {
            try {
              if (typeof applet.setSize === 'function') {
                applet.setSize(Math.floor(rect.width), Math.floor(rect.height));
              } else if (typeof applet.setWidth === 'function' && typeof applet.setHeight === 'function') {
                applet.setWidth(Math.floor(rect.width));
                applet.setHeight(Math.floor(rect.height));
              }
            } catch {
              // ignore initial resize race
            }
          }
        }
      });
    } catch (e) {
      console.warn('[GeoGebra] init failed:', e);
      setLoadError('GeoGebra 初始化失败，请检查网络连接');
    }
  }, [containerId, perspective, onReadyStable]);

  const rebuildApplet = useCallback(() => {
    if (!containerRef.current) return;

    // Reset global flag and tear down existing applet
    (window as any).__geogebraAppletInited = false;
    ggbRef.current = null;

    // Clear container DOM
    containerRef.current.innerHTML = '';
    containerRef.current.removeAttribute('data-injected');

    // Add a delay so the previous applet is fully torn down before injecting
    // a new one, ensuring appletOnLoad fires again and scriptCode re-runs.
    setTimeout(() => {
      initGeoGebra();
    }, 150);
  }, [initGeoGebra]);

  const retryLoad = useCallback(() => {
    setLoadError(null);
    setAppletInited();
    (window as any).__geogebraScriptLoaded = false;
    (window as any).__geogebraAppletInited = false;
    initGeoGebra();
  }, [initGeoGebra]);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    if ((window as any).GGBApplet) {
      initGeoGebra();
      return;
    }

    if (getScriptLoaded()) return;

    const CDN_CANDIDATES = [
      '/deployggb.js',
      'https://cdn.geogebra.org/apps/deployggb.js',
      'http://cdn.geogebra.org/apps/deployggb.js',
    ];

    const tryLoadScript = (index: number) => {
      if (index >= CDN_CANDIDATES.length) {
        const errMsg = 'GeoGebra 加载失败，请检查网络连接后重试';
        console.warn('[GeoGebra] all CDN candidates failed');
        setLoadError(errMsg);
        return;
      }

      const existingScript = document.querySelector(`script[src="${CDN_CANDIDATES[index]}"]`);
      if (existingScript) {
        const checkInterval = setInterval(() => {
          if ((window as any).GGBApplet) {
            clearInterval(checkInterval);
            initGeoGebra();
          }
        }, 100);
        setTimeout(() => clearInterval(checkInterval), 10000);
        setScriptLoaded();
        return;
      }

      const script = document.createElement('script');
      script.src = CDN_CANDIDATES[index];
      script.async = true;
      script.onload = () => {
        setScriptLoaded();
        setLoadError(null);
        initGeoGebra();
      };
      script.onerror = () => {
        console.warn(`[GeoGebra] failed to load ${CDN_CANDIDATES[index]}`);
        tryLoadScript(index + 1);
      };
      document.head.appendChild(script);
      setScriptLoaded();
    };

    tryLoadScript(0);

    return () => {
      // Keep global flag to prevent reloading in Strict Mode double-mount
    };
  }, [initGeoGebra]);

  useEffect(() => {
    // Only rebuild after initial mount (skip first run)
    if (!getAppletInited()) return;

    // If applet already exists with matching perspective, no-op
    // Otherwise rebuild since changing appName requires recreating the applet
    rebuildApplet();
  }, [perspective, rebuildApplet]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    // Watch the parent (.workspace-canvas) which stays mounted across rebuilds
    const watchTarget = el.parentElement;
    if (!watchTarget) return;

    const resizeApplet = (width: number, height: number) => {
      if (!width || !height || width < 10 || height < 10) return;
      if (!ggbRef.current) return;
      try {
        if (typeof ggbRef.current.setSize === 'function') {
          ggbRef.current.setSize(Math.floor(width), Math.floor(height));
        } else if (typeof ggbRef.current.setWidth === 'function' && typeof ggbRef.current.setHeight === 'function') {
          ggbRef.current.setWidth(Math.floor(width));
          ggbRef.current.setHeight(Math.floor(height));
        }
      } catch {
        // ignore resize race
      }
    };

    const measureAndResize = () => {
      // Use the workspace-canvas rect (parent of the GGB mount target) so we get the real area
      const rect = watchTarget.getBoundingClientRect();
      resizeApplet(rect.width, rect.height);
    };

    const observer = new ResizeObserver(() => {
      measureAndResize();
    });

    observer.observe(watchTarget);

    const handleWindowResize = () => {
      measureAndResize();
    };

    window.addEventListener('resize', handleWindowResize);

    // Initial measurement after layout settles
    const initialTimeout = window.setTimeout(() => {
      measureAndResize();
    }, 50);

    // Periodic resize as a belt-and-suspenders fallback during the first few seconds
    // while the applet's internal layout finishes settling.
    const interval = window.setInterval(() => {
      measureAndResize();
    }, 250);
    const stopInterval = window.setTimeout(() => {
      window.clearInterval(interval);
    }, 4000);

    return () => {
      observer.disconnect();
      window.removeEventListener('resize', handleWindowResize);
      window.clearTimeout(initialTimeout);
      window.clearTimeout(stopInterval);
      window.clearInterval(interval);
    };
  }, []);

  return (
    <div className="geogebra-workspace">
      <div className="workspace-toolbar">
        <div className="perspective-switcher">
          <button
            onClick={() => onPerspectiveChange?.('1')}
            className={`perspective-btn ${perspective === '1' ? 'active' : ''}`}
            title="函数/代数"
          >
            函数
          </button>
          <button
            onClick={() => onPerspectiveChange?.('2')}
            className={`perspective-btn ${perspective === '2' ? 'active' : ''}`}
            title="平面几何"
          >
            平面
          </button>
          <button
            onClick={() => onPerspectiveChange?.('5')}
            className={`perspective-btn ${perspective === '5' ? 'active' : ''}`}
            title="立体几何"
          >
            立体
          </button>
        </div>
        <div className="toolbar-divider" />
        <div className="view-toggles">
          <button
            onClick={toggleMenuBar}
            className={`toggle-btn ${showMenuBar ? 'active' : ''}`}
            title="菜单栏"
          >
            菜单
          </button>
          <button
            onClick={toggleToolBar}
            className={`toggle-btn ${showToolBar ? 'active' : ''}`}
            title="工具栏"
          >
            工具
          </button>
          <button
            onClick={toggleAlgebraInput}
            className={`toggle-btn ${showAlgebraInput ? 'active' : ''}`}
            title="代数输入栏"
          >
            输入
          </button>
          <button
            onClick={toggleAxes}
            className={`toggle-btn ${axesVisible ? 'active' : ''}`}
            title="坐标轴"
          >
            坐标
          </button>
          <button
            onClick={toggleGrid}
            className={`toggle-btn ${gridVisible ? 'active' : ''}`}
            title="网格"
          >
            网格
          </button>
        </div>
        <div className="toolbar-divider" />
        <div className="export-buttons">
          <button
            onClick={exportGGB}
            className="export-btn"
            title="导出 .ggb 文件"
          >
            导出GGB
          </button>
          <button
            onClick={exportPNG}
            className="export-btn"
            title="导出 PNG 图片"
          >
            导出图片
          </button>
        </div>
      </div>
      <div className="workspace-canvas">
        <div
          ref={containerRef}
          id={containerId}
          className="geogebra-canvas"
          style={{ width: '100%', height: '100%' }}
        />
        {loadError && (
          <div className="absolute inset-0 flex items-center justify-center bg-white/80 z-10">
            <div className="text-center">
              <p className="text-red-600 mb-2">{loadError}</p>
              <button
                onClick={retryLoad}
                className="px-3 py-1.5 bg-blue-600 text-white text-sm rounded hover:bg-blue-700"
              >
                重试
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
});

GeoGebraWorkspace.displayName = 'GeoGebraWorkspace';

export default GeoGebraWorkspace;
