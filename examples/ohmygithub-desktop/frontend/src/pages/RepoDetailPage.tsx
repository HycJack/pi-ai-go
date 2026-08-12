import React, { useState, useEffect, useCallback } from 'react';
import { API, Repo, FileContent } from '../lib/api';

interface RepoDetailPageProps {
  repoFullName: string;
  addToast: (message: string, type?: string) => void;
  onNavigate: (page: string) => void;
  onOpenExternal: (url: string) => void;
  onBack?: () => void; // 返回上一页
}

const languageColors: Record<string, string> = {
  TypeScript: '#3178c6',
  JavaScript: '#f1e05a',
  Go: '#00ADD8',
  Rust: '#dea584',
  Python: '#3572A5',
  Java: '#b07219',
  'C#': '#178600',
  Ruby: '#701516',
  Swift: '#F05138',
  Kotlin: '#A97BFF',
  HTML: '#e34c26',
  CSS: '#563d7c',
  Shell: '#89e051',
  Dockerfile: '#384d54',
};

// 文件扩展名 → 是否可渲染为文本
const textExtensions = new Set([
  'md', 'txt', 'json', 'yaml', 'yml', 'js', 'ts', 'tsx', 'jsx', 'go', 'rs',
  'py', 'java', 'c', 'h', 'cpp', 'hpp', 'cs', 'rb', 'php', 'sh', 'bash',
  'css', 'scss', 'less', 'html', 'xml', 'sql', 'toml', 'ini', 'cfg', 'conf',
  'env', 'gitignore', 'dockerfile', 'makefile', 'mod', 'sum', 'lock',
]);

// 文件扩展名 → 图标
function fileIcon(name: string, isDir: boolean): string {
  if (isDir) return '📁';
  const ext = name.split('.').pop()?.toLowerCase() || '';
  if (ext === 'md') return '📝';
  if (['js', 'ts', 'tsx', 'jsx'].includes(ext)) return '📜';
  if (['json', 'yaml', 'yml', 'toml'].includes(ext)) return '⚙️';
  if (['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp'].includes(ext)) return '🖼️';
  if (['go', 'rs', 'py', 'java', 'c', 'cpp'].includes(ext)) return '🔧';
  return '📄';
}

function isTextFile(name: string): boolean {
  const lower = name.toLowerCase();
  if (textExtensions.has(lower)) return true; // 如 dockerfile, makefile
  const ext = lower.split('.').pop() || '';
  return textExtensions.has(ext);
}

// base64 → UTF-8 字符串（处理多字节字符）
function decodeBase64Utf8(b64: string): string {
  try {
    const clean = b64.replace(/\s/g, '');
    const binary = atob(clean);
    // 处理 UTF-8：将 binary string 转为 Uint8Array 再 decode
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return new TextDecoder('utf-8').decode(bytes);
  } catch {
    return '';
  }
}

interface TreeNode {
  name: string;
  path: string;
  type: string; // 'file' | 'dir'
  size: number;
  htmlUrl: string;
  children?: TreeNode[];
  loaded?: boolean; // 子目录是否已加载
  loading?: boolean;
}

