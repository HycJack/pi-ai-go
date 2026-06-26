// 导出器（service worker 上下文）
// 通过 importScripts 加载到 background.js
// 全局挂在 self.DS_EXPORTERS

(function (global) {
  'use strict';

  function escapeHtml(s) {
    return String(s == null ? '' : s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function formatDate(ts) {
    if (!ts) return '';
    try {
      return new Date(ts).toLocaleString('zh-CN', { hour12: false });
    } catch {
      return String(ts);
    }
  }

  // === JSON ===
  function toJson(session, messages) {
    return JSON.stringify({
      schemaVersion: 'deepseek-enhanced.export.v1',
      exportedAt: new Date().toISOString(),
      session,
      messages
    }, null, 2);
  }

  // === Markdown ===
  function toMarkdown(session, messages) {
    const lines = [];
    lines.push('# ' + (session.title || '(无标题)'));
    lines.push('');
    lines.push('- 会话 ID：`' + session.id + '`');
    if (session.modelType) lines.push('- 模型：`' + session.modelType + '`');
    lines.push('- 创建时间：' + formatDate(session.createdAt));
    lines.push('- 更新时间：' + formatDate(session.updatedAt));
    lines.push('- 消息数：' + (session.messageCount || messages.length));
    if (session.toolCallCount) lines.push('- 工具调用次数：' + session.toolCallCount);
    lines.push('');
    lines.push('---');
    lines.push('');

    for (const m of messages) {
      const ts = formatDate(m.createdAt);
      if (m.role === 'user') {
        lines.push('## 👤 用户' + (ts ? '  `' + ts + '`' : ''));
        lines.push('');
        lines.push(m.content || '');
        lines.push('');
      } else if (m.role === 'assistant') {
        lines.push('## 🤖 助手' + (ts ? '  `' + ts + '`' : '') + (m.final ? '  ✓ 最终' : ''));
        lines.push('');
        lines.push(m.content || '');
        lines.push('');
      } else if (m.role === 'tool_call') {
        lines.push('### 🛠 工具调用  `' + (m.toolName || '') + '`');
        lines.push('');
        lines.push('```json');
        lines.push(m.content || '');
        lines.push('```');
        lines.push('');
      } else if (m.role === 'tool_result') {
        const tag = m.toolOk === false ? '❌' : '✅';
        lines.push('### ' + tag + ' 工具结果  `' + (m.toolName || '') + '`  ' + (m.toolElapsedMs || 0) + 'ms');
        if (m.toolError) {
          lines.push('');
          lines.push('**错误：** ' + m.toolError);
        }
        lines.push('');
        lines.push('```');
        lines.push(m.content || '');
        lines.push('```');
        lines.push('');
      }
    }
    return lines.join('\n');
  }

  // === HTML ===
  function toHtml(session, messages) {
    const blocks = [];
    for (const m of messages) {
      const ts = formatDate(m.createdAt);
      if (m.role === 'user') {
        blocks.push(
          '<div class="msg msg-user">' +
            '<div class="msg-head">👤 用户 <span class="ts">' + escapeHtml(ts) + '</span></div>' +
            '<div class="msg-body">' + escapeHtml(m.content || '').replace(/\n/g, '<br>') + '</div>' +
          '</div>'
        );
      } else if (m.role === 'assistant') {
        blocks.push(
          '<div class="msg msg-assistant">' +
            '<div class="msg-head">🤖 助手 ' + (m.final ? '<span class="final">✓ 最终</span> ' : '') + '<span class="ts">' + escapeHtml(ts) + '</span></div>' +
            '<div class="msg-body">' + escapeHtml(m.content || '').replace(/\n/g, '<br>') + '</div>' +
          '</div>'
        );
      } else if (m.role === 'tool_call') {
        blocks.push(
          '<div class="msg msg-tool-call">' +
            '<div class="msg-head">🛠 工具调用 <code>' + escapeHtml(m.toolName || '') + '</code></div>' +
            '<pre>' + escapeHtml(m.content || '') + '</pre>' +
          '</div>'
        );
      } else if (m.role === 'tool_result') {
        const tag = m.toolOk === false ? '❌' : '✅';
        blocks.push(
          '<div class="msg msg-tool-result ' + (m.toolOk === false ? 'err' : '') + '">' +
            '<div class="msg-head">' + tag + ' 工具结果 <code>' + escapeHtml(m.toolName || '') + '</code> <span class="ts">' + escapeHtml((m.toolElapsedMs || 0) + 'ms') + '</span></div>' +
            (m.toolError ? '<div class="err">错误：' + escapeHtml(m.toolError) + '</div>' : '') +
            '<pre>' + escapeHtml(m.content || '') + '</pre>' +
          '</div>'
        );
      }
    }
    return [
      '<!DOCTYPE html>',
      '<html><head><meta charset="UTF-8"><title>' + escapeHtml(session.title || '会话') + '</title>',
      '<style>',
      'body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;max-width:860px;margin:24px auto;padding:0 16px;color:#1f2937;background:#f9fafb}',
      'h1{font-size:20px;margin-bottom:8px}',
      '.meta{color:#6b7280;font-size:13px;margin-bottom:24px}',
      '.msg{border-radius:8px;padding:12px 14px;margin-bottom:10px;background:white;border:1px solid #e5e7eb}',
      '.msg-user{background:#eff6ff;border-color:#bfdbfe}',
      '.msg-assistant{background:#f0fdf4;border-color:#bbf7d0}',
      '.msg-tool-call{background:#fffbeb;border-color:#fde68a;font-size:13px}',
      '.msg-tool-result{background:#f5f3ff;border-color:#ddd6fe;font-size:13px}',
      '.msg-tool-result.err{background:#fef2f2;border-color:#fecaca}',
      '.msg-head{font-weight:600;font-size:13px;margin-bottom:6px;color:#374151}',
      '.msg-head .ts{font-weight:400;color:#9ca3af;margin-left:6px}',
      '.msg-head .final{color:#059669;font-size:11px}',
      '.msg-body{line-height:1.6;white-space:pre-wrap;word-break:break-word}',
      'pre{background:#1f2937;color:#f9fafb;padding:8px 10px;border-radius:6px;overflow-x:auto;font-size:12px;margin:6px 0 0 0;white-space:pre-wrap}',
      '.err{color:#b91c1c;font-size:12px;margin-top:4px}',
      '</style></head><body>',
      '<h1>' + escapeHtml(session.title || '(无标题)') + '</h1>',
      '<div class="meta">',
        '会话 ID：<code>' + escapeHtml(session.id) + '</code><br>',
        (session.modelType ? '模型：<code>' + escapeHtml(session.modelType) + '</code><br>' : ''),
        '创建时间：' + escapeHtml(formatDate(session.createdAt)) + '<br>',
        '更新时间：' + escapeHtml(formatDate(session.updatedAt)) + '<br>',
        '消息数：' + (session.messageCount || messages.length),
        (session.toolCallCount ? '，工具调用：' + session.toolCallCount : ''),
      '</div>',
      blocks.join('\n'),
      '</body></html>'
    ].join('\n');
  }

  global.DS_EXPORTERS = {
    toJson,
    toMarkdown,
    toHtml,
    formats: ['json', 'markdown', 'html'],
    ext: { json: 'json', markdown: 'md', html: 'html' }
  };
})(typeof self !== 'undefined' ? self : this);