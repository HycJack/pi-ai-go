(function () {
  const STORAGE_KEY = 'deepseek_enhanced_settings';
  const DIR_HANDLE_KEY = 'ds_working_dir';

  const DEFAULT_PROMPT = `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  // === DOM ===
  const promptEl = document.getElementById('prompt');
  const saveBtn = document.getElementById('save');
  const resetBtn = document.getElementById('reset');
  const importBtn = document.getElementById('import-prompt');
  const exportBtn = document.getElementById('export-prompt');
  const pickDirBtn = document.getElementById('pick-dir');
  const clearDirBtn = document.getElementById('clear-dir');
  const dirInfoEl = document.getElementById('dir-info');
  const saveToDirBtn = document.getElementById('save-to-dir');
  const readFromDirBtn = document.getElementById('read-from-dir');
  const listDirBtn = document.getElementById('list-dir');
  const writeLogBtn = document.getElementById('write-log');
  const fileListEl = document.getElementById('file-list');
  const refreshHistoryBtn = document.getElementById('refresh-history');
  const clearHistoryBtn = document.getElementById('clear-history');
  const sessionListEl = document.getElementById('session-list');
  const sessionMaskEl = document.getElementById('session-mask');
  const dialogTitleEl = document.getElementById('dialog-title');
  const dialogBodyEl = document.getElementById('dialog-body');
  const dialogCloseBtn = document.getElementById('dialog-close');
  const exportMdBtn = document.getElementById('export-md');
  const exportHtmlBtn = document.getElementById('export-html');
  const exportJsonBtn = document.getElementById('export-json');
  const deleteSessionBtn = document.getElementById('delete-session');
  const mcpNameInput = document.getElementById('mcp-name');
  const mcpUrlInput = document.getElementById('mcp-url');
  const mcpTypeSelect = document.getElementById('mcp-type');
  const mcpAddBtn = document.getElementById('mcp-add');
  const mcpRefreshAllBtn = document.getElementById('mcp-refresh-all');
  const mcpListEl = document.getElementById('mcp-list');
  const statusEl = document.getElementById('status');

  let currentSession = null; // { id, title, ... }

  let workingDir = null; // FileSystemDirectoryHandle

  // === 工具函数 ===
  function setStatus(msg, kind = '', ms = 2500) {
    statusEl.textContent = msg;
    statusEl.className = 'status' + (kind ? ' ' + kind : '');
    if (ms) {
      setTimeout(() => {
        if (statusEl.textContent === msg) statusEl.textContent = '';
        statusEl.className = 'status';
      }, ms);
    }
  }

  function checkFsApi() {
    if (!('showOpenFilePicker' in window) || !('showSaveFilePicker' in window) || !('showDirectoryPicker' in window)) {
      setStatus('当前浏览器不支持 File System Access API', 'error', 0);
      return false;
    }
    return true;
  }

  function setWorkingDir(handle, name) {
    workingDir = handle;
    if (handle) {
      dirInfoEl.textContent = '📁 ' + (name || '已选择工作目录');
      dirInfoEl.classList.add('active');
      clearDirBtn.disabled = false;
      saveToDirBtn.disabled = false;
      readFromDirBtn.disabled = false;
      listDirBtn.disabled = false;
      writeLogBtn.disabled = false;
    } else {
      dirInfoEl.textContent = '未选择工作目录';
      dirInfoEl.classList.remove('active');
      clearDirBtn.disabled = true;
      saveToDirBtn.disabled = true;
      readFromDirBtn.disabled = true;
      listDirBtn.disabled = true;
      writeLogBtn.disabled = true;
      fileListEl.style.display = 'none';
      fileListEl.textContent = '';
    }
  }

  // 持久化的目录 handle 需要再次请求权限（浏览器重启后权限会失效）
  async function ensureDirPermission(handle) {
    const opts = { mode: 'readwrite' };
    if ((await handle.queryPermission(opts)) === 'granted') return true;
    return (await handle.requestPermission(opts)) === 'granted';
  }

  // === 已保存设置 ===
  chrome.storage.local.get(STORAGE_KEY, (result) => {
    const settings = result[STORAGE_KEY] || {};
    promptEl.value = settings.prompt || DEFAULT_PROMPT;
  });

  // 恢复工作目录（如有）
  (async () => {
    const r = await chrome.storage.local.get(DIR_HANDLE_KEY);
    const handle = r[DIR_HANDLE_KEY];
    if (!handle) return;
    try {
      if (await ensureDirPermission(handle)) {
        setWorkingDir(handle, handle.name);
        setStatus('已恢复工作目录: ' + handle.name, 'success');
      } else {
        setStatus('工作目录权限被拒绝，请重新选择', 'error');
      }
    } catch (e) {
      console.warn('[DS] restore dir failed:', e);
    }
  })();

  // === 保存/重置 ===
  saveBtn.addEventListener('click', () => {
    const prompt = promptEl.value.trim() || DEFAULT_PROMPT;
    chrome.storage.local.set({ [STORAGE_KEY]: { prompt, enabled: true } }, () => {
      setStatus('✅ 已保存', 'success');
    });
  });

  resetBtn.addEventListener('click', () => {
    chrome.storage.local.set({ [STORAGE_KEY]: { prompt: DEFAULT_PROMPT, enabled: true } }, () => {
      promptEl.value = DEFAULT_PROMPT;
      setStatus('已恢复默认');
    });
  });

  // === 文件读写 ===
  // 导入 prompt
  importBtn.addEventListener('click', async () => {
    if (!checkFsApi()) return;
    try {
      const [handle] = await window.showOpenFilePicker({
        multiple: false,
        types: [{
          description: 'Prompt 文件',
          accept: {
            'text/plain': ['.txt', '.md'],
            'application/json': ['.json']
          }
        }]
      });
      const file = await handle.getFile();
      const text = await file.text();
      promptEl.value = text;
      setStatus('✅ 已从 ' + handle.name + ' 导入', 'success');
    } catch (e) {
      if (e.name !== 'AbortError') setStatus('导入失败: ' + e.message, 'error');
    }
  });

  // 导出 prompt
  exportBtn.addEventListener('click', async () => {
    if (!checkFsApi()) return;
    try {
      const handle = await window.showSaveFilePicker({
        suggestedName: 'system-prompt.txt',
        types: [{ description: 'Text', accept: { 'text/plain': ['.txt'] } }]
      });
      const writable = await handle.createWritable();
      await writable.write(promptEl.value);
      await writable.close();
      setStatus('✅ 已导出到 ' + handle.name, 'success');
    } catch (e) {
      if (e.name !== 'AbortError') setStatus('导出失败: ' + e.message, 'error');
    }
  });

  // 选择工作目录
  pickDirBtn.addEventListener('click', async () => {
    if (!checkFsApi()) return;
    try {
      const handle = await window.showDirectoryPicker({ mode: 'readwrite' });
      if (!(await ensureDirPermission(handle))) {
        setStatus('目录权限被拒绝', 'error');
        return;
      }
      await chrome.storage.local.set({ [DIR_HANDLE_KEY]: handle });
      setWorkingDir(handle, handle.name);
      setStatus('✅ 工作目录: ' + handle.name, 'success');
    } catch (e) {
      if (e.name !== 'AbortError') setStatus('选择目录失败: ' + e.message, 'error');
    }
  });

  // 清除工作目录
  clearDirBtn.addEventListener('click', async () => {
    await chrome.storage.local.remove(DIR_HANDLE_KEY);
    setWorkingDir(null);
    setStatus('已清除工作目录');
  });

  // 保存 prompt 到工作目录
  saveToDirBtn.addEventListener('click', async () => {
    if (!workingDir) return;
    try {
      const filename = 'system-prompt.txt';
      const fh = await workingDir.getFileHandle(filename, { create: true });
      const writable = await fh.createWritable();
      await writable.write(promptEl.value);
      await writable.close();
      setStatus('✅ 已写入 ' + filename, 'success');
    } catch (e) {
      setStatus('写入失败: ' + e.message, 'error');
    }
  });

  // 从工作目录读 prompt
  readFromDirBtn.addEventListener('click', async () => {
    if (!workingDir) return;
    const filename = prompt('请输入要读取的文件名（如 system-prompt.txt）:', 'system-prompt.txt');
    if (!filename) return;
    try {
      const fh = await workingDir.getFileHandle(filename);
      const file = await fh.getFile();
      const text = await file.text();
      promptEl.value = text;
      setStatus('✅ 已从 ' + filename + ' 读取', 'success');
    } catch (e) {
      setStatus('读取失败: ' + e.message, 'error');
    }
  });

  // 列出文件
  listDirBtn.addEventListener('click', async () => {
    if (!workingDir) return;
    try {
      const names = [];
      for await (const [name, h] of workingDir.entries()) {
        if (h.kind === 'file') names.push('📄 ' + name);
      }
      if (names.length === 0) {
        fileListEl.textContent = '(目录为空)';
      } else {
        fileListEl.textContent = names.sort().join('\n');
      }
      fileListEl.style.display = 'block';
      setStatus('共 ' + names.length + ' 个文件', 'success');
    } catch (e) {
      setStatus('列出文件失败: ' + e.message, 'error');
    }
  });

  // 追加日志（在工作目录下追加一行到 deepseek-enhanced.log）
  writeLogBtn.addEventListener('click', async () => {
    if (!workingDir) return;
    try {
      const filename = 'deepseek-enhanced.log';
      const fh = await workingDir.getFileHandle(filename, { create: true });
      const file = await fh.getFile();
      const prev = await file.text();
      const line = '[' + new Date().toISOString() + '] prompt updated (len=' + promptEl.value.length + ')';
      const writable = await fh.createWritable();
      await writable.write(prev + (prev && !prev.endsWith('\n') ? '\n' : '') + line + '\n');
      await writable.close();
      setStatus('✅ 已追加日志', 'success');
    } catch (e) {
      setStatus('写日志失败: ' + e.message, 'error');
    }
  });

  // ============ 历史会话 / MCP ============
  // 侧边栏是独立 JS context，直接调 chrome.runtime.sendMessage
  function callBridge(type, payload) {
    return new Promise((resolve, reject) => {
      const req = Object.assign({ type }, payload || {});
      const timer = setTimeout(() => {
        reject(new Error('Bridge timeout: ' + type));
      }, 15000);
      try {
        chrome.runtime.sendMessage(req, (response) => {
          clearTimeout(timer);
          if (chrome.runtime.lastError) {
            reject(new Error(chrome.runtime.lastError.message || 'sendMessage failed'));
            return;
          }
          if (!response) {
            reject(new Error('No response for ' + type));
            return;
          }
          if (response.error) {
            reject(new Error(response.error));
            return;
          }
          resolve(response);
        });
      } catch (e) {
        clearTimeout(timer);
        reject(e);
      }
    });
  }

  function fmtTime(ts) {
    if (!ts) return '';
    const d = new Date(ts);
    const now = new Date();
    const sameDay = d.toDateString() === now.toDateString();
    if (sameDay) return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
    return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
  }

  async function loadHistory() {
    sessionListEl.innerHTML = '<div class="session-empty">加载中...</div>';
    try {
      const { sessions } = await callBridge('__DS_LIST_SESSIONS__');
      renderSessions(sessions || []);
    } catch (e) {
      sessionListEl.innerHTML = '<div class="session-empty">加载失败: ' + escapeHtml(e.message) + '</div>';
    }
  }

  function renderSessions(sessions) {
    if (sessions.length === 0) {
      sessionListEl.innerHTML = '<div class="session-empty">暂无历史会话</div>';
      return;
    }
    sessionListEl.innerHTML = sessions.map((s) => {
      const safeTitle = escapeHtml(s.title || '(无标题)');
      const meta = [
        s.modelType ? '🤖 ' + escapeHtml(s.modelType) : '',
        '💬 ' + (s.messageCount || 0),
        s.toolCallCount ? '🛠 ' + s.toolCallCount : '',
        '🕐 ' + fmtTime(s.updatedAt || s.createdAt)
      ].filter(Boolean).join(' · ');
      return '<div class="session-item" data-id="' + escapeHtml(s.id) + '">' +
        '<div class="session-title">' + safeTitle + '</div>' +
        '<div class="session-meta">' + meta + '</div>' +
      '</div>';
    }).join('');

    sessionListEl.querySelectorAll('.session-item').forEach((el) => {
      el.addEventListener('click', () => openSession(el.dataset.id));
    });
  }

  async function openSession(id) {
    try {
      const { messages } = await callBridge('__DS_GET_MESSAGES__', { sessionId: id });
      const sess = await callBridge('__DS_LIST_SESSIONS__').then((r) => (r.sessions || []).find((s) => s.id === id));
      currentSession = sess || { id, title: '(未知)' };
      dialogTitleEl.textContent = currentSession.title;
      dialogBodyEl.innerHTML = renderMessages(messages || []);
      sessionMaskEl.style.display = 'flex';
    } catch (e) {
      setStatus('打开会话失败: ' + e.message, 'error');
    }
  }

  function renderMessages(messages) {
    if (messages.length === 0) return '<div class="session-empty">空会话</div>';
    return messages.map((m) => {
      const ts = fmtTime(m.createdAt);
      if (m.role === 'user') {
        return '<div class="dlg-msg user">' +
          '<div class="dlg-head">👤 用户 <span style="font-weight:400;color:#9ca3af">' + escapeHtml(ts) + '</span></div>' +
          '<div class="dlg-body">' + escapeHtml(m.content || '') + '</div>' +
        '</div>';
      } else if (m.role === 'assistant') {
        return '<div class="dlg-msg assistant">' +
          '<div class="dlg-head">🤖 助手 ' + (m.final ? '<span style="color:#059669">✓ 最终</span> ' : '') + '<span style="font-weight:400;color:#9ca3af">' + escapeHtml(ts) + '</span></div>' +
          '<div class="dlg-body">' + escapeHtml(m.content || '') + '</div>' +
        '</div>';
      } else if (m.role === 'tool_call') {
        return '<div class="dlg-msg tool-call">' +
          '<div class="dlg-head">🛠 工具调用 <code>' + escapeHtml(m.toolName || '') + '</code></div>' +
          '<pre>' + escapeHtml(m.content || '') + '</pre>' +
        '</div>';
      } else if (m.role === 'tool_result') {
        const tag = m.toolOk === false ? '❌' : '✅';
        return '<div class="dlg-msg tool-result' + (m.toolOk === false ? ' err' : '') + '">' +
          '<div class="dlg-head">' + tag + ' 结果 <code>' + escapeHtml(m.toolName || '') + '</code> <span style="font-weight:400;color:#9ca3af">' + escapeHtml((m.toolElapsedMs || 0) + 'ms') + '</span></div>' +
          (m.toolError ? '<div style="color:#b91c1c;font-size:11px">错误：' + escapeHtml(m.toolError) + '</div>' : '') +
          '<pre>' + escapeHtml(m.content || '') + '</pre>' +
        '</div>';
      }
      return '';
    }).join('');
  }

  function escapeHtml(s) {
    return String(s == null ? '' : s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function closeDialog() {
    sessionMaskEl.style.display = 'none';
    currentSession = null;
  }

  dialogCloseBtn.addEventListener('click', closeDialog);
  sessionMaskEl.addEventListener('click', (e) => {
    if (e.target === sessionMaskEl) closeDialog();
  });

  refreshHistoryBtn.addEventListener('click', loadHistory);
  clearHistoryBtn.addEventListener('click', async () => {
    if (!confirm('确定清空所有历史会话？此操作不可恢复。')) return;
    try {
      await callBridge('__DS_CLEAR_ALL__');
      setStatus('已清空历史', 'success');
      loadHistory();
    } catch (e) {
      setStatus('清空失败: ' + e.message, 'error');
    }
  });

  deleteSessionBtn.addEventListener('click', async () => {
    if (!currentSession) return;
    if (!confirm('删除这个会话？')) return;
    try {
      await callBridge('__DS_DELETE_SESSION__', { sessionId: currentSession.id });
      setStatus('已删除', 'success');
      closeDialog();
      loadHistory();
    } catch (e) {
      setStatus('删除失败: ' + e.message, 'error');
    }
  });

  // 导出按钮
  async function exportSession(format) {
    if (!currentSession) return;
    try {
      const r = await callBridge('__DS_EXPORT__', { sessionId: currentSession.id, format });
      const safeName = (currentSession.title || 'session').replace(/[\\/:*?"<>|]/g, '_').slice(0, 40);
      const filename = safeName + '-' + (currentSession.id.slice(-8)) + '.' + r.ext;
      let target = workingDir;
      let savedTo = '';
      // 优先写入工作目录（如已选），否则用 save picker
      if (target) {
        try {
          const fh = await target.getFileHandle(filename, { create: true });
          const w = await fh.createWritable();
          await w.write(r.content);
          await w.close();
          savedTo = '工作目录/' + filename;
        } catch (e) {
          target = null;
        }
      }
      if (!target) {
        if (!checkFsApi()) return;
        const handle = await window.showSaveFilePicker({
          suggestedName: filename,
          types: [{ description: format.toUpperCase(), accept: { [r.mime]: ['.' + r.ext] } }]
        });
        const w = await handle.createWritable();
        await w.write(r.content);
        await w.close();
        savedTo = handle.name;
      }
      setStatus('✅ 已导出 ' + format.toUpperCase() + ' → ' + savedTo, 'success');
    } catch (e) {
      if (e.name !== 'AbortError') setStatus('导出失败: ' + e.message, 'error');
    }
  }

  exportMdBtn.addEventListener('click', () => exportSession('markdown'));
  exportHtmlBtn.addEventListener('click', () => exportSession('html'));
  exportJsonBtn.addEventListener('click', () => exportSession('json'));

  // ============ MCP 服务器管理 ============
  async function loadMcp() {
    mcpListEl.innerHTML = '<div class="session-empty">加载中...</div>';
    try {
      const { servers } = await callBridge('__DS_MCP_LIST__');
      renderMcp(servers || []);
    } catch (e) {
      mcpListEl.innerHTML = '<div class="session-empty">加载失败: ' + escapeHtml(e.message) + '</div>';
    }
  }

  function renderMcp(servers) {
    if (servers.length === 0) {
      mcpListEl.innerHTML = '<div class="session-empty">未配置 MCP 服务器</div>';
      return;
    }
    mcpListEl.innerHTML = servers.map((s) => {
      const tag = s.connected ? '✅ 已连接' : '⚪ 未连接';
      const tools = (s.tools || []).slice(0, 5).map((t) => '<code>' + escapeHtml(t) + '</code>').join(' ');
      const more = s.tools && s.tools.length > 5 ? ' <span style="color:#9ca3af">+' + (s.tools.length - 5) + '</span>' : '';
      return '<div class="session-item" data-id="' + escapeHtml(s.id) + '">' +
        '<div class="session-title">' + tag + ' ' + escapeHtml(s.name) + ' <span style="font-weight:400;color:#9ca3af;font-size:11px">' + escapeHtml(s.type || 'http') + '</span></div>' +
        '<div class="session-meta">🔗 ' + escapeHtml(s.url) + ' · 🛠 ' + s.toolCount + ' 工具</div>' +
        (tools ? '<div class="session-meta" style="margin-top:2px">' + tools + more + '</div>' : '') +
        '<div style="margin-top:4px"><button class="secondary" data-act="refresh" data-id="' + escapeHtml(s.id) + '" style="padding:2px 8px;font-size:11px">🔄 刷新</button> <button class="danger" data-act="remove" data-id="' + escapeHtml(s.id) + '" style="padding:2px 8px;font-size:11px">删除</button></div>' +
      '</div>';
    }).join('');

    mcpListEl.querySelectorAll('button[data-act]').forEach((btn) => {
      btn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const id = btn.dataset.id;
        const act = btn.dataset.act;
        if (act === 'remove') {
          if (!confirm('删除此 MCP 服务器？')) return;
          try {
            await callBridge('__DS_MCP_REMOVE__', { id });
            setStatus('已删除', 'success');
            loadMcp();
          } catch (e2) {
            setStatus('删除失败: ' + e2.message, 'error');
          }
        } else if (act === 'refresh') {
          btn.disabled = true;
          btn.textContent = '连接中...';
          try {
            const { result } = await callBridge('__DS_MCP_REFRESH__', { id });
            if (result && result.ok) {
              setStatus('✅ ' + result.server.name + ' 已连接，' + result.toolCount + ' 个工具', 'success');
            } else {
              setStatus('连接失败: ' + (result && result.error), 'error');
            }
            loadMcp();
          } catch (e2) {
            setStatus('刷新失败: ' + e2.message, 'error');
          } finally {
            btn.disabled = false;
            btn.textContent = '🔄 刷新';
          }
        }
      });
    });
  }

  mcpAddBtn.addEventListener('click', async () => {
    const url = mcpUrlInput.value.trim();
    if (!url) {
      setStatus('请输入 URL', 'error');
      return;
    }
    const server = {
      name: mcpNameInput.value.trim() || url,
      url,
      type: mcpTypeSelect.value
    };
    mcpAddBtn.disabled = true;
    mcpAddBtn.textContent = '连接中...';
    try {
      const { server: added } = await callBridge('__DS_MCP_ADD__', { server });
      const { result } = await callBridge('__DS_MCP_REFRESH__', { id: added.id });
      if (result && result.ok) {
        setStatus('✅ 已添加并连接，' + result.toolCount + ' 个工具', 'success');
        mcpNameInput.value = '';
        mcpUrlInput.value = '';
      } else {
        setStatus('已添加，但连接失败: ' + (result && result.error), 'error');
      }
      loadMcp();
    } catch (e) {
      setStatus('添加失败: ' + e.message, 'error');
    } finally {
      mcpAddBtn.disabled = false;
      mcpAddBtn.textContent = '➕ 添加并连接';
    }
  });

  mcpRefreshAllBtn.addEventListener('click', async () => {
    mcpRefreshAllBtn.disabled = true;
    mcpRefreshAllBtn.textContent = '刷新中...';
    try {
      const { result } = await callBridge('__DS_MCP_REFRESH__', {});
      const arr = Array.isArray(result) ? result : (result ? [result] : []);
      const ok = arr.filter((r) => r.ok).length;
      setStatus('已刷新 ' + ok + '/' + arr.length + ' 个服务器', 'success');
      loadMcp();
    } catch (e) {
      setStatus('刷新失败: ' + e.message, 'error');
    } finally {
      mcpRefreshAllBtn.disabled = false;
      mcpRefreshAllBtn.textContent = '🔄 全部刷新';
    }
  });

  // 初始化时加载历史 + MCP
  loadHistory();
  loadMcp();
})();