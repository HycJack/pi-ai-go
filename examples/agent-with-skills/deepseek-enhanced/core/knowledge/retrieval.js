/**
 * 知识检索
 * 基于标签 + 关键词匹配
 */

import { KnowledgeStore } from '../db/knowledge-store.js';

/**
 * 搜索相关知识库条目
 * @param {string} userInput
 * @param {number} limit
 * @returns {Promise<Array>}
 */
export async function searchKnowledge(userInput, limit = 3) {
  const results = await KnowledgeStore.searchItems(userInput, limit);
  return results;
}
