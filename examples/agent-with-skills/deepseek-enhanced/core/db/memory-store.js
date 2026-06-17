/**
 * 记忆 Store - 记忆数据的 CRUD
 */

import { getAll, getById, put, remove, getByIndex } from './indexeddb.js';
import { STORE_NAMES, MEMORY_TYPES } from '../constants.js';

const STORE = STORE_NAMES.MEMORIES;

export const MemoryStore = {
  /**
   * 获取所有记忆
   */
  async all() {
    return await getAll(STORE);
  },

  /**
   * 获取单条记忆
   */
  async get(id) {
    return await getById(STORE, id);
  },

  /**
   * 按类型获取记忆
   */
  async getByType(type) {
    return await getByIndex(STORE, 'type', type);
  },

  /**
   * 获取置顶记忆
   */
  async getPinned() {
    return await getByIndex(STORE, 'pinned', true);
  },

  /**
   * 保存记忆(新增或更新)
   */
  async save(memory) {
    const now = Date.now();
    const item = {
      type: MEMORY_TYPES.USER,
      name: '',
      content: '',
      tags: [],
      pinned: false,
      ...memory,
      createdAt: memory.createdAt || now,
      updatedAt: now,
    };
    const id = await put(STORE, item);
    return { ...item, id };
  },

  /**
   * 更新记忆
   */
  async update(id, updates) {
    const existing = await getById(STORE, id);
    if (!existing) throw new Error(`Memory not found: ${id}`);
    const merged = {
      ...existing,
      ...updates,
      id,
      updatedAt: Date.now(),
    };
    await put(STORE, merged);
    return merged;
  },

  /**
   * 删除记忆
   */
  async delete(id) {
    return await remove(STORE, id);
  },

  /**
   * 简单搜索:按名称, 内容, 标签匹配
   */
  async search(query) {
    const all = await getAll(STORE);
    const q = query.toLowerCase();
    return all.filter(m => {
      if (m.name.toLowerCase().includes(q)) return true;
      if (m.content.toLowerCase().includes(q)) return true;
      if (m.tags.some(t => t.toLowerCase().includes(q))) return true;
      return false;
    });
  },
};
