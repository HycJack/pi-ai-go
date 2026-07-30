import { useEffect, useRef, useState, useImperativeHandle, forwardRef, useCallback } from 'react';

interface GeoGebraProps {
  perspective?: string;
  onReady?: (api: any) => void;
}

export interface GeoGebraRef {
  executeCommand: (cmd: string) => void;
  evalCommand: (cmd: string) => void;
  setPerspective: (p: string) => void;
  reset: () => void;
  downloadGGB: () => void;
  getPNGBase64: (callback: (data: string) => void) => void;
  getObjectNames: () => string[];
  getCommandString: (objName?: string) => string | string[];
}

const GeoGebra = forwardRef<GeoGebraRef, GeoGebraProps>(({
  perspective = '2',
  onReady,
}, ref) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const appletRef = useRef<any>(null);
  const [isReady, setIsReady] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const injectedRef = useRef(false);
  const resizeTimeoutRef = useRef<number | null>(null);
  const scriptLoadedRef = useRef(false);
  const appletIdRef = useRef(`ggb-${Math.random().toString(36).slice(2, 9)}`);

  const resizeApplet = useCallback((width: number, height: number) => {
    if (!appletRef.current || !containerRef.current) return;

    const w = Math.round(width);
    const h = Math.round(height);

    if (resizeTimeoutRef.current !== null) {
      clearTimeout(resizeTimeoutRef.current);
      resizeTimeoutRef.current = null;
    }

    resizeTimeoutRef.current = window.setTimeout(() => {
      if (!appletRef.current || !containerRef.current) return;

      if (typeof appletRef.current.setSize === 'function') {
        appletRef.current.setSize(w, h);
      }
      if (typeof appletRef.current.setWidth === 'function') {
        appletRef.current.setWidth(w);
      }
      if (typeof appletRef.current.setHeight === 'function') {
        appletRef.current.setHeight(h);
      }

      // Fallback: resize iframe
      const iframes = containerRef.current.querySelectorAll('iframe');
      iframes.forEach((iframe) => {
        iframe.style.width = w + 'px';
        iframe.style.height = h + 'px';
      });
      
      resizeTimeoutRef.current = null;
    }, 50);
  }, []);

  useImperativeHandle(ref, () => ({
    executeCommand: (cmd: string) => {
      if (appletRef.current) {
        appletRef.current.evalCommand(cmd);
      }
    },
    evalCommand: (cmd: string) => {
      if (appletRef.current) {
        appletRef.current.evalCommand(cmd);
      }
    },
    setPerspective: (p: string) => {
      if (appletRef.current) {
        appletRef.current.setPerspective(p);
      }
    },
    reset: () => {
      if (appletRef.current) {
        try {
          if (typeof appletRef.current.getAllObjectNames === 'function') {
            const names = appletRef.current.getAllObjectNames();
            if (names && names.length > 0) {
              for (const name of names) {
                if (typeof appletRef.current.deleteObject === 'function') {
                  appletRef.current.deleteObject(name);
                } else if (typeof appletRef.current.evalCommand === 'function') {
                  appletRef.current.evalCommand(`Delete(${name})`);
                }
              }
            }
          }
        } catch (e) {
          console.warn('[GeoGebra] reset failed:', e);
        }
      }
    },
    downloadGGB: () => {
      if (appletRef.current) {
        appletRef.current.getBase64((base64: string) => {
          const link = document.createElement('a');
          link.href = 'data:application/vnd.geogebra.file;base64,' + base64;
          link.download = `geogebra-export-${new Date().toISOString().slice(0,19).replace(/:/g,'-')}.ggb`;
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
        });
      }
    },
    getPNGBase64: (callback: (data: string) => void) => {
      if (appletRef.current) {
        appletRef.current.getPNGBase64(1, false, 300, false, callback);
      }
    },
    getObjectNames: () => {
      return appletRef.current ? appletRef.current.getAllObjectNames() : [];
    },
    getCommandString: (objName?: string) => {
      if (!appletRef.current) return '';
      if (objName) return appletRef.current.getCommandString(objName);
      
      const names = appletRef.current.getAllObjectNames();
      const commands: string[] = [];
      for (const name of names) {
        const cmd = appletRef.current.getCommandString(name);
        if (cmd) {
          commands.push(`${name} = ${cmd}`);
        } else {
          const val = appletRef.current.getValueString(name);
          commands.push(val);
        }
      }
      return commands;
    },
  }));

  // Handle perspective changes
  useEffect(() => {
    if (isReady && appletRef.current) {
      appletRef.current.setPerspective(perspective);
    }
  }, [perspective, isReady]);

  // Setup ResizeObserver
  useEffect(() => {
    if (!isReady || !containerRef.current) return;

    const parent = containerRef.current.parentElement;
    if (!parent) return;

    const resizeObserver = new ResizeObserver((entries) => {
      if (!appletRef.current) return;

      for (const entry of entries) {
        const rect = parent.getBoundingClientRect();
        if (rect.width > 0 && rect.height > 0) {
          resizeApplet(rect.width, rect.height);
        }
      }
    });

    resizeObserver.observe(parent);

    return () => {
      resizeObserver.disconnect();
    };
  }, [isReady, resizeApplet]);

  // Listen to window resize
  useEffect(() => {
    if (!isReady) return;

    const handleResize = () => {
      if (!containerRef.current || !appletRef.current) return;
      const parent = containerRef.current.parentElement;
      if (!parent) return;

      const rect = parent.getBoundingClientRect();
      if (rect.width > 0 && rect.height > 0) {
        resizeApplet(rect.width, rect.height);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [isReady, resizeApplet]);

  // Cleanup resize timeout
  useEffect(() => {
    return () => {
      if (resizeTimeoutRef.current !== null) {
        clearTimeout(resizeTimeoutRef.current);
        resizeTimeoutRef.current = null;
      }
    };
  }, []);

  // Initialize GeoGebra applet
  useEffect(() => {
    let scriptElement: HTMLScriptElement | null = null;

    const loadGeoGebra = () => {
      if (window.GGBApplet) {
        initApplet();
      } else {
        // Prevent duplicate loading
        const existingScript = document.querySelector('script[src="/deployggb.js"]') || document.querySelector('script[src="https://cdn.geogebra.org/apps/deployggb.js"]');
        if (existingScript) {
          // Script is loading, wait for it
          const checkInterval = setInterval(() => {
            if (window.GGBApplet) {
              clearInterval(checkInterval);
              initApplet();
            }
          }, 100);
          // Timeout after 10 seconds
          setTimeout(() => clearInterval(checkInterval), 10000);
          return;
        }

        scriptElement = document.createElement('script');
        scriptElement.src = '/deployggb.js';
        scriptElement.onload = () => {
          scriptLoadedRef.current = true;
          initApplet();
        };
        scriptElement.onerror = () => {
          console.error('[GeoGebra] Failed to load deployggb.js');
          setError('无法加载 GeoGebra，请检查网络连接');
          setIsLoading(false);
        };
        document.head.appendChild(scriptElement);
      }
    };

    const initApplet = () => {
      if (!containerRef.current) return;
      
      // Reset injection state
      containerRef.current.setAttribute('data-injected', 'yes');
      injectedRef.current = true;
      setError(null);

      const width = containerRef.current.clientWidth || 800;
      const height = containerRef.current.clientHeight || 600;

      const params: Record<string, any> = {
        appName: 'classic',
        width,
        height,
        showToolBar: true,
        showAlgebraInput: true,
        showMenuBar: false,
        perspective: perspective,
        allowStyleBar: true,
        showResetIcon: true,
        enableLabelDrags: false,
        enableShiftDragZoom: true,
        enableRightClick: true,
        capturingThreshold: null,
        showLogging: false,
        useBrowserForJS: false,
        appletOnLoad: (api: any) => {
          appletRef.current = api;
          setIsReady(true);
          setIsLoading(false);
          onReady?.(api);
          
          // Initial resize after applet is ready
          requestAnimationFrame(() => {
            const parent = containerRef.current?.parentElement;
            if (parent) {
              const rect = parent.getBoundingClientRect();
              if (rect.width > 0 && rect.height > 0) {
                resizeApplet(rect.width, rect.height);
              }
            }
          });
        }
      };

      try {
        // @ts-ignore - deployggb.js global constructor
        // @ts-ignore - second argument enables legacy API with setSize
        const applet = new window.GGBApplet(params, true);
        applet.inject(appletIdRef.current);
      } catch (e) {
        console.error('[GeoGebra] Failed to initialize applet:', e);
        setError('GeoGebra 初始化失败');
        setIsLoading(false);
      }
    };

    loadGeoGebra();

    return () => {
      if (scriptElement && scriptElement.parentNode) {
        scriptElement.parentNode.removeChild(scriptElement);
      }
    };
  }, [perspective, onReady, resizeApplet]);

  return (
    <div 
      ref={containerRef} 
      className="w-full h-full bg-white relative"
    >
      <div id={appletIdRef.current} className="w-full h-full" />
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-gray-50 z-10">
          <div className="text-sm text-gray-500">正在加载 GeoGebra...</div>
        </div>
      )}
      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-gray-50 z-10">
          <div className="text-sm text-red-500">{error}</div>
        </div>
      )}
    </div>
  );
});

GeoGebra.displayName = 'GeoGebra';

export default GeoGebra;
