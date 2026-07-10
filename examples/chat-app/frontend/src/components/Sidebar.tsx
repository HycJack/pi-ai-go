import { Plus, MessageSquare, Settings, Trash2, ChevronLeft, ChevronRight, Check, X } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';

interface SidebarProps {
  conversations: { id: string; title: string; timestamp: string }[];
  activeConversation: string | null;
  collapsed: boolean;
  onToggleCollapse: () => void;
  onSelectConversation: (id: string) => void;
  onCreateNewConversation: () => void;
  onDeleteConversation: (id: string) => void;
  onRenameConversation: (id: string, title: string) => void;
  onOpenSettings: () => void;
}

function EditableTitle({ title, onSave }: { title: string; onSave: (val: string) => void }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(title);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) inputRef.current?.focus();
  }, [editing]);

  const save = () => {
    const trimmed = value.trim();
    if (trimmed && trimmed !== title) onSave(trimmed);
    setValue(trimmed || title);
    setEditing(false);
  };

  const cancel = () => {
    setValue(title);
    setEditing(false);
  };

  if (editing) {
    return (
      <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
        <input
          ref={inputRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') save(); if (e.key === 'Escape') cancel(); }}
          className="flex-1 text-sm bg-slate-600 border border-blue-500 rounded px-1 py-0.5 text-white outline-none min-w-0"
        />
        <button onClick={save} className="p-0.5 hover:text-green-400"><Check className="w-3.5 h-3.5" /></button>
        <button onClick={cancel} className="p-0.5 hover:text-red-400"><X className="w-3.5 h-3.5" /></button>
      </div>
    );
  }

  return (
    <span
      className="text-sm truncate cursor-text hover:text-blue-400 transition-colors flex-1 min-w-0"
      onDoubleClick={() => { setValue(title); setEditing(true); }}
      title="Double-click to rename"
    >
      {title}
    </span>
  );
}

export default function Sidebar({
  conversations,
  activeConversation,
  collapsed,
  onToggleCollapse,
  onSelectConversation,
  onCreateNewConversation,
  onDeleteConversation,
  onRenameConversation,
  onOpenSettings,
}: SidebarProps) {
  if (collapsed) {
    return (
      <div className="w-14 bg-slate-900 border-r border-slate-700 flex flex-col items-center py-3 h-full">
        <button
          onClick={onCreateNewConversation}
          className="p-2 mb-3 bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors border border-slate-600"
          title="New chat"
        >
          <Plus className="w-5 h-5" />
        </button>
        <div className="flex-1 overflow-y-auto w-full px-2 space-y-1">
          {conversations.map((conv) => (
            <button
              key={conv.id}
              onClick={() => onSelectConversation(conv.id)}
              className={`w-full p-2 rounded-lg transition-colors text-left ${
                activeConversation === conv.id ? 'bg-slate-700' : 'hover:bg-slate-800'
              }`}
              title={conv.title}
            >
              <MessageSquare className="w-5 h-5 text-slate-400 mx-auto" />
            </button>
          ))}
        </div>
        <button
          onClick={onOpenSettings}
          className="p-2 mt-2 hover:bg-slate-800 rounded-lg transition-colors"
          title="Settings"
        >
          <Settings className="w-5 h-5 text-slate-400" />
        </button>
        <button
          onClick={onToggleCollapse}
          className="p-2 mt-1 hover:bg-slate-800 rounded-lg transition-colors"
          title="Expand sidebar"
        >
          <ChevronRight className="w-5 h-5 text-slate-400" />
        </button>
      </div>
    );
  }

  return (
    <div className="w-64 bg-slate-900 border-r border-slate-700 flex flex-col h-full">
      <div className="flex items-center mx-3 mt-3 gap-2">
        <button
          onClick={onCreateNewConversation}
          className="flex-1 flex items-center justify-center gap-2 px-4 py-3 bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors border border-slate-600"
        >
          <Plus className="w-5 h-5" />
          <span className="text-sm font-medium">New chat</span>
        </button>
        <button
          onClick={onToggleCollapse}
          className="p-3 bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors border border-slate-600"
          title="Collapse sidebar"
        >
          <ChevronLeft className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto py-3">
        {conversations.map((conv) => (
          <div
            key={conv.id}
            className={`group flex items-center gap-3 px-3 py-2.5 mx-3 rounded-lg cursor-pointer transition-colors ${
              activeConversation === conv.id
                ? 'bg-slate-700'
                : 'hover:bg-slate-800'
            }`}
            onClick={() => onSelectConversation(conv.id)}
          >
            <MessageSquare className="w-5 h-5 text-slate-400 flex-shrink-0" />
            <div className="flex-1 min-w-0">
              <EditableTitle title={conv.title} onSave={(val) => onRenameConversation(conv.id, val)} />
              <p className="text-xs text-slate-500">{conv.timestamp}</p>
            </div>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onDeleteConversation(conv.id);
              }}
              className="opacity-0 group-hover:opacity-100 p-1 hover:bg-slate-600 rounded transition-all"
            >
              <Trash2 className="w-4 h-4 text-slate-400" />
            </button>
          </div>
        ))}
      </div>

      <div className="border-t border-slate-700 p-3">
        <button
          onClick={onOpenSettings}
          className="w-full flex items-center gap-3 px-3 py-2 hover:bg-slate-800 rounded-lg transition-colors"
        >
          <Settings className="w-5 h-5 text-slate-400" />
          <span className="text-sm">Settings</span>
        </button>
      </div>
    </div>
  );
}
