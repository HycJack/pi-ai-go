(async function () {
  if (window.__deepseekEnhancedInjected) return;
  window.__deepseekEnhancedInjected = true;

  const originalFetch = window.fetch;

  const SYSTEM_PROMPT = `你叫小齐，是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  let currentSettings = {
    prompt: DEFAULT_PROMPT,
    enabled: true
  };

  function requestSettings() {
    return new Promise((resolve) => {
      const requestId = `settings_${Date.now()}`;

      const handler = (event) => {
        if (event.source !== window) return;
        if (!event.data || event.data.type !== '__DEEPSEEK_SETTINGS_RESULT__') return;
        if (event.data.requestId !== requestId) return;

        window.removeEventListener('message', handler);
        resolve(event.data.settings || {});
      };

      window.addEventListener('message', handler);

      window.postMessage(
        { type: '__DEEPSEEK_GET_SETTINGS__', requestId },
        '*'
      );

      setTimeout(() => {
        window.removeEventListener('message', handler);
        resolve({});
      }, 2000);
    });
  }

  const settings = await requestSettings();
  if (settings.prompt !== undefined) currentSettings.prompt = settings.prompt;
  if (settings.enabled !== undefined) currentSettings.enabled = settings.enabled;

  window.fetch = async function (url, options = {}) {
    const urlStr = typeof url === 'string' ? url : (url.href || url.url || '');
    const bodyStr = typeof options.body === 'string' ? options.body : '';

    if (!currentSettings.enabled) {
      return originalFetch.call(this, url, options);
    }

    if (urlStr.includes('/api/v0/chat/completion') && bodyStr) {
      try {
        const body = JSON.parse(bodyStr);
        if (body.prompt) {
          body.prompt = currentSettings.prompt + '\n\n---\n\n' + body.prompt;
          options.body = JSON.stringify(body);
        }
      } catch (e) {
        console.warn('[DeepSeek Enhanced] intercept error:', e);
      }
    }

    return originalFetch.call(this, url, options);
  };

  window.addEventListener('message', (event) => {
    if (event.source !== window) return;
    if (!event.data || event.data.type !== '__DEEPSEEK_SETTINGS_UPDATED__') return;

    const { prompt, enabled } = event.data;
    if (prompt !== undefined) currentSettings.prompt = prompt;
    if (enabled !== undefined) currentSettings.enabled = enabled;
    console.log('[DeepSeek Enhanced] Settings updated');
  });

  console.log('[DeepSeek Enhanced] Fetch interceptor injected');
})();
