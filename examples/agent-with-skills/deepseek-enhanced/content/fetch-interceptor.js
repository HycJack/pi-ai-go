// 直接在主世界运行的拦截器
// 注意：这个文件需要被标记为 web_accessible_resources 并通过 ui-injector 注入到主世界

(function () {
  // 防止重复注入
  if (window.__deepseekEnhancedInjected__) return;

  const SYSTEM_PROMPT = window.__deepseekEnhancedPrompt__ || `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  const ENABLED = window.__deepseekEnhancedEnabled__ !== false;

  // 保存原始 fetch
  const originalFetch = window.fetch;

  // 重写 fetch
  window.fetch = async function (url, options) {
    options = options || {};

    const urlStr = typeof url === 'string' ? url : (url instanceof URL ? url.href : (url.url || ''));

    // 匹配目标 API 路径
    if (urlStr.includes('/api/v0/chat/completion') || urlStr.includes('/api/v0/chat/regenerate')) {
      const bodyStr = typeof options.body === 'string' ? options.body : '';

      if (bodyStr && ENABLED) {
        try {
          const body = JSON.parse(bodyStr);

          // 方式1: body.prompt 格式
          if (body.prompt && typeof body.prompt === 'string') {
            body.prompt = SYSTEM_PROMPT + '\n\n---\n\n' + body.prompt;
            options.body = JSON.stringify(body);
            console.log('[DeepSeek Enhanced] ✅ Prompt injected via body.prompt');
            return originalFetch.call(this, url, options);
          }

          // 方式2: body.messages 格式
          if (body.messages && Array.isArray(body.messages)) {
            // 查找是否有 system 消息
            const systemIdx = body.messages.findIndex(m => m.role === 'system');
            if (systemIdx === -1) {
              // 没有 system 消息，添加到开头
              body.messages.unshift({
                role: 'system',
                content: SYSTEM_PROMPT
              });
            } else {
              // 有 system 消息，修改内容
              body.messages[systemIdx].content = SYSTEM_PROMPT + '\n\n---\n\n' + body.messages[systemIdx].content;
            }
            options.body = JSON.stringify(body);
            console.log('[DeepSeek Enhanced] ✅ Prompt injected via body.messages');
            return originalFetch.call(this, url, options);
          }

          console.log('[DeepSeek Enhanced] ⚠️ Unknown request format:', Object.keys(body));

        } catch (e) {
          console.error('[DeepSeek Enhanced] ❌ Parse error:', e);
        }
      }
    }

    return originalFetch.call(this, url, options);
  };

  window.__deepseekEnhancedInjected__ = true;
  console.log('[DeepSeek Enhanced] 🚀 Interceptor ready');
})();
