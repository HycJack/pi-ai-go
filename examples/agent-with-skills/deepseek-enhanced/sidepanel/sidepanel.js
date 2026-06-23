(function () {
  const STORAGE_KEY = 'deepseek_enhanced_settings';

  const DEFAULT_PROMPT = `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

  const promptInput = document.getElementById('prompt-input');
  const saveBtn = document.getElementById('save-btn');
  const resetBtn = document.getElementById('reset-btn');
  const statusEl = document.getElementById('status');

  async function loadSettings() {
    try {
      const result = await chrome.storage.local.get(STORAGE_KEY);
      const settings = result[STORAGE_KEY] || {};
      promptInput.value = settings.prompt || DEFAULT_PROMPT;
    } catch (e) {
      promptInput.value = DEFAULT_PROMPT;
    }
  }

  async function saveSettings() {
    const prompt = promptInput.value.trim();
    try {
      await chrome.storage.local.set({
        [STORAGE_KEY]: {
          prompt: prompt || DEFAULT_PROMPT,
          enabled: true
        }
      });
      statusEl.textContent = '设置已保存';
      setTimeout(() => {
        statusEl.textContent = '';
      }, 2000);
    } catch (e) {
      statusEl.textContent = '保存失败';
    }
  }

  async function resetSettings() {
    try {
      await chrome.storage.local.set({
        [STORAGE_KEY]: {
          prompt: DEFAULT_PROMPT,
          enabled: true
        }
      });
      promptInput.value = DEFAULT_PROMPT;
      statusEl.textContent = '已恢复默认';
      setTimeout(() => {
        statusEl.textContent = '';
      }, 2000);
    } catch (e) {
      statusEl.textContent = '重置失败';
    }
  }

  saveBtn.addEventListener('click', saveSettings);
  resetBtn.addEventListener('click', resetSettings);

  loadSettings();
})();
