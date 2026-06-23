(function () {
  if (window.__deepseekEnhancedInjected__) return;

  const DEFAULT_PROMPT = `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  const SYSTEM_PROMPT = window.__deepseekEnhancedPrompt__ || DEFAULT_PROMPT;
  const ENABLED = window.__deepseekEnhancedEnabled__ !== false;

  const originalFetch = window.fetch;

  function getCurrentPrompt() {
    return window.__deepseekEnhancedPrompt__ || DEFAULT_PROMPT;
  }

  function getEnabled() {
    return window.__deepseekEnhancedEnabled__ !== false;
  }

  function injectPrompt(body) {
    if (body.message && typeof body.message === 'string') {
      try {
        const messages = JSON.parse(body.message);
        if (Array.isArray(messages)) {
          const systemIdx = messages.findIndex(m => m.role === 'system');
          const prompt = getCurrentPrompt();
          if (systemIdx === -1) {
            messages.unshift({
              role: 'system',
              content: prompt
            });
          } else {
            messages[systemIdx].content = prompt + '\n\n---\n\n' + messages[systemIdx].content;
          }
          body.message = JSON.stringify(messages);
          return true;
        }
      } catch (e) {
      }
    }
    return false;
  }

  try {
    window.fetch = async function (url, options) {
      options = options || {};

      const urlStr = typeof url === 'string' ? url : (url instanceof URL ? url.href : (url.url || ''));

      if (urlStr.includes('lanz.hikvision.com') && urlStr.includes('/v1/chat/completions')) {
        const bodyStr = typeof options.body === 'string' ? options.body : '';

        if (bodyStr && getEnabled()) {
          try {
            const body = JSON.parse(bodyStr);
            if (injectPrompt(body)) {
              options.body = JSON.stringify(body);
            }
          } catch (e) {
          }
        }
      }

      return originalFetch.call(this, url, options);
    };
  } catch (e) {
  }

  // 主动从 chrome.storage 读取最新设置
  if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
    try {
      chrome.storage.local.get('deepseek_enhanced_settings', (result) => {
        const settings = result['deepseek_enhanced_settings'] || {};
        if (typeof settings.prompt === 'string' && settings.prompt) {
          window.__deepseekEnhancedPrompt__ = settings.prompt;
        }
        if (typeof settings.enabled === 'boolean') {
          window.__deepseekEnhancedEnabled__ = settings.enabled;
        }
      });
    } catch (e) {
    }
  }

  window.__deepseekEnhancedInjected__ = true;
})();