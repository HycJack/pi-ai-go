const DEFAULT_PROMPT = `你是一位专业的 AI 助手。请始终保持以下行为准则：

1. 回答要专业、准确、简洁
2. 使用中文回答用户的问题
3. 遇到不确定的问题时，要明确说明
4. 优先提供可行的解决方案

你具有长期记忆能力，可以记住用户的身份、偏好和历史对话中的关键信息。`;

const STORAGE_KEY = 'deepseek_enhanced_settings';

document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings();

  document.getElementById('save-btn').addEventListener('click', saveSettings);
  document.getElementById('reset-btn').addEventListener('click', resetSettings);
});

async function loadSettings() {
  try {
    const result = await chrome.storage.local.get(STORAGE_KEY);
    const settings = result[STORAGE_KEY] || {};
    
    document.getElementById('system-prompt').value = settings.prompt || DEFAULT_PROMPT;
    document.getElementById('enabled').checked = settings.enabled !== false;
  } catch (e) {
    console.error('Load settings error:', e);
    showStatus('加载失败', 'error');
  }
}

async function saveSettings() {
  const prompt = document.getElementById('system-prompt').value;
  const enabled = document.getElementById('enabled').checked;

  try {
    await chrome.storage.local.set({
      [STORAGE_KEY]: { prompt, enabled }
    });
    showStatus('设置已保存', 'success');
  } catch (e) {
    console.error('Save settings error:', e);
    showStatus('保存失败', 'error');
  }
}

async function resetSettings() {
  if (!confirm('确定要恢复默认设置吗？')) return;

  try {
    await chrome.storage.local.remove(STORAGE_KEY);
    document.getElementById('system-prompt').value = DEFAULT_PROMPT;
    document.getElementById('enabled').checked = true;
    showStatus('已恢复默认', 'success');
  } catch (e) {
    console.error('Reset settings error:', e);
    showStatus('恢复失败', 'error');
  }
}

function showStatus(message, type) {
  const status = document.getElementById('status');
  status.textContent = message;
  status.className = `status show ${type}`;
  setTimeout(() => {
    status.classList.remove('show');
  }, 2000);
}
