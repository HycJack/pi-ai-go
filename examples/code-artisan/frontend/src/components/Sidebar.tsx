import { useState } from 'react';
import type { ConversationSummary } from '../types';
import {
  DeleteOutlined,
  PlusOutlined,
  SettingOutlined,
  HistoryOutlined,
  CodeOutlined,
} from '../icons';

interface SidebarProps {
  conversations: ConversationSummary[];
  selectedConvId: string | null;
  pythonStatus: string;
  pythonStatusLoading: boolean;
  runtimeReady: boolean;
  onSelectConv: (summary: ConversationSummary) => void;
  onDeleteConv: (id: string) => void;
  onOpenSettings: () => void;
  onCheckPython: () => void;
  onRebuildRuntime: () => void;
  onClearCode: () => void;
}

export default function Sidebar({
  conversations,
  selectedConvId,
  pythonStatus,
  pythonStatusLoading,
  runtimeReady,
  onSelectConv,
  onDeleteConv,
  onOpenSettings,
  onCheckPython,
  onRebuildRuntime,
  onClearCode,
}: SidebarProps) {
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [toolsExpanded, setToolsExpanded] = useState(false);

  const requestDelete = (id: string) => {
    setConfirmDeleteId(id);
  };

  const confirmDelete = (id: string) => {
    onDeleteConv(id);
    setConfirmDeleteId(null);
  };

  const cancelDelete = () => setConfirmDeleteId(null);

  const parsePythonStatus = () => {
    try {
      const s = JSON.parse(pythonStatus);
      return s;
    } catch {
      return null;
    }
  };

  const statusData = parsePythonStatus();

  return (
    <aside className="app-sidebar">
      <div className="brand-block">
        <div className="brand-mark">
          <div className="brand-logo">
            <CodeOutlined size={20} />
          </div>
        </div>
        <div className="brand-text">
          <div className="brand">Code Artisan</div>
          <div className="bridge-pill">
            <span className={`status-dot ${runtimeReady ? 'green' : 'yellow'}`} />
            <span>{runtimeReady ? '就绪' : '未就绪'}</span>
          </div>
        </div>
      </div>

      <button className="primary-cta" onClick={onClearCode}>
        <PlusOutlined size={16} />
        <span>新建</span>
      </button>

      <div className="history-list">
        <div className="history-list-header">
          <span className="history-list-title">
            <HistoryOutlined size={16} />
            <span>对话记录</span>
          </span>
        </div>
        <div className="history-list-items">
          {conversations.length === 0 && (
            <div className="history-empty">暂无记录</div>
          )}
          {conversations.map((item) => {
            const isActive = selectedConvId === item.id;
            const isConfirmingDelete = confirmDeleteId === item.id;
            return (
              <div
                key={item.id}
                className={`history-item ${isActive ? 'active' : ''}`}
                onClick={() => {
                  if (isConfirmingDelete) return;
                  onSelectConv(item);
                }}
              >
                <span className="history-bullet" />
                <div className="history-meta">
                  <div className="history-title">{item.title}</div>
                  <div className="history-timestamp">{item.timestamp}</div>
                </div>
                {isConfirmingDelete ? (
                  <div className="history-confirm-actions">
                    <button
                      className="history-confirm-btn danger"
                      onClick={(e) => { e.stopPropagation(); confirmDelete(item.id); }}
                    >
                      确认
                    </button>
                    <button
                      className="history-confirm-btn"
                      onClick={(e) => { e.stopPropagation(); cancelDelete(); }}
                    >
                      取消
                    </button>
                  </div>
                ) : (
                  <button
                    className="history-action"
                    onClick={(e) => { e.stopPropagation(); requestDelete(item.id); }}
                    title="删除"
                  >
                    <DeleteOutlined size={14} />
                  </button>
                )}
              </div>
            );
          })}
        </div>
      </div>

      <div className="tools-section">
        <div
          className="tools-section-header"
          onClick={() => setToolsExpanded(!toolsExpanded)}
        >
          <span className={`tools-chevron ${toolsExpanded ? 'expanded' : ''}`}>▸</span>
          <span>Python 环境</span>
        </div>
        {toolsExpanded && (
          <div className="tools-section-content">
            {statusData ? (
              <div className="runtime-status">
                <div className={`runtime-indicator ${statusData.ready ? 'ready' : 'error'}`}>
                  <span className="status-dot-big" />
                  <span>{statusData.summary || (statusData.ready ? '就绪' : '异常')}</span>
                </div>
                {statusData.missing && statusData.missing.length > 0 && (
                  <div className="runtime-missing">
                    <span className="tool-label">缺失组件:</span>
                    <ul>
                      {statusData.missing.map((m: string, i: number) => (
                        <li key={i}>{m}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {statusData.runtimeDir && (
                  <div className="tool-info">
                    <span className="tool-label">路径:</span>
                    <span className="tool-value small">{statusData.runtimeDir}</span>
                  </div>
                )}
              </div>
            ) : pythonStatus && (
              <div className="tool-status">{pythonStatus}</div>
            )}

            <div className="tool-buttons vertical">
              <button className="tool-btn" onClick={onCheckPython} disabled={pythonStatusLoading}>
                {pythonStatusLoading ? '检查中...' : '检查 Python 环境'}
              </button>
              {statusData && !statusData.ready && statusData.canRebuild && (
                <button className="tool-btn install" onClick={onRebuildRuntime} disabled={pythonStatusLoading}>
                  {pythonStatusLoading ? '重建中...' : '重建 Runtime'}
                </button>
              )}
              {statusData && !statusData.ready && !statusData.canRebuild && (
                <div className="tool-hint">请将 runtime.zip 放入 resources/runtime/ 目录</div>
              )}
            </div>
          </div>
        )}
      </div>

      <div className="sidebar-footer">
        <button className="nav-item" onClick={onOpenSettings}>
          <SettingOutlined size={16} />
          <span>设置</span>
        </button>
      </div>
    </aside>
  );
}