export default function RepoDetailPage({ repoFullName, addToast, onNavigate, onOpenExternal, onBack }: RepoDetailPageProps) {
  const [repo, setRepo] = useState<Repo | null>(null);
  const [rootNodes, setRootNodes] = useState<TreeNode[]>([]);
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());
  const [currentFile, setCurrentFile] = useState<TreeNode | null>(null);
  const [fileContent, setFileContent] = useState<string>('');
  const [fileLoading, setFileLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingDirs, setLoadingDirs] = useState<Set<string>>(new Set());

  // 加载仓库元信息
  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.SearchRepos(`repo:${repoFullName}`);
      const result = JSON.parse(str);
      const found = result.items?.find((r: Repo) => r.fullName === repoFullName);
      setRepo(found || result.items?.[0] || null);
    } catch (e: any) {
      addToast('Failed to load repo detail: ' + (e?.message || 'error'), 'error');
    } finally {
      setLoading(false);
    }
  }, [repoFullName, addToast]);

  // 加载根目录文件列表
  const loadRootContents = useCallback(async () => {
    try {
      const str = await API.GetRepoContents(repoFullName, '');
      if (!str) {
        setRootNodes([]);
        return;
      }
      const items: FileContent[] = JSON.parse(str);
      // GitHub 风格排序：文件夹在前，然后按名称
      const sorted = [...items].sort((a, b) => {
        if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      setRootNodes(sorted.map(item => ({
        name: item.name,
        path: item.path,
        type: item.type,
        size: item.size,
        htmlUrl: item.htmlUrl,
        loaded: item.type !== 'dir',
        loading: false,
      })));
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Failed to load contents: ' + msg, 'error');
    }
  }, [repoFullName, addToast]);

  // 加载子目录
  const loadChildren = useCallback(async (node: TreeNode): Promise<TreeNode[]> => {
    try {
      const str = await API.GetRepoContents(repoFullName, node.path);
      if (!str) return [];
      const items: FileContent[] = JSON.parse(str);
      return items
        .filter(item => item.type === 'dir' || item.type === 'file')
        .sort((a, b) => {
          if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
          return a.name.localeCompare(b.name);
        })
        .map(item => ({
          name: item.name,
          path: item.path,
          type: item.type,
          size: item.size,
          htmlUrl: item.htmlUrl,
          loaded: item.type !== 'dir',
          loading: false,
        }));
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Failed to load folder: ' + msg, 'error');
      return [];
    }
  }, [repoFullName, addToast]);

  // 切换文件夹展开/折叠
  const toggleDir = useCallback(async (node: TreeNode) => {
    const isExpanded = expandedPaths.has(node.path);
    if (isExpanded) {
      // 折叠
      setExpandedPaths(prev => {
        const next = new Set(prev);
        next.delete(node.path);
        return next;
      });
      return;
    }

    // 展开
    setExpandedPaths(prev => new Set(prev).add(node.path));

    // 如果子目录未加载，先加载
    if (!node.loaded) {
      setLoadingDirs(prev => new Set(prev).add(node.path));
      const children = await loadChildren(node);
      // 更新 rootNodes 中对应节点的 children
      setRootNodes(prev => updateNodeChildren(prev, node.path, children, true));
      setLoadingDirs(prev => {
        const next = new Set(prev);
        next.delete(node.path);
        return next;
      });
    }
  }, [expandedPaths, loadChildren]);

  // 递归更新节点 children
  function updateNodeChildren(nodes: TreeNode[], targetPath: string, children: TreeNode[], markLoaded: boolean): TreeNode[] {
    return nodes.map(n => {
      if (n.path === targetPath) {
        return { ...n, children, loaded: markLoaded ? true : n.loaded };
      }
      if (n.children && targetPath.startsWith(n.path + '/')) {
        return { ...n, children: updateNodeChildren(n.children, targetPath, children, markLoaded) };
      }
      return n;
    });
  }

  // 点击文件：加载内容
  const handleFileClick = useCallback(async (node: TreeNode) => {
    setCurrentFile(node);
    setFileContent('');
    setFileLoading(true);
    try {
      // 如果节点已有 content（来自单文件 API 响应），直接用；否则调用 API
      const str = await API.GetRepoContents(repoFullName, node.path);
      const items: FileContent[] = JSON.parse(str);
      const file = items[0];
      if (file?.content && file.encoding === 'base64') {
        const decoded = decodeBase64Utf8(file.content);
        setFileContent(decoded);
      } else if (file?.content) {
        setFileContent(file.content);
      } else {
        setFileContent('');
      }
    } catch (e: any) {
      setFileContent('（无法加载文件内容：' + (e?.message || 'error') + '）');
    } finally {
      setFileLoading(false);
    }
  }, [repoFullName, addToast]);

  // 面包屑：当前路径分段
  const breadcrumbs = currentFile ? currentFile.path.split('/') : [];

  useEffect(() => {
    loadDetail();
    loadRootContents();
  }, [loadDetail, loadRootContents]);

  // 渲染文件树节点
  const renderNode = (node: TreeNode, depth: number): React.ReactNode => {
    const isExpanded = expandedPaths.has(node.path);
    const isLoading = loadingDirs.has(node.path);
    const isDir = node.type === 'dir';
    const isActive = currentFile?.path === node.path;

    return (
      <div key={node.path}>
        <div
          onClick={() => isDir ? toggleDir(node) : handleFileClick(node)}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            padding: '4px 8px',
            paddingLeft: 8 + depth * 16,
            cursor: 'pointer',
            borderRadius: 4,
            background: isActive ? 'var(--accent-soft, rgba(88,166,255,0.15))' : 'transparent',
            color: isActive ? 'var(--accent)' : 'var(--text-primary)',
            fontSize: 13,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
          onMouseEnter={(e) => { if (!isActive) e.currentTarget.style.background = 'var(--bg-hover)'; }}
          onMouseLeave={(e) => { if (!isActive) e.currentTarget.style.background = 'transparent'; }}
          title={node.name}
        >
          <span style={{ width: 14, textAlign: 'center', fontSize: 12 }}>
            {isDir ? (isLoading ? '⏳' : (isExpanded ? '▾' : '▸')) : ''}
          </span>
          <span style={{ fontSize: 14 }}>{fileIcon(node.name, isDir)}</span>
          <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>{node.name}</span>
          {!isDir && node.size > 0 && (
            <span style={{ fontSize: 10, color: 'var(--text-tertiary)' }}>
              {node.size < 1024 ? `${node.size} B` : `${(node.size / 1024).toFixed(1)} KB`}
            </span>
          )}
        </div>
        {isDir && isExpanded && node.children && node.children.length > 0 && (
          <div>
            {node.children.map(child => renderNode(child, depth + 1))}
          </div>
        )}
        {isDir && isExpanded && node.children && node.children.length === 0 && !isLoading && (
          <div style={{ paddingLeft: 32 + depth * 16, fontSize: 11, color: 'var(--text-tertiary)', padding: '4px 8px' }}>
            (empty)
          </div>
        )}
      </div>
    );
  };

  if (loading) {
    return <div className="loading-spinner"><div className="spinner" /></div>;
  }

  if (!repo) {
    return (
      <div className="empty-state">
        <h3>Repository not found</h3>
        <p>Could not load details for "{repoFullName}".</p>
        <button className="btn btn-primary btn-sm" onClick={loadDetail}>Retry</button>
      </div>
    );
  }

  const langColor = languageColors[repo.language] || '#8b949e';
  const githubUrl = `https://github.com/${repo.fullName}`;

  return (
    <div className="fade-in" style={{ paddingBottom: 32, display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* 顶部仓库信息条 + 返回按钮 */}
      <div style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-muted)',
        borderRadius: 'var(--radius-md)',
        padding: '12px 16px',
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        flexWrap: 'wrap',
      }}>
        {onBack && (
          <button
            className="btn btn-ghost btn-sm"
            onClick={onBack}
            title="返回"
            style={{ padding: '4px 10px' }}
          >
            ← 返回
          </button>
        )}
        <div style={{ flex: 1, minWidth: 200 }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 600, color: 'var(--text-primary)' }}>
            {repo.fullName}
          </h2>
          {repo.description && (
            <p style={{ margin: '4px 0 0', color: 'var(--text-secondary)', fontSize: 12 }}>
              {repo.description}
            </p>
          )}
        </div>
        <div style={{ display: 'flex', gap: 12, fontSize: 11, color: 'var(--text-secondary)', flexWrap: 'wrap' }}>
          {repo.language && (
            <span>
              <span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: langColor, marginRight: 4, verticalAlign: 'middle' }} />
              {repo.language}
            </span>
          )}
          <span>★ {repo.stars.toLocaleString()}</span>
          <span>⑂ {repo.forks.toLocaleString()}</span>
          <span>! {repo.openIssues.toLocaleString()}</span>
          <span>{repo.private ? 'Private' : 'Public'}</span>
        </div>
        <button
          className="btn btn-ghost btn-sm"
          onClick={() => onOpenExternal(githubUrl)}
          title="Open on GitHub"
        >
          ↗ GitHub
        </button>
      </div>

      {/* 快捷入口 */}
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <button className="btn btn-sm" onClick={() => onNavigate('pull-requests')}>🔀 Pull Requests</button>
        <button className="btn btn-sm" onClick={() => onNavigate('issues')}>◯ Issues</button>
        <button className="btn btn-sm" onClick={() => onNavigate('actions')}>▶ Actions</button>
      </div>

      {/* GitHub 风格文件浏览器 */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: '280px 1fr',
        gap: 12,
        minHeight: 500,
      }}>
        {/* 左侧文件树 */}
        <div style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-muted)',
          borderRadius: 'var(--radius-md)',
          padding: 8,
          overflow: 'auto',
          maxHeight: 'calc(100vh - 280px)',
        }}>
          <div style={{
            fontSize: 11, color: 'var(--text-tertiary)', textTransform: 'uppercase',
            letterSpacing: 0.5, padding: '4px 8px 8px', borderBottom: '1px solid var(--border-muted)',
            marginBottom: 4,
          }}>
            Files
          </div>
          {rootNodes.length === 0 ? (
            <div style={{ padding: 16, fontSize: 12, color: 'var(--text-tertiary)' }}>Loading...</div>
          ) : (
            rootNodes.map(node => renderNode(node, 0))
          )}
        </div>

        {/* 右侧文件内容 */}
        <div style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-muted)',
          borderRadius: 'var(--radius-md)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          maxHeight: 'calc(100vh - 280px)',
        }}>
          {/* 面包屑 */}
          <div style={{
            padding: '8px 16px',
            borderBottom: '1px solid var(--border-muted)',
            fontSize: 12,
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            flexWrap: 'wrap',
            background: 'var(--bg-secondary, var(--bg-elevated))',
          }}>
            <span
              style={{ cursor: 'pointer', color: 'var(--accent)' }}
              onClick={() => { setCurrentFile(null); setFileContent(''); }}
            >
              {repo.name}
            </span>
            {breadcrumbs.map((seg, i) => (
              <React.Fragment key={i}>
                <span style={{ color: 'var(--text-tertiary)' }}>/</span>
                <span style={{ color: i === breadcrumbs.length - 1 ? 'var(--text-primary)' : 'var(--text-secondary)' }}>
                  {seg}
                </span>
              </React.Fragment>
            ))}
            {currentFile && (
              <button
                className="btn btn-ghost btn-sm"
                style={{ marginLeft: 'auto', padding: '2px 8px', fontSize: 11 }}
                onClick={() => onOpenExternal(currentFile.htmlUrl)}
                title="Open on GitHub"
              >
                ↗ View raw
              </button>
            )}
          </div>

          {/* 内容区 */}
          <div style={{ flex: 1, overflow: 'auto' }}>
            {!currentFile ? (
              <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-tertiary)', fontSize: 13 }}>
                <div style={{ fontSize: 32, marginBottom: 8 }}>📂</div>
                <div>从左侧选择文件查看内容</div>
                <div style={{ fontSize: 11, marginTop: 4 }}>选择文件夹可展开/折叠</div>
              </div>
            ) : fileLoading ? (
              <div className="loading-spinner" style={{ padding: 32 }}>
                <div className="spinner" />
              </div>
            ) : fileContent ? (
              <pre style={{
                margin: 0, padding: 16,
                whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                fontFamily: 'var(--font-mono)', fontSize: 12,
                color: 'var(--text-primary)', lineHeight: 1.5,
              }}>
                {fileContent}
              </pre>
            ) : (
              <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-tertiary)', fontSize: 13 }}>
                <div style={{ fontSize: 24, marginBottom: 8 }}>📄</div>
                <div>{currentFile.name}</div>
                <div style={{ fontSize: 11, marginTop: 4 }}>
                  {isTextFile(currentFile.name) ? '文件为空' : '此文件类型无法预览（二进制文件或图片）'}
                </div>
                <button
                  className="btn btn-ghost btn-sm"
                  style={{ marginTop: 12 }}
                  onClick={() => onOpenExternal(currentFile.htmlUrl)}
                >
                  ↗ 在 GitHub 查看
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
