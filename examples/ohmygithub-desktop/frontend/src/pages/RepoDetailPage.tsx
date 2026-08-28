import React, { useState, useEffect, useCallback } from 'react';
import { API, Repo, FileContent, CloneRepoResult } from '../lib/api';
import CodeViewer from '../components/CodeViewer';
import { Button } from '../components/ui/button';
import { Badge } from '../components/ui/badge';
import { cn } from '@/lib/utils';
import {
  Loader2,
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen,
  File,
  FileText,
  FileCode,
  FileJson,
  FileCog,
  FileImage,
  FileArchive,
  FileLock,
  Terminal,
  Package,
  Globe,
  Code2,
  Database,
  Music,
  Film,
  GitBranch,
  ExternalLink,
  ArrowLeft,
  GitPullRequest,
  Circle,
  Play,
  Download,
  FolderTree,
  BookOpen,
  Shield,
  Hammer,
  Key,
  Sailboat,
} from 'lucide-react';

interface RepoDetailPageProps {
  repoFullName: string;
  addToast: (message: string, type?: string) => void;
  onNavigate: (page: string) => void;
  onOpenExternal: (url: string) => void;
  onBack?: () => void;
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

const textExtensions = new Set([
  'md', 'markdown', 'mdown', 'mkd', 'txt', 'log', 'json', 'jsonc',
  'yaml', 'yml', 'toml', 'ini', 'cfg', 'conf', 'js', 'mjs', 'cjs',
  'ts', 'mts', 'cts', 'tsx', 'jsx', 'go', 'rs', 'py', 'java', 'kt',
  'c', 'h', 'cpp', 'cc', 'cxx', 'hpp', 'hxx', 'cs', 'fs', 'rb', 'php',
  'sh', 'bash', 'zsh', 'fish', 'ps1', 'bat', 'cmd',
  'css', 'scss', 'sass', 'less', 'html', 'htm', 'xml', 'svg',
  'sql', 'graphql', 'gql', 'env', 'gitignore', 'npmignore',
  'dockerfile', 'makefile', 'mod', 'sum', 'lock',
  'vue', 'svelte', 'lua', 'r', 'clj', 'ex', 'erl', 'hs', 'ml', 'nim', 'zig',
  'csv', 'tsv', 'mmd', 'mermaid',
  'readme', 'license', 'editorconfig', 'prettierrc', 'eslintrc',
]);

function fileIconNode(name: string, isDir: boolean): React.ReactNode {
  if (isDir) {
    const lower = name.toLowerCase();
    if (lower === 'src' || lower === 'source') return <Folder className="size-3.5 text-blue-400" />;
    if (lower === 'test' || lower === 'tests' || lower === '__tests__') return <Folder className="size-3.5 text-green-400" />;
    if (lower === 'docs' || lower === 'doc') return <BookOpen className="size-3.5 text-amber-400" />;
    if (lower === '.github' || lower === '.gitlab') return <GitBranch className="size-3.5 text-purple-400" />;
    if (lower === 'node_modules') return <Package className="size-3.5 text-gray-400" />;
    if (lower === 'dist' || lower === 'build') return <Folder className="size-3.5 text-orange-400" />;
    if (lower === 'assets' || lower === 'public' || lower === 'static') return <Folder className="size-3.5 text-pink-400" />;
    return <Folder className="size-3.5 text-muted-foreground" />;
  }
  const lower = name.toLowerCase();
  if (lower === 'license' || lower === 'license.md' || lower === 'license.txt') return <Shield className="size-3.5 text-green-500" />;
  if (lower === 'readme.md' || lower === 'readme') return <BookOpen className="size-3.5 text-blue-400" />;
  if (lower === '.gitignore' || lower === '.npmignore') return <FileLock className="size-3.5 text-red-400" />;
  if (lower === 'dockerfile' || lower.startsWith('dockerfile.')) return <Sailboat className="size-3.5 text-cyan-400" />;
  if (lower === 'makefile') return <Hammer className="size-3.5 text-yellow-500" />;
  if (lower === 'package.json' || lower === 'package-lock.json') return <Package className="size-3.5 text-red-400" />;
  if (lower === 'go.mod' || lower === 'go.sum') return <Sailboat className="size-3.5 text-cyan-400" />;
  if (lower === 'cargo.toml' || lower === 'cargo.lock') return <FileCog className="size-3.5 text-orange-400" />;
  if (lower === '.env' || lower.startsWith('.env.')) return <Key className="size-3.5 text-yellow-400" />;

  const ext = lower.split('.').pop() || '';
  if (['md', 'markdown', 'mdown', 'mkd'].includes(ext)) return <FileText className="size-3.5 text-blue-400" />;
  if (['txt', 'log'].includes(ext)) return <FileText className="size-3.5 text-muted-foreground" />;
  if (['pdf'].includes(ext)) return <FileText className="size-3.5 text-red-400" />;
  if (['json', 'jsonc'].includes(ext)) return <FileJson className="size-3.5 text-yellow-400" />;
  if (['yaml', 'yml', 'toml', 'ini', 'cfg', 'conf'].includes(ext)) return <FileCog className="size-3.5 text-muted-foreground" />;
  if (['xml', 'svg'].includes(ext)) return <Globe className="size-3.5 text-orange-400" />;
  if (['html', 'htm'].includes(ext)) return <Globe className="size-3.5 text-orange-400" />;
  if (['css', 'scss', 'sass', 'less'].includes(ext)) return <FileCode className="size-3.5 text-blue-300" />;
  if (['vue', 'svelte'].includes(ext)) return <FileCode className="size-3.5 text-green-400" />;
  if (['js', 'mjs', 'cjs'].includes(ext)) return <FileCode className="size-3.5 text-yellow-300" />;
  if (['jsx'].includes(ext)) return <FileCode className="size-3.5 text-cyan-300" />;
  if (['ts', 'mts', 'cts'].includes(ext)) return <FileCode className="size-3.5 text-blue-400" />;
  if (['tsx'].includes(ext)) return <FileCode className="size-3.5 text-blue-300" />;
  if (ext === 'go') return <Sailboat className="size-3.5 text-cyan-400" />;
  if (ext === 'rs') return <FileCog className="size-3.5 text-orange-400" />;
  if (ext === 'py') return <Code2 className="size-3.5 text-yellow-400" />;
  if (ext === 'java') return <FileCode className="size-3.5 text-red-400" />;
  if (ext === 'kt') return <FileCode className="size-3.5 text-purple-400" />;
  if (ext === 'swift') return <FileCode className="size-3.5 text-orange-400" />;
  if (ext === 'rb') return <FileCode className="size-3.5 text-red-400" />;
  if (ext === 'php') return <FileCode className="size-3.5 text-purple-300" />;
  if (['c', 'h'].includes(ext)) return <FileCode className="size-3.5 text-blue-300" />;
  if (['cpp', 'cc', 'cxx', 'hpp', 'hxx'].includes(ext)) return <FileCode className="size-3.5 text-blue-400" />;
  if (ext === 'cs') return <FileCode className="size-3.5 text-green-400" />;
  if (ext === 'fs') return <FileCode className="size-3.5 text-purple-300" />;
  if (['sh', 'bash', 'zsh', 'fish'].includes(ext)) return <Terminal className="size-3.5 text-green-400" />;
  if (ext === 'ps1') return <Terminal className="size-3.5 text-blue-400" />;
  if (ext === 'bat' || ext === 'cmd') return <Terminal className="size-3.5 text-gray-400" />;
  if (ext === 'sql') return <Database className="size-3.5 text-blue-400" />;
  if (['graphql', 'gql'].includes(ext)) return <Code2 className="size-3.5 text-pink-400" />;
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'ico', 'bmp'].includes(ext)) return <FileImage className="size-3.5 text-green-400" />;
  if (['mp3', 'wav', 'ogg', 'flac'].includes(ext)) return <Music className="size-3.5 text-pink-400" />;
  if (['mp4', 'webm', 'avi', 'mov', 'mkv'].includes(ext)) return <Film className="size-3.5 text-purple-400" />;
  if (['zip', 'tar', 'gz', 'rar', '7z'].includes(ext)) return <FileArchive className="size-3.5 text-gray-400" />;
  if (['mmd', 'mermaid'].includes(ext)) return <Globe className="size-3.5 text-cyan-400" />;
  if (['csv', 'tsv'].includes(ext)) return <Database className="size-3.5 text-green-400" />;
  return <File className="size-3.5 text-muted-foreground" />;
}

