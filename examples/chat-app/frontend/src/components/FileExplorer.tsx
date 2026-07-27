import { useState, useEffect, useCallback, useRef } from 'react';
import { FolderOpenOutlined, CodeOutlined, RefreshOutlined, ChevronLeftOutlined } from '../icons';
import { useT } from '../i18n';

interface FileEntry {
  name: string;
  isDir: boolean;
  size?: number;
}

interface FileExplorerProps {
  workingDir: string;
  onOpenFile?: (filePath: string, fileName: string) => void;
}

export default function FileExplorer({ workingDir, onOpenFile }: FileExplorerProps) {
  const t = useT();
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [currentPath, setCurrentPath] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const loadIdRef = useRef(0);

  const loadDir = useCallback(async (dir: string) => {
    const id = ++loadIdRef.current;
    setLoading(true);
    setError('');
    try {
      const { ListDirectory } = await import('../../wailsjs/go/main/App');
      const raw = await ListDirectory(dir);
      if (id !== loadIdRef.current) return; // stale response
      const list: FileEntry[] = JSON.parse(raw);
      setEntries(list.sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
        return a.name.localeCompare(b.name);
      }));
      setCurrentPath(dir);
    } catch (e: any) {
      if (id !== loadIdRef.current) return; // stale response
      setError(String(e?.message || e));
      setEntries([]);
    } finally {
      if (id === loadIdRef.current) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    if (!workingDir) return;
    loadDir(workingDir);
  }, [workingDir, loadDir]);

  const handleEntryClick = (entry: FileEntry) => {
    if (entry.isDir) {
      const newPath = currentPath.replace(/\\/g, '/') + '/' + entry.name;
      loadDir(newPath);
    } else {
      const filePath = currentPath.replace(/\\/g, '/') + '/' + entry.name;
      onOpenFile?.(filePath, entry.name);
    }
  };

  const goUp = () => {
    const normalized = currentPath.replace(/\\/g, '/');
    const parent = normalized.substring(0, normalized.lastIndexOf('/'));
    if (parent) {
      loadDir(parent);
    }
  };

  const refresh = () => {
    if (currentPath) loadDir(currentPath);
    else if (workingDir) loadDir(workingDir);
  };

  const dirName = currentPath
    ? currentPath.replace(/\\/g, '/').split('/').filter(Boolean).slice(-1)[0] || currentPath
    : '';

  return (
    <div className="file-explorer">
      <div className="file-explorer-header">
        <div className="file-explorer-title">
          <FolderOpenOutlined size={14} />
          <span>{dirName || t('sidebar.workingDir')}</span>
        </div>
        <div className="file-explorer-actions">
          {currentPath && (
            <button className="icon-btn" onClick={goUp} title={t('app.goUp')}>
              <ChevronLeftOutlined size={14} />
            </button>
          )}
          <button className="icon-btn" onClick={refresh} disabled={loading} title={t('app.refresh')}>
            <RefreshOutlined size={14} style={{ animation: loading ? 'status-spin 900ms linear infinite' : undefined }} />
          </button>
        </div>
      </div>

      <div className="file-explorer-body">
        {!workingDir && (
          <div className="file-explorer-empty">{t('fileExplorer.noWorkingDir')}</div>
        )}
        {error && (
          <div className="file-explorer-error">{error}</div>
        )}
        {loading && entries.length === 0 && (
          <div className="file-explorer-loading">
            <span className="status-spinner" />
            <span>{t('app.loading')}</span>
          </div>
        )}
        {!loading && entries.length === 0 && workingDir && !error && (
          <div className="file-explorer-empty">{t('fileExplorer.empty')}</div>
        )}
        {entries.length > 0 && (
          <div className="file-explorer-list">
            {currentPath && (
              <div className="file-explorer-item" onClick={goUp}>
                <span className="file-explorer-item-icon">
                  <FolderOpenOutlined size={14} />
                </span>
                <span className="file-explorer-item-name">..</span>
              </div>
            )}
            {entries.map((entry, idx) => (
              <div
                key={idx}
                className={`file-explorer-item ${entry.isDir ? 'dir' : 'file'}`}
                onClick={() => handleEntryClick(entry)}
              >
                <span className="file-explorer-item-icon">
                  {entry.isDir ? <FolderOpenOutlined size={14} /> : <CodeOutlined size={14} />}
                </span>
                <span className="file-explorer-item-name" title={entry.name}>
                  {entry.name}
                </span>
                {!entry.isDir && entry.size != null && (
                  <span className="file-explorer-item-size">{formatSize(entry.size)}</span>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
