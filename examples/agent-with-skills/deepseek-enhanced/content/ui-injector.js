(function () {
  const STORAGE_KEY = 'deepseek_enhanced_settings';

  const DEFAULT_PROMPT = `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  // 立即设置默认值（防止 undefined）
  if (typeof window.__deepseekEnhancedPrompt__ === 'undefined') {
    window.__deepseekEnhancedPrompt__ = DEFAULT_PROMPT;
  }
  if (typeof window.__deepseekEnhancedEnabled__ === 'undefined') {
    window.__deepseekEnhancedEnabled__ = true;
  }

  function injectInterceptorToMainWorld() {
    const script = document.createElement('script');
    script.src = chrome.runtime.getURL('content/fetch-interceptor.js');
    script.onload = function () {
      script.remove();
    };

    (document.head || document.documentElement).prepend(script);
  }

  // 先注入拦截器（使用默认值），然后异步更新设置
  function syncSettingsAndInject() {
    // 先注入
    injectInterceptorToMainWorld();

    // 再异步加载 storage 中的设置并更新全局变量
    chrome.storage.local.get(STORAGE_KEY, (result) => {
      const settings = result[STORAGE_KEY] || {};
      const prompt = settings.prompt || DEFAULT_PROMPT;
      const enabled = settings.enabled !== false;
      window.__deepseekEnhancedPrompt__ = prompt;
      window.__deepseekEnhancedEnabled__ = enabled;
    });
  }

  // 监听设置变化
  chrome.storage.onChanged.addListener((changes, namespace) => {
    if (namespace !== 'local') return;
    if (changes[STORAGE_KEY]) {
      const settings = changes[STORAGE_KEY].newValue || {};
      window.__deepseekEnhancedPrompt__ = settings.prompt || DEFAULT_PROMPT;
      window.__deepseekEnhancedEnabled__ = settings.enabled !== false;
    }
  });

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', syncSettingsAndInject);
  } else {
    syncSettingsAndInject();
  }
})();