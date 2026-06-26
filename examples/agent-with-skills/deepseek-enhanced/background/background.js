// 加载工具注册表 + 存储 + 导出器 + MCP
importScripts('tool-registry.js');
importScripts('storage.js');
importScripts('exporters.js');
importScripts('mcp.js');

const HOOK_ID = 'ds-hook-main';
const BRIDGE_ID = 'ds-bridge';
const HOOK_MATCHES = ['https://lanz.hikvision.com/*'];
const STORAGE_KEY = 'deepseek_enhanced_settings';

// 与 hook.js 中的 DEFAULT_PROMPT 保持一致；空存储时使用此值
const DEFAULT_PROMPT = `你叫小七，是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

async function registerScripts() {
  try {
    // 幂等：先清理可能的旧注册
    await chrome.scripting
      .unregisterContentScripts({ ids: [HOOK_ID, BRIDGE_ID] })
      .catch(() => {});

    await chrome.scripting.registerContentScripts([
      {
        // MAIN world — 实际拦截 fetch/XHR/...
        id: HOOK_ID,
        matches: HOOK_MATCHES,
        js: ['background/hook.js'],
        world: 'MAIN',
        runAt: 'document_start',
        allFrames: true
      },
      {
        // ISOLATED world — 在 chrome.runtime 和 MAIN world 之间桥接消息
        id: BRIDGE_ID,
        matches: HOOK_MATCHES,
        js: ['background/bridge.js'],
        world: 'ISOLATED',
        runAt: 'document_start',
        allFrames: true
      }
    ]);
    console.log('[DeepSeek Enhanced] scripts registered');
  } catch (e) {
    console.error('[DeepSeek Enhanced] registerScripts failed:', e);
  }
}

// 从存储读取设置；没有则使用 DEFAULT_PROMPT
function getSettings() {
  return new Promise((resolve) => {
    chrome.storage.local.get(STORAGE_KEY, (result) => {
      const s = result[STORAGE_KEY];
      if (s && typeof s.prompt === 'string') {
        resolve({ prompt: s.prompt, enabled: s.enabled !== false });
      } else {
        resolve({ prompt: DEFAULT_PROMPT, enabled: true });
      }
    });
  });
}

// 响应 bridge 的设置查询
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg && msg.type === '__DS_GET_SETTINGS__') {
    getSettings().then((settings) => sendResponse({ settings }));
    return true; // 异步 sendResponse，保持通道
  }
  return false;
});

// 工具相关：返回工具提示词片段、解析 tool_call、执行工具
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || !msg.type) return false;

  if (msg.type === '__DS_GET_TOOLS_PROMPT__') {
    sendResponse({ prompt: self.DS_TOOL_REGISTRY.getToolsPromptSection(), tools: self.DS_TOOL_REGISTRY.list().map(t => ({ name: t.name, description: t.description })) });
    return false;
  }

  if (msg.type === '__DS_PARSE_TOOL_CALLS__') {
    const calls = self.DS_TOOL_REGISTRY.parseToolCalls(msg.text || '');
    sendResponse({ calls });
    return false;
  }

  if (msg.type === '__DS_EXECUTE_TOOLS__') {
    self.DS_TOOL_REGISTRY
      .executeAll(msg.calls || [])
      .then((results) => sendResponse({ results }))
      .catch((e) => sendResponse({ error: e.message }));
    return true; // 异步
  }

  // 存储：捕获一次交互
  if (msg.type === '__DS_CAPTURE__') {
    self.DS_STORAGE.captureInteraction(msg.payload || {})
      .then((r) => sendResponse({ result: r }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }

  // 存储：列出会话
  if (msg.type === '__DS_LIST_SESSIONS__') {
    self.DS_STORAGE.listSessions()
      .then((sessions) => sendResponse({ sessions }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }

  // 存储：取会话消息
  if (msg.type === '__DS_GET_MESSAGES__') {
    self.DS_STORAGE.getMessagesBySession(msg.sessionId)
      .then((messages) => sendResponse({ messages }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }

  // 存储：删除会话
  if (msg.type === '__DS_DELETE_SESSION__') {
    self.DS_STORAGE.deleteSession(msg.sessionId)
      .then(() => sendResponse({ ok: true }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }

  // 存储：清空全部
  if (msg.type === '__DS_CLEAR_ALL__') {
    self.DS_STORAGE.clearAll()
      .then(() => sendResponse({ ok: true }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }

  // 导出：取会话并生成指定格式内容
  if (msg.type === '__DS_EXPORT__') {
    (async () => {
      try {
        const sess = await self.DS_STORAGE.getSession(msg.sessionId);
        if (!sess) {
          sendResponse({ error: 'Session not found: ' + msg.sessionId });
          return;
        }
        const msgs = await self.DS_STORAGE.getMessagesBySession(msg.sessionId);
        const fmt = msg.format || 'markdown';
        let content, mime, ext;
        if (fmt === 'json') {
          content = self.DS_EXPORTERS.toJson(sess, msgs);
          mime = 'application/json';
          ext = 'json';
        } else if (fmt === 'html') {
          content = self.DS_EXPORTERS.toHtml(sess, msgs);
          mime = 'text/html';
          ext = 'html';
        } else {
          content = self.DS_EXPORTERS.toMarkdown(sess, msgs);
          mime = 'text/markdown';
          ext = 'md';
        }
        sendResponse({ content, mime, ext, session: sess, messageCount: msgs.length });
      } catch (e) {
        sendResponse({ error: e.message });
      }
    })();
    return true;
  }

  // === MCP 消息 ===
  if (msg.type === '__DS_MCP_LIST__') {
    self.DS_MCP.getStatus()
      .then((servers) => sendResponse({ servers }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }
  if (msg.type === '__DS_MCP_ADD__') {
    self.DS_MCP.addServer(msg.server || {})
      .then((server) => sendResponse({ server }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }
  if (msg.type === '__DS_MCP_UPDATE__') {
    self.DS_MCP.updateServer(msg.id, msg.patch || {})
      .then((server) => sendResponse({ server }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }
  if (msg.type === '__DS_MCP_REMOVE__') {
    self.DS_MCP.removeServer(msg.id)
      .then(() => sendResponse({ ok: true }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }
  if (msg.type === '__DS_MCP_REFRESH__') {
    const p = msg.id ? self.DS_MCP.refreshServer(msg.id) : self.DS_MCP.refreshAll();
    Promise.resolve(p)
      .then((result) => sendResponse({ result }))
      .catch((e) => sendResponse({ error: e.message }));
    return true;
  }

  return false;
});

// 把当前设置推送到所有匹配的标签页
async function pushSettingsToAllTabs() {
  const settings = await getSettings();
  const tabs = await chrome.tabs.query({ url: HOOK_MATCHES });
  for (const tab of tabs) {
    chrome.tabs
      .sendMessage(tab.id, {
        type: '__DS_PUSH_SETTINGS__',
        prompt: settings.prompt,
        enabled: settings.enabled
      })
      .catch(() => {});
  }
}

// 侧边栏保存 → 触发 storage.onChanged → 推送到所有标签页
chrome.storage.onChanged.addListener((changes, area) => {
  if (area !== 'local' || !changes[STORAGE_KEY]) return;
  pushSettingsToAllTabs();
});

// 顶层调用：处理首次启动 + 开发时点 "重新加载" 的情况（onInstalled 不会触发）
registerScripts();
self.DS_MCP.bootstrap();

// 安装 / 启动时也注册一次（幂等）
chrome.runtime.onInstalled.addListener(() => {
  chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true });
  registerScripts();
  // 启动时连接已配置的 MCP 服务器
  self.DS_MCP.bootstrap();
});

chrome.runtime.onStartup.addListener(() => {
  registerScripts();
  self.DS_MCP.bootstrap();
});

// 点击图标打开侧边栏
if (chrome.action && chrome.action.onClicked) {
  chrome.action.onClicked.addListener(async (tab) => {
    try {
      await chrome.sidePanel.open({ tabId: tab.id });
    } catch (e) {}
  });
}