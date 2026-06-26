// 工具注册表（运行在 service worker）
// 通过 importScripts 加载到 background.js
// 全局挂在 self.DS_TOOL_REGISTRY

(function (global) {
  'use strict';

  // ============ 工具描述符格式 ============
  // {
  //   id: 'builtin.get_current_time',
  //   name: 'get_current_time',
  //   description: '获取当前时间',
  //   inputSchema: { type: 'object', properties: {...}, required: [...] },
  //   provider: 'builtin',
  //   execute: async (args) => any
  // }

  const tools = new Map(); // name -> descriptor

  function register(descriptor) {
    if (!descriptor || typeof descriptor.name !== 'string') {
      throw new Error('Tool descriptor must have name');
    }
    if (typeof descriptor.execute !== 'function') {
      throw new Error('Tool ' + descriptor.name + ' missing execute()');
    }
    const id = descriptor.id || descriptor.provider + '.' + descriptor.name;
    tools.set(descriptor.name, { ...descriptor, id });
    console.log('[DS] tool registered:', descriptor.name);
  }

  function unregister(name) {
    return tools.delete(name);
  }

  function get(name) {
    return tools.get(name);
  }

  function list() {
    return Array.from(tools.values());
  }

  // ============ 提示词生成 ============
  function getToolsPromptSection() {
    const all = list();
    if (all.length === 0) return '';

    let section = '## 可用工具\n\n';
    section += '你可以调用以下工具来完成任务。**当且仅当**用户问题需要工具时，按下面的 XML 格式输出（一次响应可调用多个工具）：\n\n';
    section += '### 调用格式（严格遵守）\n\n';
    section += '```\n<tool_call name="tool_name">\n{"参数1": "值1", "参数2": 123}\n</tool_call>\n```\n\n';
    section += '### 调用示例\n\n';
    section += '用户：现在几点了？\n';
    section += '你的回复（先调用工具，不要直接回答）：\n';
    section += '```\n<tool_call name="get_current_time">\n{"timezone": "Asia/Shanghai"}\n</tool_call>\n```\n\n';
    section += '用户：帮我算一下 (3+5)*2\n';
    section += '你的回复：\n';
    section += '```\n<tool_call name="calculate">\n{"expression": "(3+5)*2"}\n</tool_call>\n```\n\n';
    section += '**重要规则：**\n';
    section += '1. 调用工具时**只输出 tool_call 块**，不要附带任何解释文字。\n';
    section += '2. 参数名必须用双引号，值用 JSON 标准格式（字符串加引号，数字不加）。\n';
    section += '3. 调用结束后系统会自动执行工具并把结果反馈给你，你**再**基于结果给用户最终回答。\n';
    section += '4. 如果用户问题不需要工具（比如普通聊天），**直接回答**，不要输出 tool_call。\n';
    section += '5. 不要捏造不存在的工具名；只用下面列表里的工具。\n\n';
    section += '### 工具列表\n\n';

    for (const t of all) {
      section += '#### `' + t.name + '`\n';
      section += (t.description || '').trim() + '\n\n';

      const schema = t.inputSchema;
      if (schema && schema.properties && Object.keys(schema.properties).length > 0) {
        section += '**参数：**\n';
        const required = new Set(schema.required || []);
        for (const [pname, pdef] of Object.entries(schema.properties)) {
          const reqMark = required.has(pname) ? '**必填**' : '可选';
          const ptype = pdef.type || 'any';
          const pdesc = pdef.description || '';
          section += '- `' + pname + '` (' + ptype + ', ' + reqMark + '): ' + pdesc + '\n';
        }
        section += '\n';
      }
    }
    return section;
  }

  // ============ 解析响应中的 tool_call ============
  // 多种格式兼容：
  //  1) <tool_call name="x">{...}</tool_call>
  //  2) <tool_call name='x'>{...}</tool_call>
  //  3) <tool_call name="x" args='{...}'></tool_call>
  //  4) <tool_call name="x" arguments='{...}'></tool_call>
  //  5) <tool_call name="x">{...}</tool_call>   (宽松：未严格闭合)
  const TOOL_CALL_RE = /<tool_call\b([^>]*)>([\s\S]*?)(?:<\/tool_call>|$)/gi;

  function parseToolCalls(text) {
    if (typeof text !== 'string') return [];
    const calls = [];
    let m;
    TOOL_CALL_RE.lastIndex = 0;
    while ((m = TOOL_CALL_RE.exec(text)) !== null) {
      const attrsRaw = m[1] || '';
      const bodyRaw = (m[2] || '').trim();
      // 解析 name
      const nameMatch = attrsRaw.match(/\bname\s*=\s*(["'])([^"']+)\1/i);
      if (!nameMatch) continue;
      const name = nameMatch[2];
      // 解析 args：先看 attrs 里的 args= / arguments=，否则用 body
      let args = {};
      let parseError = null;
      let raw = bodyRaw;
      const argsAttr = attrsRaw.match(/\b(?:args|arguments|parameters)\s*=\s*(["'])([\s\S]*?)\1/i);
      if (argsAttr) {
        raw = argsAttr[2];
      }
      try {
        if (raw) args = JSON.parse(raw);
      } catch (e) {
        parseError = e.message;
      }
      calls.push({ name, args, raw, parseError });
    }
    return calls;
  }

  // ============ 执行工具 ============
  async function executeOne(call) {
    const t = tools.get(call.name);
    const startedAt = Date.now();
    if (!t) {
      return {
        ok: false,
        name: call.name,
        args: call.args,
        summary: 'Tool not found: ' + call.name,
        detail: '',
        error: { code: 'TOOL_NOT_FOUND', message: 'Tool not registered: ' + call.name, retryable: false },
        elapsedMs: Date.now() - startedAt
      };
    }
    if (call.parseError) {
      return {
        ok: false,
        name: call.name,
        args: call.args,
        summary: 'Args parse error: ' + call.parseError,
        detail: call.raw,
        error: { code: 'ARGS_PARSE_ERROR', message: call.parseError, retryable: false },
        elapsedMs: Date.now() - startedAt
      };
    }
    try {
      const result = await t.execute(call.args || {});
      const elapsedMs = Date.now() - startedAt;
      // 规范化 result：接受 string / {summary, detail} / 其他
      let summary, detail;
      if (typeof result === 'string') {
        summary = result;
        detail = '';
      } else if (result && typeof result === 'object') {
        summary = result.summary != null ? String(result.summary) : JSON.stringify(result);
        detail = result.detail != null ? String(result.detail) : '';
      } else {
        summary = String(result);
        detail = '';
      }
      // 截断超大结果
      const MAX = 32 * 1024;
      if (summary.length > MAX) summary = summary.slice(0, MAX) + '\n... (truncated)';
      if (detail.length > MAX) detail = detail.slice(0, MAX) + '\n... (truncated)';
      return { ok: true, name: call.name, args: call.args, summary, detail, elapsedMs };
    } catch (e) {
      return {
        ok: false,
        name: call.name,
        args: call.args,
        summary: 'Execution error: ' + e.message,
        detail: e.stack || '',
        error: { code: 'EXEC_ERROR', message: e.message, retryable: false },
        elapsedMs: Date.now() - startedAt
      };
    }
  }

  async function executeAll(calls) {
    return Promise.all((calls || []).map(executeOne));
  }

  // ============ 内置工具 ============

  // 安全的数学表达式求值（仅允许数字、运算符、函数名、括号）
  function safeCalc(expr) {
    if (typeof expr !== 'string') throw new Error('expression must be string');
    const cleaned = expr.replace(/\s+/g, '');
    if (!/^[\d+\-*/().,%MathPIEsqrtlogpowabcsincon]+$/.test(cleaned)) {
      throw new Error('Invalid characters in expression');
    }
    // eslint-disable-next-line no-new-func
    const fn = new Function('"use strict"; return (' + cleaned + ');');
    const result = fn();
    if (typeof result !== 'number' || !isFinite(result)) {
      throw new Error('Non-finite result');
    }
    return result;
  }

  register({
    id: 'builtin.get_current_time',
    name: 'get_current_time',
    provider: 'builtin',
    description: '获取当前时间（ISO 8601 格式）和 Unix 时间戳。',
    inputSchema: {
      type: 'object',
      properties: {
        timezone: { type: 'string', description: '可选时区，如 Asia/Shanghai' }
      }
    },
    execute: async (args) => {
      const now = new Date();
      const summary = args.timezone
        ? now.toLocaleString('zh-CN', { timeZone: args.timezone }) + ' (' + args.timezone + ')'
        : now.toISOString();
      return {
        summary,
        detail: 'ISO: ' + now.toISOString() + '\nUnix: ' + Math.floor(now.getTime() / 1000) + '\nUTC: ' + now.toUTCString()
      };
    }
  });

  register({
    id: 'builtin.calculate',
    name: 'calculate',
    provider: 'builtin',
    description: '执行基础数学运算。支持 + - * / % 括号，以及 Math.PI / Math.E / Math.sqrt / Math.log / Math.pow / Math.abs / Math.sin / Math.cos / Math.tan。',
    inputSchema: {
      type: 'object',
      properties: {
        expression: { type: 'string', description: '数学表达式字符串，如 (3+5)*2' }
      },
      required: ['expression']
    },
    execute: async (args) => {
      const r = safeCalc(args.expression);
      return { summary: String(r), detail: args.expression + ' = ' + r };
    }
  });

  register({
    id: 'builtin.echo',
    name: 'echo',
    provider: 'builtin',
    description: '回显传入的内容（用于测试工具调用流程）。',
    inputSchema: {
      type: 'object',
      properties: {
        message: { type: 'string', description: '要回显的内容' }
      },
      required: ['message']
    },
    execute: async (args) => ({ summary: args.message })
  });

  // 输出 / 暴露
  global.DS_TOOL_REGISTRY = {
    register,
    unregister,
    get,
    list,
    getToolsPromptSection,
    parseToolCalls,
    executeOne,
    executeAll
  };
})(typeof self !== 'undefined' ? self : this);