function isTextFile(name: string): boolean {
  const lower = name.toLowerCase();
  if (textExtensions.has(lower)) return true;
  const ext = lower.split('.').pop() || '';
  return textExtensions.has(ext);
}

function decodeBase64Utf8(b64: string): string {
  try {
    const clean = b64.replace(/\s/g, '');
    const binary = atob(clean);
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
  type: string;
  size: number;
  htmlUrl: string;
  children?: TreeNode[];
  loaded?: boolean;
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
  const [cloning, setCloning] = useState(false);

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const str = await API.GetRepo(repoFullName);
      const repoData: Repo = JSON.parse(str);
      setRepo(repoData);
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Failed to load repo detail: ' + msg, 'error');
    } finally {
      setLoading(false);
    }
  }, [repoFullName, addToast]);

  const readmeNames = ['README.md', 'readme.md', 'README.MD', 'Readme.md', 'README', 'readme', 'Readme', 'README.txt'];

  const loadRootContents = useCallback(async () => {
    try {
      const str = await API.GetRepoContents(repoFullName, '');
      if (!str) {
        setRootNodes([]);
        return;
      }
      const items: FileContent[] = JSON.parse(str);
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

  const toggleDir = useCallback(async (node: TreeNode) => {
    const isExpanded = expandedPaths.has(node.path);
    if (isExpanded) {
      setExpandedPaths(prev => {
        const next = new Set(prev);
        next.delete(node.path);
        return next;
      });
      return;
    }

    setExpandedPaths(prev => new Set(prev).add(node.path));

    if (!node.loaded) {
      setLoadingDirs(prev => new Set(prev).add(node.path));
      const children = await loadChildren(node);
      setRootNodes(prev => updateNodeChildren(prev, node.path, children, true));
      setLoadingDirs(prev => {
        const next = new Set(prev);
        next.delete(node.path);
        return next;
      });
    }
  }, [expandedPaths, loadChildren]);

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

  const handleFileClick = useCallback(async (node: TreeNode) => {
    setCurrentFile(node);
    setFileContent('');
    setFileLoading(true);
    try {
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

  const breadcrumbs = currentFile ? currentFile.path.split('/') : [];

  const handleClone = useCallback(async () => {
    if (cloning) return;
    setCloning(true);
    addToast('正在下载仓库源码...', 'info');
    try {
      const str = await API.CloneRepo(repoFullName, '', '');
      const result: CloneRepoResult = JSON.parse(str);
      addToast(`已下载到: ${result.path}`, 'success');
    } catch (e: any) {
      const msg = e?.message || e?.error || (typeof e === 'string' ? e : 'unknown error');
      addToast('Clone 失败: ' + msg, 'error');
    } finally {
      setCloning(false);
    }
  }, [repoFullName, repo, cloning, addToast]);

  useEffect(() => {
    loadDetail();
    loadRootContents();
  }, [loadDetail, loadRootContents]);

  useEffect(() => {
    if (rootNodes.length === 0) return;
    if (currentFile) return;
    const readmeNode = rootNodes.find(n => readmeNames.includes(n.name));
    if (readmeNode && readmeNode.type === 'file') {
      handleFileClick(readmeNode);
    }
  }, [rootNodes, currentFile, handleFileClick]);

  const renderNode = (node: TreeNode, depth: number): React.ReactNode => {
    const isExpanded = expandedPaths.has(node.path);
    const isLoading = loadingDirs.has(node.path);
    const isDir = node.type === 'dir';
    const isActive = currentFile?.path === node.path;

    return (
      <div key={node.path}>
        <div
          onClick={() => isDir ? toggleDir(node) : handleFileClick(node)}
          className={cn(
            'flex items-center gap-1.5 py-1 px-2 rounded cursor-pointer text-[13px] truncate select-none transition-colors',
            isActive
              ? 'bg-accent text-accent-foreground'
              : 'text-foreground hover:bg-muted',
          )}
          style={{ paddingLeft: 8 + depth * 16 }}
          title={node.name}
        >
          <span className="w-3.5 text-center text-xs text-muted-foreground">
            {isDir ? (
              isLoading ? (
                <Loader2 className="size-3 animate-spin" />
              ) : isExpanded ? (
                <ChevronDown className="size-3" />
              ) : (
                <ChevronRight className="size-3" />
              )
            ) : null}
          </span>
          <span className="flex-shrink-0">
            {isDir ? (
              isExpanded ? <FolderOpen className="size-3.5" /> : fileIconNode(node.name, true)
            ) : (
              fileIconNode(node.name, false)
            )}
          </span>
          <span className="flex-1 truncate">{node.name}</span>
          {!isDir && node.size > 0 && (
            <span className="text-xs text-muted-foreground ml-auto flex-shrink-0">
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
          <div
            className="text-xs text-muted-foreground py-1 px-2"
            style={{ paddingLeft: 32 + depth * 16 }}
          >
            (empty)
          </div>
        )}
      </div>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-12">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!repo) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-muted-foreground text-center gap-3">
        <h3 className="text-base font-semibold text-secondary-foreground">Repository not found</h3>
        <p className="text-[13px] max-w-80">Could not load details for "{repoFullName}".</p>
        <Button variant="default" size="sm" onClick={loadDetail}>Retry</Button>
      </div>
    );
  }

  const langColor = languageColors[repo.language] || '#8b949e';
  const githubUrl = `https://github.com/${repo.fullName}`;

  return (
    <div className="animate-fade-in pb-8 flex flex-col gap-3">
      {/* Top repo info bar + back button */}
      <div className="flex items-center gap-3 flex-wrap bg-card border border-border rounded-md px-4 py-3">
        {onBack && (
          <Button variant="ghost" size="sm" onClick={onBack} title="返回" className="px-2.5">
            <ArrowLeft className="size-3.5" />
            返回
          </Button>
        )}
        <div className="flex-1 min-w-[200px]">
          <h2 className="m-0 text-base font-semibold text-foreground">
            {repo.fullName}
          </h2>
          {repo.description && (
            <p className="mt-1 text-secondary-foreground text-xs">
              {repo.description}
            </p>
          )}
        </div>
        <div className="flex gap-3 text-xs text-secondary-foreground flex-wrap items-center">
          {repo.language && (
            <span className="flex items-center gap-1">
              <span
                className="inline-block w-2.5 h-2.5 rounded-full"
                style={{ background: langColor }}
              />
              {repo.language}
            </span>
          )}
          <span className="flex items-center gap-1">★ {repo.stars.toLocaleString()}</span>
          <span className="flex items-center gap-1">⑂ {repo.forks.toLocaleString()}</span>
          <span className="flex items-center gap-1">! {repo.openIssues.toLocaleString()}</span>
          <Badge variant={repo.private ? 'destructive' : 'secondary'} className="text-xs px-1.5 py-0">
            {repo.private ? 'Private' : 'Public'}
          </Badge>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onOpenExternal(githubUrl)}
          title="Open on GitHub"
        >
          <ExternalLink className="size-3.5" />
          GitHub
        </Button>
      </div>

      {/* Quick links */}
      <div className="flex gap-2 flex-wrap items-center">
        <Button variant="outline" size="sm" onClick={() => onNavigate('pull-requests')}>
          <GitPullRequest className="size-3.5" />
          Pull Requests
        </Button>
        <Button variant="outline" size="sm" onClick={() => onNavigate('issues')}>
          <Circle className="size-3.5" />
          Issues
        </Button>
        <Button variant="outline" size="sm" onClick={() => onNavigate('actions')}>
          <Play className="size-3.5" />
          Actions
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={handleClone}
          disabled={cloning}
          title="下载仓库源码到本地"
          className="flex items-center gap-1"
        >
          {cloning ? (
            <>
              <Loader2 className="size-3 animate-spin" />
              下载中...
            </>
          ) : (
            <>
              <Download className="size-3.5" />
              Clone to local
            </>
          )}
        </Button>
      </div>

      {/* GitHub-style file browser */}
      <div className="grid grid-cols-[280px_1fr] gap-3 min-h-[500px]">
        {/* Left file tree */}
        <div className="bg-card border border-border rounded-md p-2 overflow-auto max-h-[calc(100vh-280px)]">
          <div className="text-xs text-muted-foreground uppercase tracking-wider px-2 pb-2 border-b border-border mb-1">
            <FolderTree className="size-3.5 inline-block mr-1 -mt-0.5" />
            Files
          </div>
          {rootNodes.length === 0 ? (
            <div className="p-4 text-xs text-muted-foreground">Loading...</div>
          ) : (
            rootNodes.map(node => renderNode(node, 0))
          )}
        </div>

        {/* Right file content */}
        <div className="bg-card border border-border rounded-md flex flex-col overflow-hidden max-h-[calc(100vh-280px)]">
          {/* Breadcrumbs */}
          <div className="px-4 py-2 border-b border-border text-xs flex items-center gap-1.5 flex-wrap bg-secondary/50">
            <span
              className="cursor-pointer text-primary font-medium hover:underline"
              onClick={() => { setCurrentFile(null); setFileContent(''); }}
            >
              {repo.name}
            </span>
            {breadcrumbs.map((seg, i) => (
              <React.Fragment key={i}>
                <span className="text-muted-foreground">/</span>
                <span className={cn(i === breadcrumbs.length - 1 ? 'text-foreground font-medium' : 'text-secondary-foreground')}>
                  {seg}
                </span>
              </React.Fragment>
            ))}
            {currentFile && (
              <Button
                variant="ghost"
                size="sm"
                className="ml-auto px-2 py-0.5 h-auto text-xs"
                onClick={() => onOpenExternal(currentFile.htmlUrl)}
                title="Open on GitHub"
              >
                <ExternalLink className="size-3" />
                View raw
              </Button>
            )}
          </div>

          {/* Content area */}
          <div className="flex-1 overflow-auto">
            {!currentFile ? (
              <div className="p-8 text-center text-muted-foreground text-[13px]">
                <FolderOpen className="size-8 mx-auto mb-2 opacity-40" />
                <div>从左侧选择文件查看内容</div>
                <div className="text-xs mt-1">选择文件夹可展开/折叠</div>
              </div>
            ) : fileLoading ? (
              <div className="flex items-center justify-center p-8">
                <Loader2 className="size-6 animate-spin text-muted-foreground" />
              </div>
            ) : fileContent ? (
              <CodeViewer fileName={currentFile.name} content={fileContent} />
            ) : (
              <div className="p-8 text-center text-muted-foreground text-[13px]">
                <FileText className="size-6 mx-auto mb-2 opacity-50" />
                <div>{currentFile.name}</div>
                <div className="text-xs mt-1">
                  {isTextFile(currentFile.name) ? '文件为空' : '此文件类型无法预览（二进制文件或图片）'}
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="mt-3"
                  onClick={() => onOpenExternal(currentFile.htmlUrl)}
                >
                  <ExternalLink className="size-3.5" />
                  在 GitHub 查看
                </Button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
