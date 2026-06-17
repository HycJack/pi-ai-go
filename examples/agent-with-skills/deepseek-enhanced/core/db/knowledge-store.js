/**
 * 知识库 Store - 知识库条目和分类的 CRUD
 */

import { getAll, getById, put, remove, getByIndex } from './indexeddb.js';
import { STORE_NAMES } from '../constants.js';

const ITEMS = STORE_NAMES.KNOWLEDGE_ITEMS;
const CATEGORIES = STORE_NAMES.KNOWLEDGE_CATEGORIES;

export const KnowledgeStore = {
  // ---- 条目 ----

  async allItems() {
    return await getAll(ITEMS);
  },

  async getItem(id) {
    return await getById(ITEMS, id);
  },

  async getItemsByCategory(categoryId) {
    return await getByIndex(ITEMS, 'category', categoryId);
  },

  async saveItem(item) {
    const now = Date.now();
    const data = {
      title: '',
      content: '',
      category: 'default',
      tags: [],
      source: 'manual',
      ...item,
      createdAt: item.createdAt || now,
      updatedAt: now,
    };
    const id = await put(ITEMS, data);
    return { ...data, id };
  },

  async updateItem(id, updates) {
    const existing = await getById(ITEMS, id);
    if (!existing) throw new Error(`Knowledge item not found: ${id}`);
    const merged = { ...existing, ...updates, id, updatedAt: Date.now() };
    await put(ITEMS, merged);
    return merged;
  },

  async deleteItem(id) {
    return await remove(ITEMS, id);
  },

  /**
   * 搜索知识库:按标题, 内容, 标签匹配
   */
  async searchItems(query, limit = 5) {
    const all = await getAll(ITEMS);
    const q = query.toLowerCase();
    const results = [];
    for (const item of all) {
      let score = 0;
      if (item.title.toLowerCase().includes(q)) score += 10;
      if (item.content.toLowerCase().includes(q)) score += 1;
      if (item.tags.some(t => t.toLowerCase().includes(q))) score += 5;
      if (score > 0) results.push({ ...item, _score: score });
    }
    results.sort((a, b) => b._score - a._score);
    return results.slice(0, limit);
  },

  // ---- 分类 ----

  async allCategories() {
    return await getAll(CATEGORIES);
  },

  async saveCategory(cat) {
    const item = {
      id: cat.id || `cat_${Date.now()}`,
      name: cat.name || '未分类',
      description: cat.description || '',
      color: cat.color || '#6366f1',
      createdAt: cat.createdAt || Date.now(),
    };
    await put(CATEGORIES, item);
    return item;
  },

  async deleteCategory(id) {
    // 同时删除该分类下的所有条目
    const items = await getByIndex(ITEMS, 'category', id);
    for (const item of items) {
      await remove(ITEMS, item.id);
    }
    return await remove(CATEGORIES, id);
  },
};
