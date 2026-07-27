import { useState, useRef } from 'react';
import {
  CheckOutlined2,
  DeleteOutlined,
  FolderOpenOutlined,
  MessageOutlined,
  PlusOutlined,
  RefreshOutlined,
  SettingOutlined,
  UserOutlined,
} from '../icons';
import { useT } from '../i18n';
import FileExplorer from './FileExplorer';

interface Conversation {
  id: string;
  title: string;
  timestamp: string;
}

interface SidebarProps {
  conversations: Conversation[];
  activeConversation: string | null;
  workingDir: string;
  onSelectConversation: (id: string) => void;
  onCreateNewConversation: () => void;
  onDeleteConversation: (id: string) => void;
  onRenameConversation: (id: string, title: string) => void;
  onOpenSettings: () => void;
  onOpenFile?: (filePath: string, fileName: string) => void;
}

export default function Sidebar({
  conversations,
  activeConversation,
  workingDir,
  onSelectConversation,
  onCreateNewConversation,
  onDeleteConversation,
  onRenameConversation,
  onOpenSettings,
  onOpenFile,
}: SidebarProps) {
  const t = useT();
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState('');
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const [treeExpanded, setTreeExpanded] = useState(true);
  const treeRef = useRef<HTMLDivElement>(null);
  const [, forceUpdate] = useState(0);

  const dirName = workingDir ? workingDir.split(/[\\/]/).filter(Boolean).slice(-1)[0] : '';

  const refreshTree = () => {
    // Force remount FileExplorer by toggling a dummy key
    forceUpdate((n) => n + 1);
  };

  const beginEdit = (conv: Conversation) => {
    setEditingId(conv.id);
    setEditingTitle(conv.title);
    setConfirmDeleteId(null);
  };

  const commitEdit = () => {
    if (!editingId) return;
    const trimmed = editingTitle.trim();
    if (trimmed) {
      onRenameConversation(editingId, trimmed);
    }
    setEditingId(null);
    setEditingTitle('');
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditingTitle('');
  };

  const requestDelete = (id: string) => {
    setConfirmDeleteId(id);
    setEditingId(null);
  };

  const confirmDelete = () => {
    if (confirmDeleteId) {
      onDeleteConversation(confirmDeleteId);
      setConfirmDeleteId(null);
    }
  };

  const cancelDelete = () => setConfirmDeleteId(null);

  return (
    <aside className="app-sidebar">
      <div className="brand-block">
        <div className="brand-mark">
          <div className="brand-logo">π</div>
        </div>
        <div className="brand-text">
          <div className="brand">Pi-AI Chat</div>
          <div className="bridge-pill">
            <span className="status-dot green" />
            <span>{t('sidebar.ready')}</span>
          </div>
        </div>
      </div>

      <button className="primary-cta" onClick={onCreateNewConversation}>
        <PlusOutlined size={16} />
        <span>{t('app.newChat')}</span>
      </button>

      <div className="history-list">
        <div className="history-list-header">
          <span className="history-list-title">
            <MessageOutlined size={16} />
            <span>{t('sidebar.history')}</span>
          </span>
        </div>
        <div className="history-list-items">
          {conversations.length === 0 && (
            <div className="history-empty">{t('sidebar.noConversations')}</div>
          )}
          {conversations.map((conv) => {
            const isActive = activeConversation === conv.id;
            const isEditing = editingId === conv.id;
            const isConfirmingDelete = confirmDeleteId === conv.id;
            return (
              <div
                key={conv.id}
                className={`history-item ${isActive ? 'active' : ''}`}
                onClick={() => {
                  if (isEditing || isConfirmingDelete) return;
                  onSelectConversation(conv.id);
                }}
                onDoubleClick={() => beginEdit(conv)}
              >
                <span className="history-bullet" />
                <div className="history-meta">
                  {isEditing ? (
                    <input
                      autoFocus
                      className="history-title-input"
                      value={editingTitle}
                      onChange={(e) => setEditingTitle(e.target.value)}
                      onClick={(e) => e.stopPropagation()}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') { e.preventDefault(); commitEdit(); }
                        else if (e.key === 'Escape') { cancelEdit(); }
                      }}
                      onBlur={commitEdit}
                      maxLength={120}
                    />
                  ) : (
                    <div className="history-title">{conv.title}</div>
                  )}
                  {isConfirmingDelete ? (
                    <div className="history-confirm-row">
                      <span className="history-confirm-text">{t('chat.deleteConfirm')}</span>
                      <button
                        className="history-confirm-btn danger"
                        onClick={(e) => { e.stopPropagation(); confirmDelete(); }}
                      >
                        {t('chat.deleteYes')}
                      </button>
                      <button
                        className="history-confirm-btn"
                        onClick={(e) => { e.stopPropagation(); cancelDelete(); }}
                      >
                        {t('chat.deleteNo')}
                      </button>
                    </div>
                  ) : (
                    <div className="history-timestamp">{conv.timestamp}</div>
                  )}
                </div>
                {!isEditing && !isConfirmingDelete && (
                  <button
                    className="history-action"
                    onClick={(e) => { e.stopPropagation(); requestDelete(conv.id); }}
                    aria-label={t('chat.delete')}
                    title={t('chat.delete')}
                  >
                    <DeleteOutlined size={14} />
                  </button>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {workingDir && (
        <div className="workspace-block">
          <div className="workspace-block-header" onClick={() => setTreeExpanded(!treeExpanded)}>
            <span className={`workspace-chevron ${treeExpanded ? 'expanded' : ''}`}>▸</span>
            <FolderOpenOutlined size={16} />
            <span className="workspace-block-path" title={workingDir}>
              {dirName || workingDir}
            </span>
            <button
              className="workspace-block-refresh"
              onClick={(e) => { e.stopPropagation(); refreshTree(); }}
              title={t('app.refresh')}
            >
              <RefreshOutlined size={12} />
            </button>
          </div>
          {treeExpanded && (
            <div className="workspace-block-tree" ref={treeRef}>
              <FileExplorer
                key={`tree-${workingDir}-${treeExpanded}`}
                workingDir={workingDir}
                onOpenFile={onOpenFile}
              />
            </div>
          )}
        </div>
      )}

      <div className="sidebar-footer">
        <button className="nav-item" onClick={onOpenSettings}>
          <SettingOutlined size={16} />
          <span>{t('app.settings')}</span>
        </button>
        <button className="nav-item profile-link">
          <UserOutlined size={16} />
          <span>{t('sidebar.agent')}</span>
        </button>
      </div>
    </aside>
  );
}
