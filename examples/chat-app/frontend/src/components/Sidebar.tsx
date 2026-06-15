import { Plus, MessageSquare, Settings, Trash2 } from 'lucide-react';

interface SidebarProps {
  conversations: { id: string; title: string; timestamp: string }[];
  activeConversation: string | null;
  onSelectConversation: (id: string) => void;
  onCreateNewConversation: () => void;
  onDeleteConversation: (id: string) => void;
  onOpenSettings: () => void;
}

export default function Sidebar({
  conversations,
  activeConversation,
  onSelectConversation,
  onCreateNewConversation,
  onDeleteConversation,
  onOpenSettings,
}: SidebarProps) {
  return (
    <div className="w-64 bg-slate-900 border-r border-slate-700 flex flex-col h-full">
      <button
        onClick={onCreateNewConversation}
        className="mx-3 mt-3 flex items-center justify-center gap-2 px-4 py-3 bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors border border-slate-600"
      >
        <Plus className="w-5 h-5" />
        <span className="text-sm font-medium">New chat</span>
      </button>

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
              <p className="text-sm truncate">{conv.title}</p>
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
