import { useState } from 'react';
import { Eye, EyeOff, Download, Copy, Check } from 'lucide-react';

interface GeogebraHTMLPreviewProps {
  html: string;
}

function saveHTMLToFile(html: string) {
  const blob = new Blob([html], { type: 'text/html;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `geogebra-${Date.now()}.html`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export default function GeogebraHTMLPreview({ html }: GeogebraHTMLPreviewProps) {
  const [showPreview, setShowPreview] = useState(false);
  const [copied, setCopied] = useState(false);

  const copyHTML = () => {
    navigator.clipboard.writeText(html).catch(() => {});
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <div className="geogebra-block">
      <button
        className="geogebra-block-btn"
        onClick={() => setShowPreview(!showPreview)}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
        </svg>
        <span className="geogebra-block-text">GeoGebra 课件</span>
        <span className="geogebra-block-badge">HTML</span>
        {showPreview ? <EyeOff size={16} /> : <Eye size={16} />}
      </button>

      <div className="geogebra-block-actions">
        <button className="geogebra-action-btn" onClick={() => saveHTMLToFile(html)} title="保存到本地">
          <Download size={14} />
          保存
        </button>
        <button className="geogebra-action-btn" onClick={copyHTML}>
          {copied ? <Check size={14} /> : <Copy size={14} />}
          {copied ? '已复制' : '复制 HTML'}
        </button>
      </div>

      {showPreview && (
        <div className="geogebra-preview-body">
          <iframe
            className="geogebra-preview-iframe"
            srcDoc={html}
            title="GeoGebra Preview"
            sandbox="allow-scripts allow-same-origin allow-forms"
          />
        </div>
      )}
    </div>
  );
}
