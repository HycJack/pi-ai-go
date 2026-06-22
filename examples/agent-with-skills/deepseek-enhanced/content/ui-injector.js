// Content Script - 在 isolated world 运行
(function () {
  const STORAGE_KEY = 'deepseek_enhanced_settings';

  const DEFAULT_PROMPT = `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  // 同步设置到主世界
  async function syncSettingsToMainWorld() {
    try {
      const result = await chrome.storage.local.get(STORAGE_KEY);
      const settings = result[STORAGE_KEY] || {};
      const prompt = settings.prompt || DEFAULT_PROMPT;
      const enabled = settings.enabled !== false;

      // 通过全局变量传递给主世界
      window.__deepseekEnhancedPrompt__ = prompt;
      window.__deepseekEnhancedEnabled__ = enabled;

      console.log('[DeepSeek Enhanced] Settings synced:', { enabled, promptLength: prompt.length });
    } catch (e) {
      console.error('[DeepSeek Enhanced] Sync error:', e);
    }
  }

  // 注入拦截器到主世界
  function injectInterceptorToMainWorld() {
    // 首先同步设置
    syncSettingsToMainWorld().then(() => {
      const script = document.createElement('script');
      script.src = chrome.runtime.getURL('content/fetch-interceptor.js');
      script.onload = function () {
        script.remove();
        console.log('[DeepSeek Enhanced] Interceptor injected into main world');
      };
      script.onerror = function (e) {
        console.error('[DeepSeek Enhanced] Failed to inject interceptor:', e);
      };

      // 注入到 document 开始处，确保在页面脚本之前
      (document.head || document.documentElement).prepend(script);
    });
  }

  // 监听设置变化
  chrome.storage.onChanged.addListener((changes, namespace) => {
    if (namespace !== 'local') return;
    if (changes[STORAGE_KEY]) {
      const settings = changes[STORAGE_KEY].newValue || {};
      window.__deepseekEnhancedPrompt__ = settings.prompt || DEFAULT_PROMPT;
      window.__deepseekEnhancedEnabled__ = settings.enabled !== false;
      console.log('[DeepSeek Enhanced] Settings updated via storage listener');
    }
  });

  // 在页面加载前注入
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', injectInterceptorToMainWorld);
  } else {
    injectInterceptorToMainWorld();
  }

  console.log('[DeepSeek Enhanced] UI Injector ready (isolated world)');
})();
