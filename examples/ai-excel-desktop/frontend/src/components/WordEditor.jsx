// WordEditor - 在线 Word 文档查看与编辑（纯前端）
// 使用 mammoth.js 读取 .docx（→ HTML），html-docx-js 保存（HTML → .docx）
// 富文本编辑使用 contentEditable，无需额外编辑器框架

import { useState, useRef, useCallback, useEffect } from "react";
import {
  FileText,
  Upload,
  Download,
  FileSpreadsheet,
  Bold,
  Italic,
  Underline,
  List,
  ListOrdered,
  Heading,
  AlignLeft,
  AlignCenter,
  AlignRight,
  Undo2,
  Redo2,
  Eye,
  Edit3,
  Loader2,
} from "lucide-react";

// 动态导入 mammoth（按需加载，不增大主包）
let mammothPromise = null;
function getMammoth() {
  if (!mammothPromise) {
    mammothPromise = import("mammoth");
  }
  return mammothPromise;
}

// 将 HTML 内容转换为 docx 格式的 Blob
async function htmlToDocxBlob(html, title) {
  const { Document, Packer, Paragraph, TextRun, HeadingLevel, Table, TableRow, TableCell } = await import("docx");

  // 解析 HTML，构建 docx 文档
  // 简单解析：按 <h1>/<h2>/<h3>/<p> 拆分
  const parts = html.match(/<h[123]>.*?<\/h[123]>|<p>.*?<\/p>|<table>.*?<\/table>/gi) || [];
  const children = [];

  for (const part of parts) {
    const trimmed = part.trim();
    if (trimmed.startsWith("<h1")) {
      const text = trimmed.replace(/<[^>]+>/g, "");
      children.push(new Paragraph({ heading: HeadingLevel.HEADING_1, children: [new TextRun({ text, bold: true, size: 28 })] }));
    } else if (trimmed.startsWith("<h2")) {
      const text = trimmed.replace(/<[^>]+>/g, "");
      children.push(new Paragraph({ heading: HeadingLevel.HEADING_2, children: [new TextRun({ text, bold: true, size: 24 })] }));
    } else if (trimmed.startsWith("<h3")) {
      const text = trimmed.replace(/<[^>]+>/g, "");
      children.push(new Paragraph({ heading: HeadingLevel.HEADING_3, children: [new TextRun({ text, bold: true, size: 20 })] }));
    } else if (trimmed.startsWith("<p")) {
      const text = trimmed.replace(/<[^>]+>/g, "");
      if (text.trim()) {
        children.push(new Paragraph({ children: [new TextRun({ text, size: 21 })] }));
      } else {
        children.push(new Paragraph({ spacing: { after: 200 }, children: [] }));
      }
    } else if (trimmed.startsWith("<table")) {
      // 简单表格，不解析 td/tr
      children.push(new Paragraph({ children: [new TextRun({ text: "[表格]", italics: true, color: "888888" })] }));
    }
  }

  if (children.length === 0) {
    // 没有识别出标题/段落，直接放全部纯文本
    const text = html.replace(/<[^>]+>/g, "");
    if (text.trim()) {
      children.push(new Paragraph({ children: [new TextRun({ text, size: 21 })] }));
    }
  }

  const doc = new Document({
    title: title || "文档",
    sections: [{ children }],
  });

  const blob = await Packer.toBlob(doc);
  return blob;
}

