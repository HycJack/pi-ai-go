// IndexedDB 存储封装（service worker 上下文）
// 通过 importScripts 加载到 background.js
// 全局挂在 self.DS_STORAGE

(function (global) {
  'use strict';

  // 直接用字符串常量避免 const 在 importScripts / strict 模式下的怪异行为
  const DB_NAME = 'deepseek_enhanced';
  const DB_VERSION = 2;

  let dbPromise = null;

  function openDB() {
    if (dbPromise) return dbPromise;
    dbPromise = new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = (e) => {
        const db = req.result;
        if (!db.objectStoreNames.contains('sessions')) {
          const s = db.createObjectStore('sessions', { keyPath: 'id' });
          s.createIndex('updatedAt', 'updatedAt');
        }
        if (!db.objectStoreNames.contains('messages')) {
          const m = db.createObjectStore('messages', { keyPath: 'id' });
          m.createIndex('sessionId', 'sessionId');
          m.createIndex('createdAt', 'createdAt');
        }
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
    return dbPromise;
  }

  function tx(storeNames, mode) {
    return openDB().then((db) => db.transaction(storeNames, mode));
  }

  function reqAsPromise(req) {
    return new Promise((resolve, reject) => {
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  }

  // === Sessions ===
  async function putSession(session) {
    const t = await tx('sessions', 'readwrite');
    await reqAsPromise(t.objectStore('sessions').put(session));
    return session;
  }

  async function getSession(id) {
    const t = await tx('sessions', 'readonly');
    return reqAsPromise(t.objectStore('sessions').get(id));
  }

  async function listSessions() {
    const t = await tx('sessions', 'readonly');
    const all = await reqAsPromise(t.objectStore('sessions').getAll());
    // 按 updatedAt 倒序
    return (all || []).sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0));
  }

  async function deleteSession(id) {
    const t = await tx(['sessions', 'messages'], 'readwrite');
    t.objectStore('sessions').delete(id);
    const idx = t.objectStore('messages').index('sessionId');
    const cursorReq = idx.openCursor(IDBKeyRange.only(id));
    await new Promise((resolve, reject) => {
      cursorReq.onsuccess = (e) => {
        const cursor = e.target.result;
        if (cursor) {
          cursor.delete();
          cursor.continue();
        } else {
          resolve();
        }
      };
      cursorReq.onerror = () => reject(cursorReq.error);
    });
  }

  async function clearAll() {
    const t = await tx(['sessions', 'messages'], 'readwrite');
    await Promise.all([
      reqAsPromise(t.objectStore('sessions').clear()),
      reqAsPromise(t.objectStore('messages').clear())
    ]);
  }

  // === Messages ===
  async function putMessage(message) {
    const t = await tx('messages', 'readwrite');
    await reqAsPromise(t.objectStore('messages').put(message));
    return message;
  }

  async function getMessagesBySession(sessionId) {
    const t = await tx('messages', 'readonly');
    const all = await reqAsPromise(t.objectStore('messages').index('sessionId').getAll(sessionId));
    return (all || []).sort((a, b) => (a.createdAt || 0) - (b.createdAt || 0));
  }

  // === 派生 session id ===
  // 由 messages 数组生成稳定的 sessionId
  function deriveSessionId(messages) {
    const firstUser = messages.find((m) => m.role === 'user');
    if (!firstUser) return null;
    const sys = messages.find((m) => m.role === 'system');
    const key = (sys ? sys.content.slice(0, 200) : '') + '|' + firstUser.content.slice(0, 200);
    let h = 0;
    for (let i = 0; i < key.length; i++) h = ((h << 5) - h + key.charCodeAt(i)) | 0;
    return 'sess-' + (h >>> 0).toString(16);
  }

  // 简单的 UUID
  function uuid() {
    return 'msg-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 10);
  }

  // === 捕获一次交互（请求 + 响应 + 工具调用）===
  async function captureInteraction({ sessionId, messages, responseText, toolRounds, modelType }) {
    const sid = sessionId || deriveSessionId(messages);
    if (!sid) return null;
    const now = Date.now();
    const firstUser = messages.find((m) => m.role === 'user');
    const title = firstUser ? firstUser.content.slice(0, 60).replace(/\s+/g, ' ').trim() : '(无标题)';

    // 更新 session
    const existing = await getSession(sid);
    const sess = {
      id: sid,
      title: title || existing?.title || '(空)',
      modelType: modelType || existing?.modelType || null,
      createdAt: existing?.createdAt || now,
      updatedAt: now,
      messageCount: 0,
      toolCallCount: 0
    };

    // 写消息：找到 messages 中所有 role=user/assistant 的，作为单条消息插入（去重）
    // 简化策略：本轮增量 = 找到与已有 messages 的差异，只插入新的
    const existingMsgs = await getMessagesBySession(sid);
    const existingKey = new Set(existingMsgs.map((m) => (m.role || '') + '|' + (m.content || '')));
    const newMsgs = [];
    let order = existingMsgs.length;
    for (const m of messages) {
      if (m.role !== 'user' && m.role !== 'assistant') continue;
      const key = (m.role || '') + '|' + (m.content || '');
      if (existingKey.has(key)) continue;
      newMsgs.push({
        id: uuid(),
        sessionId: sid,
        role: m.role,
        content: m.content,
        createdAt: now,
        order: order++
      });
    }

    // 把本轮最终的 assistant 响应（不含 tool_call 块之前的中间结果）作为单独记录
    if (responseText) {
      const finalKey = 'assistant|' + responseText;
      if (!existingKey.has(finalKey)) {
        newMsgs.push({
          id: uuid(),
          sessionId: sid,
          role: 'assistant',
          content: responseText,
          createdAt: now,
          order: order++,
          final: true
        });
      }
    }

    // 工具调用记录
    if (toolRounds && toolRounds.length > 0) {
      for (const r of toolRounds) {
        for (const call of r.calls || []) {
          newMsgs.push({
            id: uuid(),
            sessionId: sid,
            role: 'tool_call',
            content: JSON.stringify(call.args || {}, null, 2),
            toolName: call.name,
            createdAt: now,
            order: order++
          });
        }
        for (const res of r.results || []) {
          newMsgs.push({
            id: uuid(),
            sessionId: sid,
            role: 'tool_result',
            content: (res.summary || '') + (res.detail ? '\n\n' + res.detail : ''),
            toolName: res.name,
            toolOk: res.ok,
            toolElapsedMs: res.elapsedMs,
            toolError: res.error ? res.error.message : null,
            createdAt: now,
            order: order++
          });
        }
      }
    }

    // 批量写入消息
    for (const m of newMsgs) await putMessage(m);

    sess.messageCount = (existing?.messageCount || 0) + newMsgs.filter((m) => m.role === 'user' || m.role === 'assistant').length;
    sess.toolCallCount = (existing?.toolCallCount || 0) + newMsgs.filter((m) => m.role === 'tool_call').length;
    await putSession(sess);

    return { sessionId: sid, newMessageCount: newMsgs.length, session: sess };
  }

  global.DS_STORAGE = {
    openDB,
    putSession,
    getSession,
    listSessions,
    deleteSession,
    clearAll,
    putMessage,
    getMessagesBySession,
    deriveSessionId,
    captureInteraction
  };
})(typeof self !== 'undefined' ? self : this);