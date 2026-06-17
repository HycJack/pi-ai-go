/**
 * ui-injector.js (content script)
 * 职责:
 *   1. 将 fetch-interceptor.js 注入到页面主世界
 *   2. 监听来自主世界的工具调用消息
 *   3. 渲染工具执行结果到 DeepSeek 页面
 *   4. 可选:自动续跑(将工具结果发回 AI)
 */

// ==================== XML 工具调用解析(内联,避免跨模块 import) ====================

const TOOL_CALL_REGEX = /<tool_call>(\w+)(\{[\s\S]*?\})?<\/tool_call>/g;

function parseToolCalls(text) {
  const calls = [];
  const re = new RegExp(TOOL_CALL_REGEX.source, 'g');
  let match;
  while ((match = re.exec(text)) !== null) {
    const toolName = match[1];
    let params = {};
    if (match[2]) {
      try {
        params = JSON.parse(match[2]);
      } catch (e) {
        console.warn('[DeepSeek Enhanced] JSON parse error for tool:', toolName, e);
        params = { raw: match[2] };
      }
    }
    calls.push({ tool: toolName, params });
  }
  return calls;
}

// ==================== 注入主世界脚本 ====================

(function injectInterceptor() {
  const script = document.createElement('script');
  script.src = chrome.runtime.getURL('content/fetch-interceptor.js');
  script.onload = () => {
    script.remove();
    console.log('[DeepSeek Enhanced] Interceptor injected into main world');
  };
  (document.head || document.documentElement).appendChild(script);
})();

// ==================== 监听主世界消息 ====================

window.addEventListener('message', async (event) => {
  if (event.source !== window) return;
  if (!event.data) return;

  // 响应处理(工具调用)
  if (event.data.type === '__DEEPSEEK_ENHANCED_RESPONSE__') {
    await processResponse(event.data.content);
    return;
  }

  // prompt 增强请求(主世界 → isolated world → SW)
  if (event.data.type === '__DEEPSEEK_AUGMENT_REQUEST__') {
    await handleAugmentRequest(event.data);
    return;
  }
});

// ==================== 工具执行 + 渲染 ====================

let toolBlockContainer = null;

async function processResponse(aiContent) {
  const toolCalls = parseToolCalls(aiContent);
  if (toolCalls.length === 0) return;

  // 显示工具执行块
  const block = getOrCreateToolBlock();
  const statusEl = block.querySelector('.deepseek-enhanced-status');
  const detailEl = block.querySelector('.deepseek-enhanced-detail');

  block.style.display = 'block';
  statusEl.textContent = `🔧 正在执行 ${toolCalls.length} 个工具...`;
  detailEl.innerHTML = '';

  // 逐个执行工具
  const results = [];
  for (const call of toolCalls) {
    try {
      const result = await sendToSW({
        type: 'executeTool',
        tool: call.tool,
        params: call.params,
      });
      results.push({ call, result });
      detailEl.innerHTML += renderToolResult(call, result);
    } catch (e) {
      results.push({ call, result: { error: e.message } });
      detailEl.innerHTML += renderToolResult(call, { error: e.message });
    }
  }

  const successCount = results.filter(r => r.result && !r.result.error).length;
  statusEl.textContent = `🔧 已执行 ${successCount}/${toolCalls.length} 个工具`;

  // 清除隐藏状态
  block.classList.add('done');
}

/**
 * 创建或获取工具执行块容器
 */
function getOrCreateToolBlock() {
  if (toolBlockContainer && document.body.contains(toolBlockContainer)) {
    return toolBlockContainer;
  }

  const container = document.createElement('div');
  container.className = 'deepseek-enhanced-tool-block';
  container.style.cssText = `
    position: fixed;
    bottom: 80px;
    right: 20px;
    width: 360px;
    max-height: 300px;
    overflow-y: auto;
    background: #ffffff;
    border: 1px solid #e2e8f0;
    border-radius: 12px;
    box-shadow: 0 4px 24px rgba(0,0,0,0.12);
    padding: 16px;
    z-index: 99999;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 13px;
    display: none;
  `;
  container.innerHTML = `
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;">
      <span class="deepseek-enhanced-status" style="font-weight:600;color:#1e293b;"></span>
      <div>
        <button class="deepseek-enhanced-expand" style="background:none;border:none;cursor:pointer;color:#64748b;font-size:14px;padding:2px 6px;" title="展开/折叠">▼</button>
        <button class="deepseek-enhanced-close" style="background:none;border:none;cursor:pointer;color:#64748b;font-size:16px;padding:2px 6px;" title="关闭">×</button>
      </div>
    </div>
    <div class="deepseek-enhanced-detail" style="border-top:1px solid #f1f5f9;padding-top:8px;"></div>
  `;

  // 事件绑定
  container.querySelector('.deepseek-enhanced-close').addEventListener('click', () => {
    container.style.display = 'none';
  });

  const expandBtn = container.querySelector('.deepseek-enhanced-expand');
  const detailEl = container.querySelector('.deepseek-enhanced-detail');
  expandBtn.addEventListener('click', () => {
    const hidden = detailEl.style.display === 'none';
    detailEl.style.display = hidden ? 'block' : 'none';
    expandBtn.textContent = hidden ? '▼' : '▶';
  });

  document.body.appendChild(container);
  toolBlockContainer = container;
  return container;
}

/**
 * 渲染单个工具结果
 */
function renderToolResult(call, result) {
  const success = result && !result.error;
  return `
    <div style="margin-bottom:8px;padding:8px;background:${success ? '#f0fdf4' : '#fef2f2'};border-radius:6px;border-left:3px solid ${success ? '#22c55e' : '#ef4444'};">
      <div style="font-weight:600;margin-bottom:4px;color:${success ? '#166534' : '#991b1b'};">
        ${success ? '✅' : '❌'} ${call.tool}
      </div>
      <div style="color:#475569;font-size:12px;word-break:break-all;">
        ${escapeHtml(JSON.stringify(result, null, 0).substring(0, 200))}
      </div>
    </div>
  `;
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

/**
 * 向 Service Worker 发送消息
 */
function sendToSW(message) {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendMessage(message, (response) => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message));
      } else {
        resolve(response);
      }
    });
  });
}

/**
 * 处理来自主世界的 prompt 增强请求,转发给 SW
 */
async function handleAugmentRequest({ requestId, body }) {
  try {
    const enhancedBody = await sendToSW({ type: 'augmentRequest', body });
    window.postMessage(
      { type: '__DEEPSEEK_AUGMENT_RESULT__', requestId, body: enhancedBody },
      '*'
    );
  } catch (e) {
    // 失败时返回原始 body
    window.postMessage(
      { type: '__DEEPSEEK_AUGMENT_RESULT__', requestId, body },
      '*'
    );
  }
}

console.log('[DeepSeek Enhanced] UI Injector ready');
