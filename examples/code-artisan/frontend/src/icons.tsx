// SVG icon components - simplified from lucide-react style
function createIcon(paths: string, viewBox = '0 0 24 24') {
  return ({ size = 24, className }: { size?: number; className?: string }) => (
    <svg
      width={size}
      height={size}
      viewBox={viewBox}
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
    >
      {paths.split('|').map((p, i) => {
        const [tag, ...rest] = p.split(' ');
        if (tag === 'path') return <path key={i} {...parseAttrs(rest.join(' '))} />;
        if (tag === 'circle') return <circle key={i} {...parseAttrs(rest.join(' '))} />;
        if (tag === 'line') return <line key={i} {...parseAttrs(rest.join(' '))} />;
        if (tag === 'polyline') return <polyline key={i} {...parseAttrs(rest.join(' '))} />;
        if (tag === 'rect') return <rect key={i} {...parseAttrs(rest.join(' '))} />;
        return null;
      })}
    </svg>
  );
}

function parseAttrs(str: string): Record<string, string> {
  const attrs: Record<string, string> = {};
  str.replace(/(\w+)="([^"]*)"/g, (_, k, v) => { attrs[k] = v; return ''; });
  return attrs;
}

export const SendOutlined = createIcon('path d="M22 2L11 13"|path d="M22 2L15 22L11 13L2 9Z"');
export const StopOutlined = createIcon('rect x="4" y="4" width="16" height="16" rx="2"');
export const PlayOutlined = createIcon('polyline points="6 3 20 12 6 21 6 3"');
export const DeleteOutlined = createIcon('path d="M3 6h18"|path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6"|path d="M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2"|line x1="10" y1="11" x2="10" y2="17"|line x1="14" y1="11" x2="14" y2="17"');
export const PlusOutlined = createIcon('line x1="12" y1="5" x2="12" y2="19"|line x1="5" y1="12" x2="19" y2="12"');
export const CopyOutlined = createIcon('rect x="9" y="9" width="13" height="13" rx="2"|path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"');
export const CheckOutlined = createIcon('polyline points="20 6 9 17 4 12"');
export const CloseOutlined = createIcon('line x1="18" y1="6" x2="6" y2="18"|line x1="6" y1="6" x2="18" y2="18"');
export const SettingOutlined = createIcon('circle cx="12" cy="12" r="3"|path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-2 2 2 2 0 01-2-2v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 01-2-2 2 2 0 012-2h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 012-2 2 2 0 012 2v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06A1.65 1.65 0 0019.32 9a1.65 1.65 0 001.51 1H21a2 2 0 012 2 2 2 0 01-2 2h-.09a1.65 1.65 0 00-1.51 1z"');
export const HistoryOutlined = createIcon('circle cx="12" cy="12" r="10"|polyline points="12 6 12 12 16 14"');
export const CodeOutlined = createIcon('polyline points="16 18 22 12 16 6"|polyline points="8 6 2 12 8 18"');
export const RefreshOutlined = createIcon('polyline points="23 4 23 10 17 10"|path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"');
export const FolderOpenOutlined = createIcon('path d="M2 6a2 2 0 012-2h5l2 2h9a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"');
export const MessageOutlined = createIcon('path d="M21 15a2 2 0 01-2 2H7L3 21V5a2 2 0 012-2h14a2 2 0 012 2z"');
export const UserOutlined = createIcon('path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"|circle cx="12" cy="7" r="4"');
export const ChevronDownOutlined = createIcon('polyline points="6 9 12 15 18 9"');
export const Brain = createIcon('path d="M12 2C7.58 2 4 5.58 4 10v2c0 4.42 3.58 8 8 8s8-3.58 8-8v-2c0-4.42-3.58-8-8-8z"|path d="M8 10h8"|path d="M8 14h5"');
export const Paperclip = createIcon('path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48"');
export const CloseCircle = createIcon('circle cx="12" cy="12" r="10"|line x1="15" y1="9" x2="9" y2="15"|line x1="9" y1="9" x2="15" y2="15"');
