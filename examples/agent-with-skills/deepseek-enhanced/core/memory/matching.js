/**
 * 记忆匹配 - 根据用户输入匹配相关记忆
 */

import { MemoryStore } from '../db/memory-store.js';

/**
 * 从用户 prompt 中提取关键词
 */
function extractKeywords(text, maxWords = 5) {
  const cleaned = text.toLowerCase().replace(/[^\u4e00-\u9fa5a-zA-Z0-9]/g, ' ');
  const words = cleaned.split(/\s+/).filter(w => w.length > 1);
  // 简单的频率统计
  const freq = {};
  for (const w of words) {
    freq[w] = (freq[w] || 0) + 1;
  }
  return Object.entries(freq)
    .sort((a, b) => b[1] - a[1])
    .slice(0, maxWords)
    .map(e => e[0]);
}

/**
 * 匹配相关记忆
 * @param {string} userInput - 用户当前输入
 * @param {number} limit - 返回数量上限
 * @returns {Promise<Array>}
 */
export async function matchMemories(userInput, limit = 5) {
  if (!userInput || userInput.trim().length === 0) return [];
  
  const all = await MemoryStore.all();
  if (all.length === 0) return [];
  
  const keywords = extractKeywords(userInput);
  if (keywords.length === 0) {
    // 没有关键词,返回置顶记忆
    return await MemoryStore.getPinned();
  }
  
  const scored = [];
  for (const mem of all) {
    let score = 0;
    
    // 置顶加分
    if (mem.pinned) score += 20;
    
    // 名称匹配
    const nameLower = (mem.name || '').toLowerCase();
    for (const kw of keywords) {
      if (nameLower.includes(kw)) score += 10;
    }
    
    // 内容匹配
    const contentLower = (mem.content || '').toLowerCase();
    for (const kw of keywords) {
      if (contentLower.includes(kw)) score += 3;
    }
    
    // 标签匹配
    const tags = mem.tags || [];
    for (const kw of keywords) {
      if (tags.some(t => t.toLowerCase().includes(kw))) score += 8;
    }
    
    if (score > 0) {
      scored.push({ ...mem, _score: score });
    }
  }
  
  scored.sort((a, b) => b._score - a._score);
  return scored.slice(0, limit);
}
