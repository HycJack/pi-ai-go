// WordEditor - 基于 @docx-editor.dev/react 的 Word 文档编辑器
import { useState, useRef, useCallback, useEffect, Component } from "react";
import {
  DocxEditorRoot,
  DocxEditorViewport,
  DocxEditorContent,
  DocxEditorLoading,
  DocxEditorToolbar,
  PageIndicator,
} from "@docx-editor.dev/react";
import {
  Loader2,
  FileText,
  Save,
  X,
  AlertTriangle,
} from "lucide-react";

// ============================================================================
// WordEditorErrorBoundary - 捕获 DocxEditor 内部运行时错误
// ============================================================================
class WordEditorErrorBoundary extends Component {
  constructor(props) {
    super(props);
    this.state = { error: null, errorInfo: null };
  }
  static getDerivedStateFromError(error) {
    return { error };
  }
  componentDidCatch(error, errorInfo) {
    console.error("[WordEditor ErrorBoundary]", error, errorInfo);
    this.setState({ errorInfo });
  }
  render() {
    if (this.state.error) {
      return (
        <div className="flex h-full flex-col items-center justify-center gap-4 bg-white p-8">
          <AlertTriangle className="h-10 w-10 text-red-400" />
          <div className="text-center">
            <h3 className="text-base font-semibold text-gray-800">Word 编辑器加载失败</h3>
            <p className="mt-1 text-sm text-gray-500">{this.state.error?.message || String(this.state.error)}</p>
          </div>
          <div className="flex gap-3">
            {this.props.onClose && (
              <button
                onClick={this.props.onClose}
                className="rounded-md border border-gray-200 px-4 py-2 text-sm text-gray-600 hover:bg-gray-50"
              >
                关闭
              </button>
            )}
            <button
              onClick={() => this.setState({ error: null, errorInfo: null })}
              className="rounded-md bg-indigo-600 px-4 py-2 text-sm text-white hover:bg-indigo-700"
            >
              重试
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

// ============================================================================
// WordEditor - 主组件
// ============================================================================
export function WordEditor({ file, docxBase64, fileName: propFileName, onClose }) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [docBytes, setDocBytes] = useState(null);
  const [fileName, setFileName] = useState(propFileName || "文档.docx");
  const editorRef = useRef(null);

  // 加载 docx 二进制数据
  useEffect(() => {
    const loadDocument = async () => {
      setLoading(true);
      setError("");
      try {
        let bytes;

        if (file instanceof File) {
          setFileName(file.name);
          bytes = new Uint8Array(await file.arrayBuffer());
        } else if (typeof docxBase64 === "string" && docxBase64.length > 0) {
          setFileName(propFileName || "文档.docx");
          const binary = atob(docxBase64);
          bytes = new Uint8Array(binary.length);
          for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
          }
        } else {
          // 没有文档，创建一个空白文档
          const { Document, Packer, Paragraph, TextRun } = await import("docx");
          const doc = new Document({
            title: "新文档",
            sections: [{ children: [new Paragraph({ children: [new TextRun("新文档")] })] }],
          });
          const blob = await Packer.toBlob(doc);
          bytes = new Uint8Array(await blob.arrayBuffer());
        }

        setDocBytes(bytes);
      } catch (e) {
        console.error("[WordEditor] load error:", e);
        setError(`文档加载失败: ${e.message || e}`);
      } finally {
        setLoading(false);
      }
    };

    loadDocument();
  }, [file, docxBase64, propFileName]);

  // 保存文档
  const handleSave = useCallback(async () => {
    try {
      if (!docBytes) return;
      const blob = new Blob([docBytes], { type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document" });
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
    }
  }, [docBytes, fileName]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex flex-col items-center gap-3 text-muted-foreground">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
          <span className="text-sm">正在加载 Word 文档...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col bg-white">
      {/* 顶栏 */}
      <div className="flex items-center justify-between border-b border-gray-200 bg-white px-4 py-2">
        <div className="flex items-center gap-2">
          <FileText className="h-4 w-4 text-indigo-600" />
          <span className="text-sm font-medium text-gray-700">{fileName}</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleSave}
            className="inline-flex items-center gap-1.5 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 hover:border-gray-300"
          >
            <Save className="h-3.5 w-3.5" />
            保存
          </button>
          {onClose && (
            <button
              onClick={onClose}
              className="inline-flex h-7 w-7 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-600"
              title="关闭"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="border-b border-yellow-200 bg-yellow-50 px-4 py-2 text-xs text-yellow-700">
          {error}
        </div>
      )}

      {/* DocxEditor */}
      <WordEditorErrorBoundary onClose={onClose}>
      <div className="min-h-0 flex-1 overflow-hidden">
        <DocxEditorRoot
          document={docBytes}
          mode="edit"
          zoom={{ value: 100, type: "fixed"}}
          onReady={(editor) => { editorRef.current = editor; }}
        >
          <DocxEditorToolbar
            preset
            onSave={handleSave}
            className="border-b border-gray-200 bg-white shadow-sm"
          />
          <DocxEditorLoading>
            <div className="flex h-full items-center justify-center">
              <div className="flex flex-col items-center gap-3 text-gray-400">
                <Loader2 className="h-8 w-8 animate-spin text-indigo-500" />
                <span className="text-sm">渲染文档...</span>
              </div>
            </div>
          </DocxEditorLoading>
          <div className="flex min-h-0 flex-1 flex-col">
            <DocxEditorViewport className="flex-1 overflow-auto bg-[#f3f4f6]">
              <DocxEditorContent />
            </DocxEditorViewport>
            <div className="flex items-center justify-between border-t border-gray-200 bg-white px-4 py-1.5">
              <PageIndicator className="text-xs text-gray-400" />
            </div>
          </div>
        </DocxEditorRoot>
      </div>
      </WordEditorErrorBoundary>
    </div>
  );
}


