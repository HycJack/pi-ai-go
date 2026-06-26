// ISOLATED world 桥接脚本 — 在 chrome.runtime 和页面 MAIN world 之间转发消息
// ISOLATED world 在 document_start 先于 MAIN world 执行

// chrome.runtime → window
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg && msg.type === '__DS_PUSH_SETTINGS__') {
    window.postMessage({
      type: '__DS_SETTINGS__',
      prompt: msg.prompt,
      enabled: msg.enabled
    }, '*');
  }
  return false;
});

// window → chrome.runtime：把 hook / sidepanel 的请求转发到 service worker，再把响应 post 回 MAIN world
const FORWARD_TYPES = new Set([
  '__DS_REQUEST_SETTINGS__',
  '__DS_GET_TOOLS_PROMPT__',
  '__DS_PARSE_TOOL_CALLS__',
  '__DS_EXECUTE_TOOLS__',
  '__DS_CAPTURE__',
  '__DS_LIST_SESSIONS__',
  '__DS_GET_MESSAGES__',
  '__DS_DELETE_SESSION__',
  '__DS_CLEAR_ALL__',
  '__DS_EXPORT__',
  '__DS_MCP_LIST__',
  '__DS_MCP_ADD__',
  '__DS_MCP_UPDATE__',
  '__DS_MCP_REMOVE__',
  '__DS_MCP_REFRESH__'
]);

window.addEventListener('message', (event) => {
  if (event.source !== window) return;
  const data = event.data;
  if (!data || !data.type || !FORWARD_TYPES.has(data.type)) return;

  // 大部分消息直接转发 type 即可，部分需要带 payload
  const req = { type: data.type };
  if (data.type === '__DS_PARSE_TOOL_CALLS__') req.text = data.text;
  if (data.type === '__DS_EXECUTE_TOOLS__') req.calls = data.calls;
  if (data.type === '__DS_CAPTURE__') req.payload = data.payload;
  if (data.type === '__DS_GET_MESSAGES__') req.sessionId = data.sessionId;
  if (data.type === '__DS_DELETE_SESSION__') req.sessionId = data.sessionId;
  if (data.type === '__DS_EXPORT__') {
    req.sessionId = data.sessionId;
    req.format = data.format;
  }
  if (data.type === '__DS_MCP_ADD__') req.server = data.server;
  if (data.type === '__DS_MCP_UPDATE__') {
    req.id = data.id;
    req.patch = data.patch;
  }
  if (data.type === '__DS_MCP_REMOVE__') req.id = data.id;
  if (data.type === '__DS_MCP_REFRESH__') req.id = data.id;

  chrome.runtime.sendMessage(req, (response) => {
    if (chrome.runtime.lastError) return;
    if (!response) return;
    window.postMessage({
      type: data.type + '__DS_RESP__',
      ok: response.error ? false : true,
      error: response.error,
      settings: response.settings,
      prompt: response.prompt,
      tools: response.tools,
      calls: response.calls,
      results: response.results,
      result: response.result,
      sessions: response.sessions,
      messages: response.messages,
      content: response.content,
      mime: response.mime,
      ext: response.ext,
      session: response.session,
      messageCount: response.messageCount,
      servers: response.servers,
      server: response.server
    }, '*');
  });
});