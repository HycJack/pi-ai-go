// 内容脚本 — 通过 chrome.scripting.registerContentScripts 注册
// runAt: 'document_start' + world: 'MAIN' 保证在任何页面脚本之前注入
(function () {
  if (window.__dsInstalled) return;
  window.__dsInstalled = true;

  console.log('[DS] ⚡ installed @', new Date().toISOString());

  const DEFAULT_PROMPT = `你叫小七，是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  const MAX_TOOL_ROUNDS = 3;

  let currentPrompt = DEFAULT_PROMPT;
  let currentEnabled = true;
  let toolExecutionEnabled = true;
  let cachedToolsPrompt = '';

  // ============ 通用 bridge 调用 ============
  function callBridge(type, payload) {
    return new Promise((resolve, reject) => {
      const respType = type + '__DS_RESP__';
      const onMsg = (event) => {
        if (event.source !== window) return;
        const d = event.data;
        if (!d || d.type !== respType) return;
        window.removeEventListener('message', onMsg);
        clearTimeout(timer);
        if (d.error) reject(new Error(d.error));
        else resolve(d);
      };
      window.addEventListener('message', onMsg);
      const timer = setTimeout(() => {
        window.removeEventListener('message', onMsg);
        reject(new Error('Bridge timeout: ' + type));
      }, 30000);
      window.postMessage(Object.assign({ type }, payload || {}), '*');
    });
  }

  // 预取工具提示词（首次拦截前调用）
  async function prefetchToolsPrompt() {
    try {
      const r = await callBridge('__DS_GET_TOOLS_PROMPT__');
      cachedToolsPrompt = r.prompt || '';
    } catch (e) {
      console.warn('[DS] prefetch tools prompt failed:', e);
    }
  }

  // ============ 解析请求 body ============
  // 兼容多种格式：
  //  A) 标准 OpenAI 风格：body.messages = [...]
  //  B) Hikvision 自定义：body.message = "JSON.stringify(messages)"
  //  C) 嵌套：body.data.messages = [...]
  function parseRequest(bodyStr) {
    let body = null;
    try {
      body = JSON.parse(bodyStr);
    } catch (e) {
      return null;
    }
    if (!body || typeof body !== 'object') return null;

    // 候选位置
    const candidates = [
      body.messages,
      body.message,
      body && body.data && body.data.messages,
      body && body.data && body.data.message,
      body && body.payload && body.payload.messages
    ];
    for (const c of candidates) {
      if (Array.isArray(c)) return { body, messages: c };
      if (typeof c === 'string') {
        try {
          const m = JSON.parse(c);
          if (Array.isArray(m)) return { body, messages: m };
        } catch (e) {}
      }
    }
    return null;
  }

  function injectSystemPrompt(messages, extraPrompt) {
    const systemIdx = messages.findIndex((m) => m.role === 'system');
    // 我们的 prompt + 工具定义放在最前，原始 system 内容追加在后
    // 这样 LLM 优先看到我们的小七设定 + 工具说明
    const parts = [currentPrompt];
    if (extraPrompt) parts.push(extraPrompt);
    if (systemIdx === -1) {
      messages.unshift({ role: 'system', content: parts.join('\n\n---\n\n') });
    } else {
      const orig = messages[systemIdx].content || '';
      messages[systemIdx].content = parts.join('\n\n---\n\n') + (orig ? '\n\n---\n\n' + orig : '');
    }
    return messages;
  }

  function rebuildBody(originalBody, messages) {
    const b = JSON.parse(JSON.stringify(originalBody));
    // 写回原字段（保持格式一致）
    if (Array.isArray(originalBody.messages)) {
      b.messages = messages;
    } else if (typeof originalBody.message === 'string') {
      b.message = JSON.stringify(messages);
    } else if (originalBody.data && originalBody.data.messages) {
      b.data.messages = messages;
    } else if (originalBody.data && typeof originalBody.data.message === 'string') {
      b.data.message = JSON.stringify(messages);
    } else {
      // 兜底：用 message 字段
      b.message = JSON.stringify(messages);
    }
    return JSON.stringify(b);
  }

  // ============ 解析响应 ============
  // 支持非流式 JSON 和流式 SSE，兼容多种 LLM 响应格式
  function pickTextFromJson(obj) {
    if (!obj || typeof obj !== 'object') return '';
    // OpenAI 风格：choices[0].message.content / choices[0].text / choices[0].delta.content
    if (Array.isArray(obj.choices) && obj.choices.length > 0) {
      const c = obj.choices[0];
      if (c.message && typeof c.message.content === 'string') return c.message.content;
      if (c.delta && typeof c.delta.content === 'string') return c.delta.content;
      if (typeof c.text === 'string') return c.text;
    }
    // 其它风格：data.choices / result.message / message.content / content
    if (Array.isArray(obj.data) && obj.data[0]) return pickTextFromJson(obj.data[0]);
    if (obj.result) {
      if (typeof obj.result === 'string') return obj.result;
      if (obj.result.message && typeof obj.result.message.content === 'string') return obj.result.message.content;
      if (Array.isArray(obj.result.choices) && obj.result.choices[0]) {
        const c = obj.result.choices[0];
        if (c.message && typeof c.message.content === 'string') return c.message.content;
        if (c.delta && typeof c.delta.content === 'string') return c.delta.content;
      }
    }
    if (obj.message && typeof obj.message === 'object' && typeof obj.message.content === 'string') return obj.message.content;
    if (typeof obj.message === 'string') return obj.message;
    if (typeof obj.content === 'string') return obj.content;
    if (typeof obj.answer === 'string') return obj.answer;
    return '';
  }

  function extractAssistantText(rawText, contentType) {
    if (!rawText) return '';
    const isSSE = (contentType || '').includes('event-stream');

    if (isSSE) {
      let acc = '';
      const lines = rawText.split('\n');
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed.startsWith('data:')) continue;
        const data = trimmed.slice(5).trim();
        if (!data || data === '[DONE]') continue;
        try {
          const obj = JSON.parse(data);
          acc += pickTextFromJson(obj);
        } catch {}
      }
      return acc;
    }

    // 非 SSE：可能是 JSON 也可能是纯文本
    try {
      const obj = JSON.parse(rawText);
      const text = pickTextFromJson(obj);
      if (text) return text;
    } catch {}
    return rawText;
  }

  function formatToolResults(results) {
    if (!results || results.length === 0) return '(no tool results)';
    return results.map((r) => {
      const tag = r.ok ? '✅' : '❌';
      const head = tag + ' tool=' + r.name + ' (' + r.elapsedMs + 'ms)';
      let body = r.summary || '';
      if (r.detail) body += '\n' + r.detail;
      if (r.error) body += '\n[error: ' + r.error.message + ']';
      return '### ' + head + '\n```\n' + body + '\n```';
    }).join('\n\n');
  }

  // ============ fetch 拦截 + 工具执行循环 ============
  const origFetch = window.fetch.bind(window);

  // 把完整文本（SSE 或 JSON）包回成与原 content-type 一致的 Response
  function wrapResponse(rawText, contentType, status, statusText) {
    if (contentType && contentType.includes('event-stream')) {
      // 重新包成 SSE 响应流
      const encoder = new TextEncoder();
      const stream = new ReadableStream({
        start(controller) {
          controller.enqueue(encoder.encode(rawText));
          controller.close();
        }
      });
      return new Response(stream, {
        status: status || 200,
        statusText: statusText || 'OK',
        headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' }
      });
    }
    return new Response(rawText, {
      status: status || 200,
      statusText: statusText || 'OK',
      headers: { 'Content-Type': contentType || 'application/json' }
    });
  }

  // 把 stream body 完整读成字符串
  async function readStreamToString(response) {
    if (!response.body) {
      return await response.text();
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let acc = '';
    try {
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        acc += decoder.decode(value, { stream: true });
      }
      acc += decoder.decode();
    } finally {
      try { reader.releaseLock(); } catch {}
    }
    return acc;
  }

  async function interceptedFetch(url, init) {
    if (!url.includes('/v1/chat/completions') || typeof init.body !== 'string') {
      return origFetch(url, init);
    }

    console.log('[DS] ➡️ fetch', url);

    const parsed = parseRequest(init.body);
    if (!parsed) {
      console.warn('[DS] ⚠️ cannot parse body, pass through');
      return origFetch(url, init);
    }

    // 1) 注入 system prompt（含工具定义）
    if (!cachedToolsPrompt) await prefetchToolsPrompt();
    const initialMessages = injectSystemPrompt(parsed.messages.slice(), cachedToolsPrompt);
    const initialBody = rebuildBody(parsed.body, initialMessages);
    console.log('[DS] 📝 system prompt injected, tools section length:', cachedToolsPrompt.length);

    // 2) 多轮：发请求 → 读取完整响应 → 解析 tool_call → 执行 → 重新发
    let currentMessages = initialMessages;
    let currentBody = initialBody;
    let finalRaw = '';
    let finalAssistantText = '';
    let finalStatus = 200;
    let finalStatusText = 'OK';
    let finalContentType = 'application/json';
    const toolRounds = [];

    for (let round = 0; round <= MAX_TOOL_ROUNDS; round++) {
      const response = await origFetch(url, Object.assign({}, init, { body: currentBody }));

      finalStatus = response.status;
      finalStatusText = response.statusText;
      finalContentType = response.headers.get('content-type') || 'application/json';

      // 无论流式还是非流式，都读完整 body
      const raw = await readStreamToString(response);
      finalRaw = raw;

      const assistantText = extractAssistantText(raw, finalContentType);
      finalAssistantText = assistantText;
      console.log('[DS] 📥 round ' + round + ' response, text length:', assistantText.length, 'preview:', JSON.stringify(assistantText.slice(0, 200)));

      if (!toolExecutionEnabled) break;

      // 解析 tool_call
      let calls = [];
      try {
        const r = await callBridge('__DS_PARSE_TOOL_CALLS__', { text: assistantText });
        calls = r.calls || [];
      } catch (e) {
        console.warn('[DS] parse tool_calls failed:', e);
        break;
      }
      if (calls.length === 0) {
        console.log('[DS] 📭 no tool_call found in response');
        break;
      }

      console.log('[DS] 🛠 tool calls round', round + 1, ':', calls.map((c) => c.name), 'args:', calls.map((c) => JSON.stringify(c.args)));

      // 执行
      let results = [];
      try {
        const r = await callBridge('__DS_EXECUTE_TOOLS__', { calls });
        results = r.results || [];
      } catch (e) {
        console.warn('[DS] execute tools failed:', e);
        break;
      }
      console.log('[DS] 🛠 results:', results.map((x) => x.name + (x.ok ? '✅' : '❌')));

      toolRounds.push({ calls, results });

      // 把 assistant 文本 + tool 结果追加进 messages
      currentMessages = currentMessages.concat([
        { role: 'assistant', content: assistantText },
        { role: 'user', content: '工具执行结果：\n\n' + formatToolResults(results) + '\n\n请基于以上结果继续回答用户。' }
      ]);

      // 重新发请求
      currentBody = rebuildBody(parsed.body, currentMessages);
    }

    // 3) 捕获本次会话到 IndexedDB
    try {
      await callBridge('__DS_CAPTURE__', {
        payload: {
          messages: deriveSessionMessages(parsed.messages, finalAssistantText),
          responseText: finalAssistantText,
          toolRounds,
          modelType: parsed.body.model || parsed.body.modelType || null
        }
      });
      console.log('[DS] 💾 captured');
    } catch (e) {
      console.warn('[DS] capture failed:', e);
    }

    // 4) 把最终响应包回原格式
    return wrapResponse(finalRaw, finalContentType, finalStatus, finalStatusText);
  }

  // 推导 sessionId 并构造要存储的 messages 列表
  // 简化：把 finalText 之前的 messages + finalText 作为完整对话
  function deriveSessionMessages(originalMessages, finalText) {
    // 直接使用原始 messages + finalText 作为 assistant 的最终回复
    const list = originalMessages.slice();
    // 把最后一条 assistant 替换为 finalText（如果存在），否则追加
    const lastIdx = list.length - 1;
    if (lastIdx >= 0 && list[lastIdx].role === 'assistant') {
      list[lastIdx] = { role: 'assistant', content: finalText };
    } else {
      list.push({ role: 'assistant', content: finalText });
    }
    return list;
  }

  window.fetch = new Proxy(origFetch, {
    apply(target, thisArg, args) {
      const input = args[0];
      const init = args[1] || {};
      const url = typeof input === 'string' ? input : (input && input.url ? input.url : '');

      if (currentEnabled && url.includes('/v1/chat/completions') && typeof init.body === 'string') {
        // 异步拦截，返回 Promise
        return interceptedFetch(url, init).catch((e) => {
          console.error('[DS] interceptedFetch error, fallback:', e);
          // 出错时退回原始请求
          return Reflect.apply(target, thisArg, args);
        });
      }

      console.log('[DS] ➡️ fetch', url);
      return Reflect.apply(target, thisArg, args);
    }
  });

  // === XHR ===
  const origOpen = XMLHttpRequest.prototype.open;
  const origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (m, u, ...r) {
    this.__dsUrl = u;
    return origOpen.call(this, m, u, ...r);
  };
  XMLHttpRequest.prototype.send = function (body) {
    const url = this.__dsUrl || '';
    console.log('[DS] ➡️ XHR', url);
    if (currentEnabled && url.includes('/v1/chat/completions') && typeof body === 'string') {
      const parsed = parseRequest(body);
      if (parsed) {
        const messages = injectSystemPrompt(parsed.messages.slice(), cachedToolsPrompt);
        body = rebuildBody(parsed.body, messages);
        console.log('[DS] 📦 XHR body augmented');
      }
    }
    return origSend.call(this, body);
  };

  // === EventSource ===
  const OrigES = window.EventSource;
  window.EventSource = new Proxy(OrigES, {
    construct(target, args) {
      console.log('[DS] ➡️ EventSource', args[0]);
      return new target(...args);
    }
  });
  window.EventSource.prototype = OrigES.prototype;

  // === Service Worker 注册 ===
  if (navigator.serviceWorker) {
    const origReg = navigator.serviceWorker.register.bind(navigator.serviceWorker);
    navigator.serviceWorker.register = function (...args) {
      console.log('[DS] ➡️ SW.register', args[0]);
      return origReg(...args);
    };
  }

  // === Worker / SharedWorker ===
  if (window.Worker) {
    const OrigWorker = window.Worker;
    window.Worker = new Proxy(OrigWorker, {
      construct(target, args) {
        console.log('[DS] ➡️ Worker', args[0]);
        return new target(...args);
      }
    });
  }
  if (window.SharedWorker) {
    const OrigSW = window.SharedWorker;
    window.SharedWorker = new Proxy(OrigSW, {
      construct(target, args) {
        console.log('[DS] ➡️ SharedWorker', args[0]);
        return new target(...args);
      }
    });
  }

  // === sendBeacon ===
  if (navigator.sendBeacon) {
    const origBeacon = navigator.sendBeacon.bind(navigator);
    navigator.sendBeacon = function (url, data) {
      console.log('[DS] ➡️ sendBeacon', url);
      if (currentEnabled && typeof url === 'string' && url.includes('/v1/chat/completions') && typeof data === 'string') {
        const parsed = parseRequest(data);
        if (parsed) {
          const messages = injectSystemPrompt(parsed.messages.slice(), cachedToolsPrompt);
          data = rebuildBody(parsed.body, messages);
          console.log('[DS] 📦 sendBeacon body augmented');
        }
      }
      return origBeacon(url, data);
    };
  }

  // 接收来自 service worker 的设置更新
  window.addEventListener('message', (event) => {
    if (event.source !== window) return;
    const data = event.data;
    if (!data) return;
    if (data.type === '__DS_SETTINGS__') {
      if (typeof data.prompt === 'string') currentPrompt = data.prompt || DEFAULT_PROMPT;
      if (typeof data.enabled === 'boolean') currentEnabled = data.enabled;
    }
  });

  // 注入完成，请求初始设置 + 预取工具提示词
  queueMicrotask(() => {
    window.postMessage({ type: '__DS_REQUEST_SETTINGS__' }, '*');
    prefetchToolsPrompt();
  });

  console.log('[DS] 🚀 ready');
})();