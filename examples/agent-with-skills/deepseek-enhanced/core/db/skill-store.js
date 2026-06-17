/**
 * Skill Store - 自定义 Skill 的 CRUD
 */

import { getAll, getById, put, remove } from './indexeddb.js';
import { STORE_NAMES } from '../constants.js';

const STORE = STORE_NAMES.SKILLS;

export const SkillStore = {
  async all() {
    return await getAll(STORE);
  },

  async get(id) {
    return await getById(STORE, id);
  },

  async save(skill) {
    const item = {
      id: skill.id || `custom_${Date.now()}`,
      name: skill.name || 'unnamed',
      description: '',
      template: '',
      tags: [],
      source: 'custom',
      createdAt: skill.createdAt || Date.now(),
      updatedAt: Date.now(),
      ...skill,
    };
    await put(STORE, item);
    return item;
  },

  async delete(id) {
    return await remove(STORE, id);
  },
};
