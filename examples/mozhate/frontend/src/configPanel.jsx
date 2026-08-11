import React from 'react';
import {
    Type,
    LayoutGrid,
    Palette,
    Info,
    Sun,
    Moon,
    Grid3X3,
    Square
} from 'lucide-react';

const GridType = { TIAN: 'tian', MI: 'mi', EMPTY: 'empty' };

const COLOR_OPTIONS = [
    { name: '银灰', value: '#d1d5db' },
    { name: '浅红', value: '#fca5a5' },
    { name: '朱红', value: '#ef4444' },
    { name: '薄荷', value: '#86efac' },
    { name: '岩灰', value: '#64748b' },
];

const GRID_COLORS = [
    { name: '浅色', value: '#e2e8f0' },
    { name: '标准', value: '#94a3b8' },
    { name: '纯黑', value: '#000000' },
    { name: '主题蓝', value: '#6366f1' },
];

function ConfigPanel({ config, setConfig, title, setTitle, inputText, setInputText }) {
    const updateConfig = (key, value) => {
        setConfig({ ...config, [key]: value });
    };

    return (
        <div className="config-panel">
            {/* Title */}
            <Section icon={<Type size={14} />} label="页面主标题">
                <input
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="输入字帖标题..."
                    className="input"
                />
            </Section>

            {/* Input Text */}
            <div className="section">
                <div className="section-header">
                    <div className="section-label">
                        <LayoutGrid size={14} />
                        <span>练习内容</span>
                    </div>
                    <span className="tip-badge">
                        <Info size={10} /> 加 '*' 开启描红
                    </span>
                </div>
                <textarea
                    rows={5}
                    value={inputText}
                    onChange={(e) => setInputText(e.target.value)}
                    placeholder="例如：勤*学* 苦*练*"
                    className="textarea"
                />
                <p className="hint">词语间请用空格分隔。</p>
            </div>

            {/* Grid Type */}
            <Section icon={<Grid3X3 size={14} />} label="网格排版配置">
                <div className="btn-row">
                    {Object.entries({ tian: '田字格', mi: '米字格', empty: '空白格' }).map(([key, label]) => (
                        <button
                            key={key}
                            onClick={() => updateConfig('gridType', key)}
                            className={`btn-sm ${config.gridType === key ? 'active' : ''}`}
                        >
                            {label}
                        </button>
                    ))}
                </div>
                <div className="btn-row" style={{ marginTop: 8 }}>
                    {['solid', 'dashed'].map((style) => (
                        <button
                            key={style}
                            onClick={() => updateConfig('lineStyle', style)}
                            className={`btn-sm ${config.lineStyle === style ? 'active-dark' : ''}`}
                        >
                            {style === 'solid' ? '实线网格' : '虚线网格'}
                        </button>
                    ))}
                </div>
            </Section>

            {/* Sliders */}
            <Section icon={<Square size={14} />} label="尺寸设置">
                <Slider
                    label="格子大小"
                    value={config.gridSize}
                    min={30}
                    max={120}
                    onChange={(v) => updateConfig('gridSize', v)}
                />
                <Slider
                    label="汉字字号"
                    value={config.charSize}
                    min={16}
                    max={100}
                    onChange={(v) => updateConfig('charSize', v)}
                />
                <Slider
                    label="拼音字号"
                    value={config.pinyinSize}
                    min={8}
                    max={32}
                    onChange={(v) => updateConfig('pinyinSize', v)}
                />
                <Slider
                    label="词间距"
                    value={config.wordGap}
                    min={0}
                    max={100}
                    onChange={(v) => updateConfig('wordGap', v)}
                />
                <Slider
                    label="行间距"
                    value={config.rowSpacing}
                    min={5}
                    max={80}
                    onChange={(v) => updateConfig('rowSpacing', v)}
                />
            </Section>

            {/* Colors */}
            <Section icon={<Palette size={14} />} label="描红文字颜色">
                <div className="color-row">
                    {COLOR_OPTIONS.map((col) => (
                        <button
                            key={col.value}
                            onClick={() => updateConfig('shadowColor', col.value)}
                            className={`color-btn ${config.shadowColor === col.value ? 'active' : ''}`}
                            style={{ backgroundColor: col.value }}
                            title={col.name}
                        />
                    ))}
                </div>
            </Section>

            <Section icon={<Palette size={14} />} label="格线颜色">
                <div className="color-row">
                    {GRID_COLORS.map((col) => (
                        <button
                            key={col.value}
                            onClick={() => updateConfig('gridColor', col.value)}
                            className={`color-btn ${config.gridColor === col.value ? 'active' : ''}`}
                            style={{ backgroundColor: col.value }}
                            title={col.name}
                        />
                    ))}
                </div>
            </Section>

            {/* Toggle */}
            <div className="section">
                <div className="toggle-row">
                    <span className="toggle-label">显示拼音</span>
                    <label className="toggle">
                        <input
                            type="checkbox"
                            checked={config.showPinyin}
                            onChange={(e) => updateConfig('showPinyin', e.target.checked)}
                        />
                        <span className="toggle-slider" />
                    </label>
                </div>
            </div>
        </div>
    );
}

// Helper components
function Section({ icon, label, children }) {
    return (
        <div className="section">
            <div className="section-label">
                {icon}
                <span>{label}</span>
            </div>
            {children}
        </div>
    );
}

function Slider({ label, value, min, max, onChange }) {
    return (
        <div className="slider-group">
            <div className="slider-header">
                <span className="slider-label">{label}</span>
                <span className="slider-value">{value}px</span>
            </div>
            <input
                type="range"
                min={min}
                max={max}
                value={value}
                onChange={(e) => onChange(parseInt(e.target.value))}
                className="range-input"
            />
        </div>
    );
}

export default ConfigPanel;
