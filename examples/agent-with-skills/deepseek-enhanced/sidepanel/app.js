/**
 * sidepanel/app.js
 * 侧边栏主入口:Tab 导航 + 页面加载 + 设置面板
 */

import MemoriesPage from './pages/memories.js';
import SkillsPage from './pages/skills.js';
import KnowledgePage from './pages/knowledge.js';
import SettingsPanel from './pages/settings.js';

// ==================== 状态 ====================

const state = {
  currentTab: 'memories',
  pages: {},
};

// ==================== 初始化 ====================

document.addEventListener('DOMContentLoaded', () => {
  initTabs();
  initSettingsBtn();
  loadPage('memories');
});

// ==================== Tab 导航 ====================

function initTabs() {
  const tabs = document.querySelectorAll('.tab');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      const tabName = tab.dataset.tab;
      if (state.currentTab === tabName) return;

      // 更新活动状态
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      state.currentTab = tabName;
      loadPage(tabName);
    });
  });
}

// ==================== 页面加载 ====================

async function loadPage(tabName) {
  const container = document.getElementById('pageContainer');
  const settingsPanel = document.getElementById('settingsPanel');

  // 隐藏设置面板
  settingsPanel.style.display = 'none';

  // 缓存页面实例
  if (!state.pages[tabName]) {
    state.pages[tabName] = createPage(tabName);
  }

  const page = state.pages[tabName];
  container.innerHTML = '';
  container.appendChild(page.container);

  // 触发页面的初始化/刷新
  if (page.refresh) {
    await page.refresh();
  }
}

function createPage(tabName) {
  switch (tabName) {
    case 'memories':
      return MemoriesPage();
    case 'skills':
      return SkillsPage();
    case 'knowledge':
      return KnowledgePage();
    default:
      return { container: document.createElement('div') };
  }
}

// ==================== 设置按钮 ====================

function initSettingsBtn() {
  const btn = document.getElementById('settingsBtn');
  btn.addEventListener('click', () => {
    const container = document.getElementById('pageContainer');
    const settingsPanel = document.getElementById('settingsPanel');

    if (settingsPanel.style.display === 'none') {
      // 显示设置
      container.innerHTML = '';
      settingsPanel.style.display = 'block';
      SettingsPanel.render(settingsPanel);
    } else {
      // 隐藏设置,回到当前 Tab
      settingsPanel.style.display = 'none';
      loadPage(state.currentTab);
    }
  });
}

// ==================== SW 通信工具 ====================

/**
 * 向 Service Worker 发送消息
 */
export function sendToSW(message) {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendMessage(message, (response) => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message));
      } else if (response && response.error && typeof response.error === 'string') {
        reject(new Error(response.error));
      } else {
        resolve(response);
      }
    });
  });
}

/**
 * 格式化日期
 */
export function formatDate(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  const hours = String(d.getHours()).padStart(2, '0');
  const min = String(d.getMinutes()).padStart(2, '0');
  return `${month}-${day} ${hours}:${min}`;
}

/**
 * HTML 转义
 */
export function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

/**
 * 创建 DOM 元素
 */
export function createEl(tag, attrs = {}, children = []) {
  const el = document.createElement(tag);
  for (const [key, val] of Object.entries(attrs)) {
    if (key === 'className') {
      el.className = val;
    } else if (key === 'textContent') {
      el.textContent = val;
    } else if (key === 'innerHTML') {
      el.innerHTML = val;
    } else if (key.startsWith('on')) {
      el.addEventListener(key.slice(2).toLowerCase(), val);
    } else if (key === 'style' && typeof val === 'object') {
      Object.assign(el.style, val);
    } else {
      el.setAttribute(key, val);
    }
  }
  for (const child of children) {
    if (typeof child === 'string') {
      el.appendChild(document.createTextNode(child));
    } else if (child instanceof Node) {
      el.appendChild(child);
    }
  }
  return el;
}
