// Background Service Worker
// 职责:消息路由 + 工具执行 + prompt 增强

import { executeTool } from '../core/tools/index.js';
import { MemoryStore } from '../core/db/memory-store.js';
import { SkillStore } from '../core/db/skill-store.js';
import { KnowledgeStore } from '../core/db/knowledge-store.js';
import { augmentRequest } from '../core/prompt/augmentation.js';
import { getAllSkills } from '../core/skill/registry.js';

// ==================== 消息处理 ====================

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  handleMessage(message, sender).then(sendResponse).catch(err => {
    console.error('[DeepSeek Enhanced] SW error:', err);
    sendResponse({ error: err.message });
  });
  return true; // 保持消息通道开放(异步响应)
});

async function handleMessage(message, sender) {
  switch (message.type) {
    // --- Prompt 增强(来自 fetch-interceptor) ---
    case 'augmentRequest':
      return await augmentRequest(message.body);

    // --- 工具执行 ---
    case 'executeTool':
      return await executeTool(message.tool, message.params);

    // --- 记忆 CRUD ---
    case 'getMemories':
      return await MemoryStore.getAll();
    case 'saveMemory':
      return await MemoryStore.save(message.memory);
    case 'updateMemory':
      return await MemoryStore.update(message.id, message.memory);
    case 'deleteMemory':
      return await MemoryStore.delete(message.id);

    // --- Skill CRUD ---
    case 'getSkills':
      return await getAllSkills();
    case 'saveSkill':
      return await SkillStore.save(message.skill);
    case 'deleteSkill':
      return await SkillStore.delete(message.id);

    // --- 知识库 CRUD ---
    case 'getKnowledgeItems':
      return await KnowledgeStore.allItems();
    case 'saveKnowledgeItem':
      return await KnowledgeStore.saveItem(message.item);
    case 'updateKnowledgeItem':
      return await KnowledgeStore.updateItem(message.id, message.item);
    case 'deleteKnowledgeItem':
      return await KnowledgeStore.deleteItem(message.id);
    case 'getKnowledgeCategories':
      return await KnowledgeStore.allCategories();
    case 'saveKnowledgeCategory':
      return await KnowledgeStore.saveCategory(message.category);
    case 'deleteKnowledgeCategory':
      return await KnowledgeStore.deleteCategory(message.id);

    // --- 搜索 ---
    case 'searchKnowledge':
      return await KnowledgeStore.searchItems(message.query, message.limit || 5);

    // --- 设置 ---
    case 'getSettings':
      return await chrome.storage.local.get(message.keys || null);
    case 'saveSettings':
      await chrome.storage.local.set(message.settings);
      return { success: true };

    default:
      throw new Error(`Unknown message type: ${message.type}`);
  }
}

// ==================== 点击图标打开侧边栏 ====================

chrome.action.onClicked.addListener((tab) => {
  chrome.sidePanel.open({ windowId: tab.windowId });
});

console.log('[DeepSeek Enhanced] Service Worker ready');
