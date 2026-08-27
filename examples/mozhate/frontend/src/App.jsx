import React, { useState, useMemo, useEffect } from 'react';
import {
    Download,
    Settings2,
    Sparkles,
    History,
    Trash2,
    Printer,
    ChevronRight,
    ChevronLeft,
    BookOpen
} from 'lucide-react';
import { getPinyin, generateContent, exitApp } from './wailsApi';
import ConfigPanel from './configPanel';
import PreviewPanel from './previewPanel';
import AIConfigPanel from './aiConfigPanel';

const GridType = { TIAN: 'tian', MI: 'mi', EMPTY: 'empty' };

const DEFAULT_CONFIG = {
    gridType: GridType.TIAN,
    lineStyle: 'dashed',
    pinyinSize: 14,
    charSize: 42,
    gridSize: 60,
    gridSpacing: 1,
    wordGap: 15,
    rowSpacing: 15,
    shadowColor: '#d1d5db',
    gridColor: '#94a3b8',
    showPinyin: true,
};

const PX_TO_MM = 0.264583;
const A4_WIDTH_MM = 210;
const A4_HEIGHT_MM = 297;
const PAGE_PADDING_MM = 15;
const HEADER_HEIGHT_MM = 25;

function App() {
    const [title, setTitle] = useState('勤学苦练 天天向上');
    const [inputText, setInputText] = useState('勤*学* 苦*练* 积*极* 向*上* 自*强* 不*息*');
    const [config, setConfig] = useState(DEFAULT_CONFIG);
    const [isGenerating, setIsGenerating] = useState(false);
    const [sidebarOpen, setSidebarOpen] = useState(true);
    const [wordGroups, setWordGroups] = useState([]);
    const [error, setError] = useState('');
    const [aiConfigOpen, setAiConfigOpen] = useState(false);

    // Process input text to character items
    useEffect(() => {
        const processInput = async () => {
            const groups = inputText.trim().split(/\s+/).filter(Boolean);
            if (groups.length === 0) {
                setWordGroups([]);
                return;
            }

            const processedGroups = [];
            for (const group of groups) {
                const pureText = group.replace(/\*/g, '');
                let pinyins = [];

                try {
                    pinyins = await getPinyin(pureText);
                } catch (e) {
                    console.error('Pinyin error:', e);
                    pinyins = [];
                }

                const chars = [];
                let pinyinIdx = 0;
                for (let i = 0; i < group.length; i++) {
                    if (group[i] === '*') continue;
                    const char = group[i];
                    const showChar = (i + 1 < group.length && group[i + 1] === '*');
                    chars.push({
                        id: `char-${Math.random().toString(36).substr(2, 9)}`,
                        char,
                        showChar,
                        pinyin: pinyins[pinyinIdx] || '',
                    });
                    pinyinIdx++;
                }
                if (chars.length > 0) {
                    processedGroups.push(chars);
                }
            }
            setWordGroups(processedGroups);
        };

        const timeout = setTimeout(processInput, 200);
        return () => clearTimeout(timeout);
    }, [inputText]);

    // Pagination logic
    const pagedData = useMemo(() => {
        const usableWidth = A4_WIDTH_MM - PAGE_PADDING_MM * 2;
        const usableHeight = A4_HEIGHT_MM - PAGE_PADDING_MM * 2;

        const charWidthMm = (config.gridSize + config.gridSpacing) * PX_TO_MM;
        const wordGapMm = config.wordGap * PX_TO_MM;
        const rowHeightMm = (config.pinyinSize + config.gridSize + config.rowSpacing + 10) * PX_TO_MM;

        const rows = [];
        let currentRow = [];
        let currentRowWidthMm = 0;

        for (const group of wordGroups) {
            const groupWidthMm = group.length * charWidthMm;
            if (currentRow.length > 0 && currentRowWidthMm + wordGapMm + groupWidthMm > usableWidth) {
                rows.push(currentRow);
                currentRow = [group];
                currentRowWidthMm = groupWidthMm;
            } else {
                const gap = currentRow.length > 0 ? wordGapMm : 0;
                currentRow.push(group);
                currentRowWidthMm += gap + groupWidthMm;
            }
        }
        if (currentRow.length > 0) rows.push(currentRow);

        const pages = [];
        let currentPageRows = [];
        let currentHeight = HEADER_HEIGHT_MM;

        for (const row of rows) {
            if (currentHeight + rowHeightMm > usableHeight && currentPageRows.length > 0) {
                pages.push({ rows: currentPageRows });
                currentPageRows = [row];
                currentHeight = rowHeightMm;
            } else {
                currentPageRows.push(row);
                currentHeight += rowHeightMm;
            }
        }
        if (currentPageRows.length > 0) pages.push({ rows: currentPageRows });

        return pages;
    }, [wordGroups, config]);

    const [aiPrompt, setAiPrompt] = useState('');

    const handleGenerate = async () => {
        const prompt = aiPrompt || '请生成古诗词相关字帖';
        setIsGenerating(true);
        setError('');
        try {
            const phrases = await generateContent(prompt);
            if (phrases && phrases.length > 0) {
                setInputText(phrases.join(' '));
            } else {
                setError('AI 未返回结果，请检查 API 密钥');
            }
        } catch (e) {
            setError('AI 请求失败：' + e.message);
        } finally {
            setIsGenerating(false);
        }
    };

    return (
        <div className="app-container">
            {/* AI Config Modal */}
            <AIConfigPanel
                visible={aiConfigOpen}
                onClose={() => setAiConfigOpen(false)}
            />

            {/* Sidebar */}
            <aside className={`sidebar ${sidebarOpen ? 'open' : ''}`}>
                <div className="sidebar-header">
                    <div className="sidebar-title">
                        <div className="sidebar-icon">
                            <BookOpen size={22} />
                        </div>
                        <h1>沫渣特<br />默写字帖生成器</h1>
                    </div>
                    <button onClick={() => setSidebarOpen(false)} className="sidebar-toggle">
                        <ChevronLeft size={20} />
                    </button>
                </div>

                <div className="sidebar-content">
                    <ConfigPanel
                        config={config}
                        setConfig={setConfig}
                        title={title}
                        setTitle={setTitle}
                        inputText={inputText}
                        setInputText={setInputText}
                    />
                </div>

                <div className="sidebar-footer">
                    <div className="ai-input-group">
                        <input
                            type="text"
                            value={aiPrompt}
                            onChange={(e) => setAiPrompt(e.target.value)}
                            placeholder="输入主题：大自然、古诗词..."
                            className="ai-input"
                        />
                        <div className="ai-action-row">
                            <button onClick={handleGenerate} disabled={isGenerating} className="btn btn-primary btn-ai flex-1">
                                <Sparkles size={18} />
                                {isGenerating ? '生成中...' : 'AI 生成'}
                            </button>
                            <button
                                onClick={() => setAiConfigOpen(true)}
                                className="btn btn-icon"
                                title="AI 设置"
                            >
                                <Settings2 size={18} />
                            </button>
                        </div>
                    </div>
                    {error && <p className="error-text">{error}</p>}
                </div>
            </aside>

            {/* Main */}
            <main className="main-content">
                {!sidebarOpen && (
                    <button onClick={() => setSidebarOpen(true)} className="open-sidebar-btn">
                        <ChevronRight size={24} />
                    </button>
                )}

                <header className="main-header">
                    <div className="header-left">
                        <History size={16} />
                        <span>已自动保存</span>
                    </div>
                    <div className="header-right">
                        <button
                            onClick={() => setAiConfigOpen(true)}
                            className="header-settings-btn"
                            title="AI 设置"
                        >
                            <Settings2 size={16} />
                            <span>AI 配置</span>
                        </button>
                        <span className="status-badge">
                            <Sparkles size={14} />
                            AI 增强
                        </span>
                        <div className="divider" />
                        <span className="page-label">A4 页面预览</span>
                    </div>
                </header>

                <div className="preview-wrapper">
                    <div className="preview-scroll">
                        <PreviewPanel pages={pagedData} config={config} title={title} />
                    </div>
                </div>
            </main>
        </div>
    );
}

export default App;
