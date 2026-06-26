// MCP 客户端（service worker 上下文）
// 通过 importScripts 加载到 background.js
// 全局挂在 self.DS_MCP
// 配置存储在 chrome.storage.local['ds_mcp_servers']

(function (global) {
  'use strict';

  const PROTOCOL_VERSION = '2025-06-18';
  const STORAGE_KEY = 'ds_mcp_servers';
  const CLIENT_INFO = { name: 'deepseek-enhanced', version: '1.0.0' };
  const TIMEOUT_MS = 30000;

  // === 工具前缀 ===
  const TOOL_PREFIX = 'mcp';

  // === JSON-RPC 错误 ===
  class McpError extends Error {
    constructor(code, message, data) {
      super(message);
      this.code = code;
      this.data = data;
    }
  }

  // === 工具描述转参数 schema（简化 JSON Schema）===
  function schemaToParamsDescription(schema) {
    if (!schema || typeof schema !== 'object') return '- (无参数)';
    const props = schema.properties || {};
    const required = schema.required || [];
    const lines = [];
    for (const [name, def] of Object.entries(props)) {
      const req = required.includes(name) ? '必填' : '可选';
      const type = def.type || 'any';
      let line = '- `' + name + '` (' + type + ', ' + req + ')';
      if (def.description) line += ': ' + def.description;
      lines.push(line);
    }
    if (lines.length === 0) return '- (无参数)';
    return lines.join('\n');
  }

  // === McpClient 单个服务器连接 ===
  class McpClient {
    constructor(server) {
      this.server = server; // { id, name, url, type, headers? }
      this.sessionId = null;
      this.tools = [];
      this.capabilities = null;
      this.initialized = false;
      this._reqId = 0;
    }

    _newId() {
      this._reqId += 1;
      return this._reqId;
    }

    async _request(method, params) {
      const id = this._newId();
      const payload = {
        jsonrpc: '2.0',
        id,
        method,
        params: params || {}
      };

      const headers = Object.assign(
        { 'Content-Type': 'application/json', 'Accept': 'application/json, text/event-stream' },
        this.server.headers || {}
      );
      if (this.sessionId) headers['Mcp-Session-Id'] = this.sessionId;

      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), TIMEOUT_MS);

      let resp;
      try {
        resp = await fetch(this.server.url, {
          method: 'POST',
          headers,
          body: JSON.stringify(payload),
          signal: ctrl.signal
        });
      } catch (e) {
        clearTimeout(timer);
        throw new McpError(-32000, 'Network error: ' + e.message);
      }
      clearTimeout(timer);

      if (!resp.ok) {
        const text = await resp.text().catch(() => '');
        throw new McpError(resp.status, 'HTTP ' + resp.status + ': ' + text.slice(0, 200));
      }

      // 记录 session id（如果返回）
      const newSession = resp.headers.get('Mcp-Session-Id');
      if (newSession) this.sessionId = newSession;

      const ct = resp.headers.get('content-type') || '';
      if (ct.includes('text/event-stream')) {
        // 简化：SSE 响应只取第一个 JSON-RPC result/error
        return await this._readSseResponse(resp, id);
      } else {
        const json = await resp.json();
        if (json.error) throw new McpError(json.error.code, json.error.message, json.error.data);
        if (json.id !== id) throw new McpError(-32000, 'Response id mismatch');
        return json.result;
      }
    }

    async _readSseResponse(resp, expectedId) {
      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      let dataLines = [];
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        for (const line of lines) {
          if (line.startsWith('data:')) {
            dataLines.push(line.slice(5).trim());
          } else if (line === '' && dataLines.length > 0) {
            // 一个 event 结束
            const data = dataLines.join('\n');
            dataLines = [];
            if (!data || data === '[DONE]') continue;
            try {
              const obj = JSON.parse(data);
              if (obj.id === expectedId) {
                try { await reader.cancel(); } catch {}
                if (obj.error) throw new McpError(obj.error.code, obj.error.message, obj.error.data);
                return obj.result;
              }
            } catch (e) {
              if (e instanceof McpError) throw e;
            }
          }
        }
      }
      throw new McpError(-32000, 'SSE stream ended without response');
    }

    async initialize() {
      const result = await this._request('initialize', {
        protocolVersion: PROTOCOL_VERSION,
        capabilities: {},
        clientInfo: CLIENT_INFO
      });
      this.capabilities = result.capabilities || {};
      this.initialized = true;
      // initialized 通知（无响应）
      try {
        await this._request('notifications/initialized', {});
      } catch (e) {
        // 通知可能不需要响应，吞掉错误
      }
      return result;
    }

    async listTools() {
      if (!this.initialized) await this.initialize();
      const result = await this._request('tools/list', {});
      this.tools = result.tools || [];
      return this.tools;
    }

    async callTool(name, args) {
      if (!this.initialized) await this.initialize();
      const result = await this._request('tools/call', { name, arguments: args || {} });
      // 转换 content 数组为字符串
      if (result && Array.isArray(result.content)) {
        return result.content.map((c) => {
          if (c.type === 'text') return c.text || '';
          if (c.type === 'image') return '[image: ' + (c.mimeType || 'image') + ']';
          if (c.type === 'resource') return JSON.stringify(c.resource || c);
          return JSON.stringify(c);
        }).join('\n');
      }
      return result;
    }
  }

  // === 服务器管理 ===
  function loadServers() {
    return new Promise((resolve) => {
      chrome.storage.local.get([STORAGE_KEY], (r) => {
        resolve(r[STORAGE_KEY] || []);
      });
    });
  }

  function saveServers(servers) {
    return new Promise((resolve, reject) => {
      chrome.storage.local.set({ [STORAGE_KEY]: servers }, () => {
        if (chrome.runtime.lastError) reject(new Error(chrome.runtime.lastError.message));
        else resolve();
      });
    });
  }

  // === 注册 MCP 工具到工具注册表 ===
  function mcpToolName(serverId, toolName) {
    return TOOL_PREFIX + '__' + serverId + '__' + toolName;
  }
  function parseMcpToolName(fullName) {
    if (!fullName.startsWith(TOOL_PREFIX + '__')) return null;
    const rest = fullName.slice((TOOL_PREFIX + '__').length);
    const idx = rest.indexOf('__');
    if (idx < 0) return null;
    return { serverId: rest.slice(0, idx), toolName: rest.slice(idx + 2) };
  }

  // 把某个 server 的工具注册到 DS_TOOL_REGISTRY
  function registerServerTools(client) {
    const server = client.server;
    for (const t of client.tools) {
      const fullName = mcpToolName(server.id, t.name);
      // 防止和已有工具冲突
      if (global.DS_TOOL_REGISTRY.get(fullName)) {
        global.DS_TOOL_REGISTRY.unregister(fullName);
      }
      global.DS_TOOL_REGISTRY.register({
        name: fullName,
        description: '[MCP:' + server.name + '] ' + (t.description || t.name),
        parameters: schemaToParamsDescription(t.inputSchema),
        riskLevel: 'mcp',
        source: { type: 'mcp', serverId: server.id, serverName: server.name, toolName: t.name },
        execute: async (args) => {
          try {
            const text = await client.callTool(t.name, args);
            return { ok: true, summary: String(text || '').slice(0, 200), detail: String(text || '') };
          } catch (e) {
            return { ok: false, error: e.message || String(e) };
          }
        }
      });
    }
  }

  function unregisterServerTools(serverId) {
    const list = global.DS_TOOL_REGISTRY.list();
    for (const t of list) {
      if (t.source && t.source.type === 'mcp' && t.source.serverId === serverId) {
        global.DS_TOOL_REGISTRY.unregister(t.name);
      }
    }
  }

  // === 缓存的客户端实例 ===
  const clients = new Map(); // serverId -> McpClient

  async function getClient(server) {
    let c = clients.get(server.id);
    if (!c) {
      c = new McpClient(server);
      clients.set(server.id, c);
    }
    return c;
  }

  function evictClient(serverId) {
    unregisterServerTools(serverId);
    clients.delete(serverId);
  }

  // === 对外 API ===
  async function listServers() {
    return await loadServers();
  }

  async function addServer(server) {
    if (!server.id) server.id = 'srv-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 6);
    if (!server.name) server.name = server.url;
    const servers = await loadServers();
    servers.push(server);
    await saveServers(servers);
    return server;
  }

  async function updateServer(id, patch) {
    const servers = await loadServers();
    const idx = servers.findIndex((s) => s.id === id);
    if (idx < 0) throw new Error('Server not found: ' + id);
    servers[idx] = Object.assign({}, servers[idx], patch);
    await saveServers(servers);
    evictClient(id); // 改完重连
    return servers[idx];
  }

  async function removeServer(id) {
    const servers = await loadServers();
    const filtered = servers.filter((s) => s.id !== id);
    await saveServers(filtered);
    evictClient(id);
  }

  async function refreshServer(id) {
    const servers = await loadServers();
    const server = servers.find((s) => s.id === id);
    if (!server) throw new Error('Server not found: ' + id);
    evictClient(id); // 重新建立连接
    const c = await getClient(server);
    try {
      await c.listTools();
      registerServerTools(c);
      return { ok: true, server, toolCount: c.tools.length, tools: c.tools.map((t) => t.name) };
    } catch (e) {
      return { ok: false, server, error: e.message };
    }
  }

  async function refreshAll() {
    const servers = await loadServers();
    // 先清掉所有 MCP 工具
    for (const s of servers) evictClient(s.id);
    const results = [];
    for (const s of servers) {
      try {
        const c = await getClient(s);
        await c.listTools();
        registerServerTools(c);
        results.push({ id: s.id, name: s.name, ok: true, toolCount: c.tools.length });
      } catch (e) {
        results.push({ id: s.id, name: s.name, ok: false, error: e.message });
      }
    }
    return results;
  }

  async function getStatus() {
    const servers = await loadServers();
    return servers.map((s) => {
      const c = clients.get(s.id);
      return {
        id: s.id,
        name: s.name,
        url: s.url,
        type: s.type || 'http',
        connected: !!(c && c.initialized),
        toolCount: c ? c.tools.length : 0,
        tools: c ? c.tools.map((t) => t.name) : []
      };
    });
  }

  // 启动时自动连接
  async function bootstrap() {
    try {
      const results = await refreshAll();
      const ok = results.filter((r) => r.ok).length;
      console.log('[DS-MCP] bootstrap:', ok + '/' + results.length, 'servers connected');
    } catch (e) {
      console.warn('[DS-MCP] bootstrap failed:', e);
    }
  }

  global.DS_MCP = {
    PROTOCOL_VERSION,
    listServers,
    addServer,
    updateServer,
    removeServer,
    refreshServer,
    refreshAll,
    getStatus,
    bootstrap,
    mcpToolName,
    parseMcpToolName
  };
})(typeof self !== 'undefined' ? self : this);