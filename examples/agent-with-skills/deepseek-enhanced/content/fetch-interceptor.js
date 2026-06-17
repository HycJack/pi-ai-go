/**
 * fetch-interceptor.js
 * 注入到页面主世界,拦截 DeepSeek 网页的 fetch 请求
 * 通过 window.postMessage 与 ui-injector(isolated world)通信
 */

(async function () {
  if (window.__deepseekEnhancedInjected) return;
  window.__deepseekEnhancedInjected = true;

  const originalFetch = window.fetch;

  window.fetch = async function (url, options = {}) {
    const urlStr = typeof url === 'string' ? url : (url.href || url.url || '');
    const bodyStr = typeof options.body === 'string' ? options.body : '';

    // ========== 拦截请求:增强 prompt(通过 postMessage 委托给 isolated world)==========
    let enhanced = false;
    if (urlStr.includes('/chat/completions') && bodyStr) {
      try {
        const body = JSON.parse(bodyStr);
        if (body.messages) {
          const enhancedBody = await augmentViaPostMessage(body);
          if (enhancedBody) {
            options.body = JSON.stringify(enhancedBody);
            enhanced = true;
          }
        }
      } catch (e) {
        console.warn('[DeepSeek Enhanced] intercept error:', e);
      }
    }

    // ========== 发送请求 ==========
    const response = await originalFetch.call(this, url, options);

    // ========== 拦截响应:解析工具调用 ==========
    if (urlStr.includes('/chat/completions') && enhanced) {
      try {
        const cloned = response.clone();
        processResponse(cloned).catch(e =>
          console.warn('[DeepSeek Enhanced] response process error:', e)
        );
      } catch (e) {
        // clone 失败则忽略
      }
    }

    return response;
  };

  /**
   * 通过 postMessage 委托 ui-injector(isolated world)调用 SW 进行 prompt 增强
   */
  function augmentViaPostMessage(body) {
    return new Promise((resolve) => {
      const requestId = `augment_${Date.now()}_${Math.random().toString(36).slice(2)}`;

      const handler = (event) => {
        if (event.source !== window) return;
        if (!event.data || event.data.type !== '__DEEPSEEK_AUGMENT_RESULT__') return;
        if (event.data.requestId !== requestId) return;

        window.removeEventListener('message', handler);
        resolve(event.data.body || null);
      };

      window.addEventListener('message', handler);

      window.postMessage(
        { type: '__DEEPSEEK_AUGMENT_REQUEST__', requestId, body },
        '*'
      );

      // 超时兜底
      setTimeout(() => {
        window.removeEventListener('message', handler);
        resolve(null);
      }, 3000);
    });
  }

  /**
   * 处理响应:解析工具调用并通知 UI 层
   */
  async function processResponse(response) {
    try {
      const data = await response.json();
      const choices = data.choices || [];
      for (const choice of choices) {
        const content = choice.message?.content || '';
        if (!content) continue;

        window.postMessage(
          {
            type: '__DEEPSEEK_ENHANCED_RESPONSE__',
            content,
            choiceIndex: choice.index,
          },
          '*'
        );
      }
    } catch (e) {
      // 忽略 JSON 解析错误
    }
  }

  console.log('[DeepSeek Enhanced] Fetch interceptor injected');
})();
