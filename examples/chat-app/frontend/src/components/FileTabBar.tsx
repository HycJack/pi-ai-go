import { useState } from 'react';
import { CodeOutlined } from '../icons';

export interface FileTab {
  id: string;        // file path as unique id
  label: string;     // file name
  filePath: string;
}

interface Props {
  tabs: FileTab[];
  activeTabId: string | null;
  onSelectTab: (id: string) => void;
  onCloseTab: (id: string) => void;
}

export default function FileTabBar({ tabs, activeTabId, onSelectTab, onCloseTab }: Props) {
  const [hoveredClose, setHoveredClose] = useState<string | null>(null);

  if (tabs.length === 0) return null;

  return (
    <div className="file-tabbar">
      {tabs.map((tab) => {
        const isActive = tab.id === activeTabId;
        return (
          <div
            key={tab.id}
            className={`file-tab ${isActive ? 'active' : ''}`}
            onClick={() => onSelectTab(tab.id)}
            onMouseDown={(e) => {
              if (e.button === 1) e.preventDefault();
            }}
            onAuxClick={(e) => {
              if (e.button !== 1) return;
              e.preventDefault();
              e.stopPropagation();
              onCloseTab(tab.id);
            }}
          >
            <span className="file-tab-icon">
              <CodeOutlined size={13} />
            </span>
            <span className="file-tab-label" title={tab.filePath}>
              {tab.label}
            </span>
            <button
              className="file-tab-close"
              onClick={(e) => { e.stopPropagation(); onCloseTab(tab.id); }}
              onMouseEnter={() => setHoveredClose(tab.id)}
              onMouseLeave={() => setHoveredClose(null)}
              title="Close"
              aria-label={`Close ${tab.label}`}
            >
              <svg width="11" height="11" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true">
                <line x1="2" y1="2" x2="8" y2="8" />
                <line x1="8" y1="2" x2="2" y2="8" />
              </svg>
            </button>
          </div>
        );
      })}
    </div>
  );
}
