import { useState, useEffect, useRef, useCallback } from 'react';
import { Play, Pause, Save, RotateCcw, FileText, Wand2, X, Loader2 } from 'lucide-react';

interface ScriptEditorProps {
  initialCode?: string;
  onSave?: (code: string) => void;
  onExecute?: (commands: string[]) => void;
  onReset?: () => void;
  onAiModify?: (instruction: string, selectedText: string) => void;
  className?: string;
}

export default function ScriptEditor({
  initialCode = '',
  onSave,
  onExecute,
  onReset,
  onAiModify,
  className = '',
}: ScriptEditorProps) {
  const [code, setCode] = useState(initialCode);
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentLine, setCurrentLine] = useState(-1);
  const [isDirty, setIsDirty] = useState(false);

  const [selectionRange, setSelectionRange] = useState<{ start: number; end: number } | null>(null);
  const [showAiInput, setShowAiInput] = useState(false);
  const [aiInstruction, setAiInstruction] = useState('');
  const [isAiProcessing, setIsAiProcessing] = useState(false);

  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const lineNumbersRef = useRef<HTMLDivElement>(null);
  const executionTimerRef = useRef<number | null>(null);
  const lastSavedCodeRef = useRef(initialCode);

  // Sync external code changes
  useEffect(() => {
    if (initialCode !== lastSavedCodeRef.current) {
      setCode(initialCode);
      lastSavedCodeRef.current = initialCode;
      setIsDirty(false);
    }
  }, [initialCode]);

  // Auto-save with debounce
  useEffect(() => {
    const timer = setTimeout(() => {
      if (isDirty && onSave) {
        onSave(code);
        lastSavedCodeRef.current = code;
        setIsDirty(false);
      }
    }, 1000);

    return () => clearTimeout(timer);
  }, [code, isDirty, onSave]);

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setCode(e.target.value);
    setIsDirty(true);
  };

  const handleSelect = (e: React.SyntheticEvent<HTMLTextAreaElement>) => {
    const target = e.currentTarget;
    if (target.selectionStart !== target.selectionEnd) {
      setSelectionRange({ start: target.selectionStart, end: target.selectionEnd });
    } else {
      setSelectionRange(null);
    }
  };

  const handleScroll = () => {
    if (textareaRef.current && lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = textareaRef.current.scrollTop;
    }
  };

  const stopExecution = useCallback(() => {
    if (executionTimerRef.current !== null) {
      clearTimeout(executionTimerRef.current);
      executionTimerRef.current = null;
    }
    setIsPlaying(false);
    setCurrentLine(-1);
  }, []);

  // Step-by-step execution
  const runScript = useCallback(() => {
    if (isPlaying) {
      stopExecution();
      return;
    }

    if (!onExecute || !onReset) return;

    setIsPlaying(true);
    onReset();
    
    const lines = code.split('\n').filter(line => line.trim() !== '');
    let index = 0;

    const executeNext = () => {
      if (index >= lines.length) {
        stopExecution();
        return;
      }

      setCurrentLine(index);
      const cmd = lines[index];
      onExecute([cmd]);

      index++;
      executionTimerRef.current = window.setTimeout(executeNext, 800);
    };

    executeNext();
  }, [code, isPlaying, onExecute, onReset, stopExecution]);

  // Batch execution
  const runBatch = useCallback(() => {
    if (isPlaying || !onExecute || !onReset) return;

    onReset();
    
    const lines = code.split('\n').filter(line => line.trim() !== '');
    if (lines.length > 0) {
      onExecute(lines);
    }
  }, [code, isPlaying, onExecute, onReset]);

  // Clear all
  const clearAll = useCallback(() => {
    stopExecution();
    onReset?.();
    setCode('');
    setIsDirty(false);
    lastSavedCodeRef.current = '';
    setCurrentLine(-1);
  }, [onReset, stopExecution]);

  // AI modify
  const handleAiModify = async () => {
    if (!selectionRange || !onAiModify || !aiInstruction.trim()) return;

    setIsAiProcessing(true);
    try {
      const selectedText = code.substring(selectionRange.start, selectionRange.end);
      await onAiModify(aiInstruction, selectedText);
      setShowAiInput(false);
      setAiInstruction('');
      setSelectionRange(null);
    } catch (error) {
      console.error('AI modify failed:', error);
    } finally {
      setIsAiProcessing(false);
    }
  };

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (executionTimerRef.current !== null) {
        clearTimeout(executionTimerRef.current);
      }
    };
  }, []);

  const lineCount = code.split('\n').length;

  return (
    <div className={`flex flex-col h-full bg-white border-l border-gray-200 ${className}`}>
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-2 bg-gray-50 border-b border-gray-200 shrink-0">
        <div className="flex items-center gap-2">
          <FileText className="w-4 h-4 text-gray-500" />
          <span className="text-sm font-medium text-gray-700">GGB 脚本</span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={runScript}
            className={`
              flex items-center gap-1 px-2 py-1 rounded text-xs font-medium transition-colors
              ${isPlaying 
                ? 'bg-amber-100 text-amber-700 hover:bg-amber-200' 
                : 'bg-green-100 text-green-700 hover:bg-green-200'
              }
            `}
            title="逐行执行"
          >
            {isPlaying ? (
              <>
                <Pause className="w-3 h-3" />
                暂停
              </>
            ) : (
              <>
                <Play className="w-3 h-3" />
                逐行
              </>
            )}
          </button>
          
          <button
            onClick={runBatch}
            disabled={isPlaying}
            className={`
              flex items-center gap-1 px-2 py-1 rounded text-xs font-medium transition-colors
              ${isPlaying
                ? 'bg-gray-100 text-gray-400 cursor-not-allowed'
                : 'bg-blue-100 text-blue-700 hover:bg-blue-200'
              }
            `}
            title="批量执行"
          >
            <Play className="w-3 h-3" />
            批量
          </button>
          
          <button
            onClick={clearAll}
            className="p-1 text-gray-500 hover:text-gray-700 hover:bg-gray-200 rounded transition-colors"
            title="清除"
          >
            <RotateCcw className="w-3 h-3" />
          </button>
        </div>
      </div>

      {/* Editor Area */}
      <div className="flex-1 relative overflow-hidden">
        <div className="absolute inset-0 flex">
          {/* Line Numbers */}
          <div 
            ref={lineNumbersRef}
            className="w-10 bg-gray-50 border-r border-gray-200 pt-2 text-right pr-2 text-gray-400 font-mono text-xs select-none overflow-hidden leading-5"
          >
            {Array.from({ length: lineCount }, (_, i) => (
              <div 
                key={i} 
                className={`
                  ${currentLine === i ? 'text-blue-600 font-bold bg-blue-50' : ''}
                `}
              >
                {i + 1}
              </div>
            ))}
          </div>
          
          {/* Text Area */}
          <textarea
            ref={textareaRef}
            value={code}
            onChange={handleChange}
            onSelect={handleSelect}
            onScroll={handleScroll}
            className="flex-1 p-2 font-mono text-xs bg-white border-none focus:ring-0 resize-none outline-none text-gray-800 leading-5 whitespace-pre overflow-auto"
            spellCheck={false}
            placeholder="输入 GeoGebra 指令，每行一条..."
          />
        </div>

        {/* Floating AI Button */}
        {selectionRange && !showAiInput && onAiModify && (
          <button
            onClick={() => setShowAiInput(true)}
            className="absolute top-4 right-8 bg-white text-purple-600 border border-purple-200 shadow-md px-3 py-1.5 rounded-full text-xs font-medium flex items-center gap-1.5 hover:bg-purple-50 transition-all z-10"
          >
            <Wand2 className="w-3 h-3" />
            AI 修改
          </button>
        )}

        {/* AI Input Popover */}
        {showAiInput && (
          <div className="absolute top-[60px] left-4 right-4 z-20 bg-white border border-blue-200 shadow-lg rounded-lg p-3">
            <div className="flex items-center gap-2 mb-2">
              <Wand2 className="w-4 h-4 text-purple-600" />
              <span className="text-sm font-medium text-gray-700">AI 修改选中指令</span>
              <button 
                onClick={() => setShowAiInput(false)}
                className="ml-auto text-gray-400 hover:text-gray-600"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="flex gap-2">
              <input
                type="text"
                value={aiInstruction}
                onChange={(e) => setAiInstruction(e.target.value)}
                placeholder="例如：把这个三角形改成等边三角形..."
                className="flex-1 px-3 py-1.5 text-sm border border-gray-300 rounded-md focus:ring-2 focus:ring-purple-500 focus:border-transparent outline-none"
                onKeyDown={(e) => e.key === 'Enter' && handleAiModify()}
                autoFocus
              />
              <button
                onClick={handleAiModify}
                disabled={isAiProcessing || !aiInstruction.trim()}
                className="px-3 py-1.5 bg-purple-600 text-white text-sm font-medium rounded-md hover:bg-purple-700 disabled:opacity-50 flex items-center gap-1"
              >
                {isAiProcessing ? <Loader2 className="w-3 h-3 animate-spin" /> : '修改'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Status Bar */}
      <div className="px-3 py-1.5 bg-gray-50 border-t border-gray-200 text-xs text-gray-500 flex justify-between items-center shrink-0">
        <span>{lineCount} 行指令</span>
        <span className={`
          flex items-center gap-1
          ${isDirty ? 'text-amber-600' : 'text-green-600'}
        `}>
          <Save className="w-3 h-3" />
          {isDirty ? '编辑中...' : '已保存'}
        </span>
      </div>
    </div>
  );
}
