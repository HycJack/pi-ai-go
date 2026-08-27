import React, { useRef, useEffect } from 'react';
import { Download, Printer } from 'lucide-react';

const GridType = { TIAN: 'tian', MI: 'mi', EMPTY: 'empty' };

function PreviewPanel({ pages, config, title }) {
    const printRef = useRef(null);

    const generateGridLines = (config) => {
        const { gridType, lineStyle, gridColor } = config;

        if (gridType === GridType.EMPTY) return null;

        const dash = lineStyle === 'dashed' ? '4 4' : 'none';

        return (
            <>
                {/* Horizontal center line */}
                <div
                    className="grid-line-h"
                    style={{
                        borderTop: `1px ${lineStyle} ${gridColor}`,
                    }}
                />
                {/* Vertical center line */}
                <div
                    className="grid-line-v"
                    style={{
                        borderLeft: `1px ${lineStyle} ${gridColor}`,
                    }}
                />
                {/* Mi grid diagonals */}
                {gridType === GridType.MI && (
                    <svg className="grid-diagonal" viewBox="0 0 100 100">
                        <line
                            x1="0" y1="0" x2="100" y2="100"
                            stroke={gridColor}
                            strokeWidth="1"
                            strokeDasharray={dash}
                        />
                        <line
                            x1="100" y1="0" x2="0" y2="100"
                            stroke={gridColor}
                            strokeWidth="1"
                            strokeDasharray={dash}
                        />
                    </svg>
                )}
            </>
        );
    };

    const handlePrint = () => {
        window.print();
    };

    const handleExportPDF = () => {
        // Use print to PDF as a simple export method
        // In Wails, the built-in print dialog allows "Save as PDF"
        window.print();
    };

    if (!pages || pages.length === 0) {
        return (
            <div className="empty-state">
                <p>请在上方输入要练习的内容</p>
            </div>
        );
    }

    return (
        <div className="preview-container">
            {/* Action buttons */}
            <div className="preview-actions">
                <button onClick={handleExportPDF} className="btn btn-primary">
                    <Download size={18} />
                    导出 PDF
                </button>
                <button onClick={handlePrint} className="btn btn-secondary">
                    <Printer size={18} />
                    打印
                </button>
            </div>

            {/* Pages */}
            <div ref={printRef} className="pages">
                {pages.map((page, idx) => (
                    <div key={idx} className="a4-page">
                        {/* Header (first page only) */}
                        {idx === 0 && (
                            <div className="page-header">
                                <div>
                                    <h1 className="page-title">{title || '默写字帖'}</h1>
                                    <p className="page-subtitle">沫渣特系列 • 汉字默写练习字帖</p>
                                </div>
                                <div className="page-info">
                                    <span>姓名: __________</span>
                                    <span>得分: ______</span>
                                </div>
                            </div>
                        )}

                        {/* Row container */}
                        <div
                            className="rows-container"
                            style={{ gap: `${config.rowSpacing}px` }}
                        >
                            {page.rows.map((row, rIdx) => (
                                <div
                                    key={rIdx}
                                    className="row"
                                    style={{ gap: `${config.wordGap}px` }}
                                >
                                    {row.map((group, gIdx) => (
                                        <div
                                            key={gIdx}
                                            className="group"
                                            style={{ gap: `${config.gridSpacing}px` }}
                                        >
                                            {group.map((item) => (
                                                <div key={item.id} className="char-cell-wrapper">
                                                    {/* Pinyin */}
                                                    <div
                                                        className={`pinyin-text ${config.showPinyin ? '' : 'hidden'}`}
                                                        style={{
                                                            fontSize: `${config.pinyinSize}px`,
                                                            minWidth: `${config.gridSize}px`,
                                                        }}
                                                    >
                                                        {item.pinyin}
                                                    </div>

                                                    {/* Grid cell */}
                                                    <div
                                                        className="grid-cell"
                                                        style={{
                                                            width: `${config.gridSize}px`,
                                                            height: `${config.gridSize}px`,
                                                            border: `1.5px solid ${config.gridColor}`,
                                                        }}
                                                    >
                                                        {generateGridLines(config)}

                                                        {/* Character */}
                                                        {item.showChar && (
                                                            <div
                                                                className="char-overlay"
                                                                style={{
                                                                    fontSize: `${config.charSize}px`,
                                                                    color: config.shadowColor,
                                                                }}
                                                            >
                                                                {item.char}
                                                            </div>
                                                        )}
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    ))}
                                </div>
                            ))}
                        </div>

                        {/* Page footer */}
                        <div className="page-footer">
                            <span className="page-num">
                                沫渣特 • 第 {idx + 1} 页，共 {pages.length} 页
                            </span>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}

export default PreviewPanel;
