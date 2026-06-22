(function injectInterceptor() {
  const script = document.createElement('script');
  script.src = chrome.runtime.getURL('content/fetch-interceptor.js');
  script.onload = () => {
    script.remove();
    console.log('[DeepSeek Enhanced] Interceptor injected into main world');
  };
  (document.head || document.documentElement).appendChild(script);
})();

const STORAGE_KEY = 'deepseek_enhanced_settings';

async function getSettings() {
  try {
    const result = await chrome.storage.local.get(STORAGE_KEY);
    return result[STORAGE_KEY] || {};
  } catch (e) {
    console.error('[DeepSeek Enhanced] Get settings error:', e);
    return {};
  }
}

window.addEventListener('message', async (event) => {
  if (event.source !== window) return;
  if (!event.data) return;

  if (event.data.type === '__DEEPSEEK_GET_SETTINGS__') {
    const settings = await getSettings();
    window.postMessage(
      { type: '__DEEPSEEK_SETTINGS_RESULT__', requestId: event.data.requestId, settings },
      '*'
    );
    return;
  }
});

chrome.storage.onChanged.addListener((changes, namespace) => {
  if (namespace !== 'local') return;
  if (changes[STORAGE_KEY]) {
    const settings = changes[STORAGE_KEY].newValue || {};
    window.postMessage(
      { type: '__DEEPSEEK_SETTINGS_UPDATED__', ...settings },
      '*'
    );
  }
});

console.log('[DeepSeek Enhanced] UI Injector ready');
