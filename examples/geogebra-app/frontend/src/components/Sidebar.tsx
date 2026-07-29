import { Triangle, MessageSquare, Plus, Trash2, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import type { Conversation } from '../types';

interface SidebarProps {
  conversations: Conversation[];
  activeConversationId: string | null;
  onSelectConversation: (id: string) => void;
  onNewConversation: () => void;
  onDeleteConversation: (id: string) => void;
  onOpenSettings: () => void;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
}

export default function Sidebar({
  conversations,
  activeConversationId,
  onSelectConversation,
  onNewConversation,
  onDeleteConversation,
  onOpenSettings,
  collapsed = false,
  onToggleCollapse,
}: SidebarProps) {
  return (
    <aside className={`app-sidebar ${collapsed ? 'collapsed' : ''}`}>
      <div className="sidebar-header">
        {!collapsed && (
          <div className="sidebar-brand">
            <div className="sidebar-brand-icon">
              <Triangle size={20} />
            </div>
            <div>
              <div className="sidebar-brand-text">GeoGebra</div>
              <div className="sidebar-brand-sub">指令生成器</div>
            </div>
          </div>
        )}
        {onToggleCollapse && (
          <button className="sidebar-collapse-btn" onClick={onToggleCollapse} title={collapsed ? '展开' : '收起'}>
            {collapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
          </button>
        )}
      </div>

      {!collapsed && (
        <>
          <button className="btn-new" onClick={onNewConversation}>
            <Plus size={14} />
            新建
          </button>

          <div className="history-section">
            <div className="history-header">
              <span className="history-header-title">历史记录</span>
            </div>
            <div className="history-list">
              {conversations.length === 0 && (
                <div className="history-empty">暂无记录，输入描述生成 GeoGebra 课件</div>
              )}
              {conversations.map((conv) => {
                const isActive = activeConversationId === conv.id;
                return (
                  <div
                    key={conv.id}
                    className={`history-item ${isActive ? 'active' : ''}`}
                    onClick={() => onSelectConversation(conv.id)}
                  >
                    <MessageSquare size={14} className="history-icon" />
                    <div className="history-item-content">
                      <span className="history-item-title">{conv.title}</span>
                      <span className="history-item-time">{conv.timestamp}</span>
                    </div>
                    <button
                      className="history-delete-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        const title = conv.title || '此对话';
                        const confirmed = confirm(`确定要删除"${title}"吗？此操作不可恢复。`);
                        if (!confirmed) return;
                        onDeleteConversation(conv.id);
                      }}
                      title="删除"
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        </>
      )}

      {!collapsed && (
        <div className="sidebar-footer">
          <button className="sidebar-footer-btn" onClick={onOpenSettings}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="3" />
              <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
            </svg>
            <span>设置</span>
          </button>
        </div>
      )}
    </aside>
  );
}