// ============================================================================
// WordEditor 组件
// ============================================================================
export function WordEditor({ file, docxBase64, fileName: propFileName, onClose }) {
  const editorRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [docContent, setDocContent] = useState("");
  const [fileName, setFileName] = useState(propFileName || "文档.docx");
  const [saving, setSaving] = useState(false);
  const [mode, setMode] = useState("edit"); // "view" | "edit"
  const [error, setError] = useState("");

  // 加载 docx 文件
  useEffect(() => {
    const loadDocx = async (source) => {
      setLoading(true);
      setError("");
      try {
        let arrayBuffer;

        if (source instanceof File) {
          setFileName(source.name);
          arrayBuffer = await source.arrayBuffer();
        } else if (typeof source === "string") {
          // base64 from Go backend
          setFileName(propFileName || "文档.docx");
          const binary = atob(source);
          const bytes = new Uint8Array(binary.length);
          for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
          }
          arrayBuffer = bytes.buffer;
        } else {
          return;
        }

        const { default: mammoth } = await getMammoth();
        const result = await mammoth.convertToHtml({ arrayBuffer });

        const html = result.value;
        const messages = result.messages;
        if (messages?.length > 0) {
          console.log("[WordEditor] mammoth messages:", messages);
        }
        setDocContent(html);

        const warnings = messages?.filter((m) => m.type === "warning");
        if (warnings?.length > 0) {
          setError(`解析警告: ${warnings[0].message}`);
        }
      } catch (e) {
        console.error("[WordEditor] load error:", e);
        setError(`文件加载失败: ${e.message || e}`);
      } finally {
        setLoading(false);
      }
    };

    if (file) {
      loadDocx(file);
    } else if (docxBase64) {
      loadDocx(docxBase64);
    }
  }, [file, docxBase64, propFileName]);

  // 内容变化时同步（用 MutationObserver 实时监听编辑内容）
  useEffect(() => {
    const el = editorRef.current;
    if (!el || mode !== "edit") return;

    const observer = new MutationObserver(() => {
      setDocContent(el.innerHTML);
    });

    observer.observe(el, {
      childList: true,
      subtree: true,
      characterData: true,
    });

    return () => observer.disconnect();
  }, [mode]);

  // 保存为 docx
  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      const html = editorRef.current?.innerHTML || docContent;
      const blob = await htmlToDocxBlob(html, fileName);

      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = fileName.replace(/\.docx$/i, "") + "_编辑后.docx";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (e) {
      console.error("[WordEditor] save error:", e);
      setError(`保存失败: ${e.message || e}`);
    } finally {
      setSaving(false);
    }
  }, [docContent, fileName]);

  // 格式化命令
  const execCmd = (cmd, val = null) => {
    document.execCommand(cmd, false, val);
    if (editorRef.current) editorRef.current.focus();
  };

  // 文件选择器 - 打开 docx
  const handleOpenFile = async () => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".docx";
    input.onchange = (e) => {
      const f = e.target.files?.[0];
      if (f) {
        // 通过父组件或直接处理
        const reader = new FileReader();
        reader.onload = async (ev) => {
          try {
            setLoading(true);
            setError("");
            setFileName(f.name);
            const arrayBuffer = ev.target.result;
            const { default: mammoth } = await getMammoth();
            const result = await mammoth.convertToHtml({ arrayBuffer });
            setDocContent(result.value);
          } catch (e) {
            setError(`打开失败: ${e.message}`);
          } finally {
            setLoading(false);
          }
        };
        reader.readAsArrayBuffer(f);
      }
    };
    input.click();
  };

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex flex-col items-center gap-3 text-foreground-muted">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm">正在解析 Word 文档...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col bg-white/5">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-1 border-b border-border/60 bg-background-elevated/80 px-3 py-2">
        {/* 文件操作 */}
        <ToolbarButton onClick={handleOpenFile} icon={Upload} label="打开" />
        <ToolbarButton onClick={handleSave} icon={Download} label="保存" disabled={saving} />
        <span className="mx-1 h-5 w-px bg-border" />

        {/* 视图切换 */}
        <ToolbarButton
          onClick={() => setMode(mode === "edit" ? "view" : "edit")}
          icon={mode === "edit" ? Eye : Edit3}
          label={mode === "edit" ? "预览" : "编辑"}
          active={mode === "edit"}
        />

        {mode === "edit" && (
          <>
            <span className="mx-1 h-5 w-px bg-border" />

            {/* 文本格式 */}
            <ToolbarButton onClick={() => execCmd("bold")} icon={Bold} label="粗体" />
            <ToolbarButton onClick={() => execCmd("italic")} icon={Italic} label="斜体" />
            <ToolbarButton onClick={() => execCmd("underline")} icon={Underline} label="下划线" />
            <span className="mx-1 h-5 w-px bg-border" />

            {/* 标题 */}
            <select
              onChange={(e) => {
                if (e.target.value) {
                  execCmd("formatBlock", e.target.value);
                  e.target.value = "";
                }
              }}
              className="rounded border border-border bg-secondary px-1.5 py-1 text-xs text-foreground outline-none"
              defaultValue=""
            >
              <option value="" disabled>样式</option>
              <option value="h1">标题 1</option>
              <option value="h2">标题 2</option>
              <option value="h3">标题 3</option>
              <option value="p">正文</option>
            </select>
            <span className="mx-1 h-5 w-px bg-border" />

            {/* 列表 */}
            <ToolbarButton onClick={() => execCmd("insertUnorderedList")} icon={List} label="无序列表" />
            <ToolbarButton onClick={() => execCmd("insertOrderedList")} icon={ListOrdered} label="有序列表" />
            <span className="mx-1 h-5 w-px bg-border" />

            {/* 对齐 */}
            <ToolbarButton onClick={() => execCmd("justifyLeft")} icon={AlignLeft} label="左对齐" />
            <ToolbarButton onClick={() => execCmd("justifyCenter")} icon={AlignCenter} label="居中" />
            <ToolbarButton onClick={() => execCmd("justifyRight")} icon={AlignRight} label="右对齐" />
            <span className="mx-1 h-5 w-px bg-border" />

            {/* 字号 */}
            <select
              onChange={(e) => { if (e.target.value) execCmd("fontSize", e.target.value); }}
              className="rounded border border-border bg-secondary px-1.5 py-1 text-xs text-foreground outline-none"
            >
              <option value="3">字号</option>
              <option value="1">极小</option>
              <option value="2">小</option>
              <option value="3">正常</option>
              <option value="4">大</option>
              <option value="5">很大</option>
              <option value="6">超大</option>
              <option value="7">极大</option>
            </select>
          </>
        )}

        {/* 文件名 */}
        <span className="ml-auto flex items-center gap-1.5 text-xs text-foreground-muted">
          <FileText className="h-3.5 w-3.5" />
          {fileName}
        </span>

        {onClose && (
          <button
            onClick={onClose}
            className="btn-ghost ml-2 inline-flex h-7 w-7 items-center justify-center rounded-md text-xs"
            title="关闭"
          >
            ✕
          </button>
        )}
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="border-b border-warning/20 bg-warning/5 px-4 py-2 text-xs text-warning">
          {error}
        </div>
      )}

      {/* 编辑器 / 预览区域 */}
      <div className="min-h-0 flex-1 overflow-auto p-6">
        <div className="mx-auto max-w-4xl">
          {mode === "view" ? (
            <div
              className="word-preview prose prose-sm max-w-none prose-headings:text-foreground prose-p:text-foreground prose-strong:text-foreground prose-table:text-foreground"
              dangerouslySetInnerHTML={{ __html: docContent }}
            />
          ) : (
            <div
              ref={editorRef}
              className="word-editor min-h-[400px] rounded-lg border border-border bg-white p-6 shadow-sm outline-none"
              contentEditable
              suppressContentEditableWarning
              dangerouslySetInnerHTML={{ __html: docContent }}
              onBlur={(e) => setDocContent(e.currentTarget.innerHTML)}
            />
          )}
        </div>
      </div>
    </div>
  );
}

// ============================================================================
// ToolbarButton 子组件
// ============================================================================
function ToolbarButton({ onClick, icon: Icon, label, disabled, active }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={label}
      className={`inline-flex h-7 w-7 items-center justify-center rounded-md text-xs transition-colors ${
        active
          ? "bg-primary/15 text-primary"
          : "text-foreground-muted hover:bg-secondary hover:text-foreground"
      } disabled:opacity-40 disabled:cursor-not-allowed`}
    >
      <Icon className="h-3.5 w-3.5" />
    </button>
  );
}
