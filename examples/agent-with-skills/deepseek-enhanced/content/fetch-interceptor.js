(function () {
  if (window.__deepseekEnhancedInjected__) return;

  const SYSTEM_PROMPT = window.__deepseekEnhancedPrompt__ || `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  const ENABLED = window.__deepseekEnhancedEnabled__ !== false;

  const originalFetch = window.fetch;

  window.fetch = async function (url, options) {
    options = options || {};

    const urlStr = typeof url === 'string' ? url : (url instanceof URL ? url.href : (url.url || ''));

    if (urlStr.includes('lanz.hikvision.com') && urlStr.includes('/v1/chat/completions')) {
      const bodyStr = typeof options.body === 'string' ? options.body : '';

      if (bodyStr && ENABLED) {
        try {
          const body = JSON.parse(bodyStr);

          if (body.prompt && typeof body.prompt === 'string') {
            body.prompt = SYSTEM_PROMPT + '\n\n---\n\n' + body.prompt;
            options.body = JSON.stringify(body);
            return originalFetch.call(this, url, options);
          }

          if (body.messages && Array.isArray(body.messages)) {
            const systemIdx = body.messages.findIndex(m => m.role === 'system');
            if (systemIdx === -1) {
              body.messages.unshift({
                role: 'system',
                content: SYSTEM_PROMPT
              });
            } else {
              body.messages[systemIdx].content = SYSTEM_PROMPT + '\n\n---\n\n' + body.messages[systemIdx].content;
            }
            options.body = JSON.stringify(body);
            return originalFetch.call(this, url, options);
          }
        } catch (e) {
        }
      }
    }

    return originalFetch.call(this, url, options);
  };

  window.__deepseekEnhancedInjected__ = true;
})();
