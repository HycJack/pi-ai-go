/**
 * IndexedDB 封装 - 提供 Promise 化的 CRUD 操作
 */

import { DB_NAME, DB_VERSION, STORE_NAMES } from '../constants.js';

let db = null;

/**
 * 打开数据库,如果不存在则创建
 */
export async function openDB() {
  if (db) return db;
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);
    
    request.onerror = () => reject(request.error);
    request.onsuccess = () => {
      db = request.result;
      resolve(db);
    };
    
    request.onupgradeneeded = (event) => {
      const database = event.target.result;
      
      // 记忆表
      if (!database.objectStoreNames.contains(STORE_NAMES.MEMORIES)) {
        const store = database.createObjectStore(STORE_NAMES.MEMORIES, {
          keyPath: 'id',
          autoIncrement: true,
        });
        store.createIndex('type', 'type', { unique: false });
        store.createIndex('pinned', 'pinned', { unique: false });
        store.createIndex('updatedAt', 'updatedAt', { unique: false });
      }
      
      // 自定义 Skill 表
      if (!database.objectStoreNames.contains(STORE_NAMES.SKILLS)) {
        const store = database.createObjectStore(STORE_NAMES.SKILLS, {
          keyPath: 'id',
        });
        store.createIndex('name', 'name', { unique: true });
      }
      
      // 知识库条目表
      if (!database.objectStoreNames.contains(STORE_NAMES.KNOWLEDGE_ITEMS)) {
        const store = database.createObjectStore(STORE_NAMES.KNOWLEDGE_ITEMS, {
          keyPath: 'id',
          autoIncrement: true,
        });
        store.createIndex('category', 'category', { unique: false });
        store.createIndex('updatedAt', 'updatedAt', { unique: false });
      }
      
      // 知识库分类表
      if (!database.objectStoreNames.contains(STORE_NAMES.KNOWLEDGE_CATEGORIES)) {
        database.createObjectStore(STORE_NAMES.KNOWLEDGE_CATEGORIES, {
          keyPath: 'id',
        });
      }
    };
  });
}

function getStore(storeName, mode = 'readonly') {
  return db.transaction(storeName, mode).objectStore(storeName);
}

// ---- 通用 CRUD ----

export async function getAll(storeName) {
  await openDB();
  return new Promise((resolve, reject) => {
    const request = getStore(storeName).getAll();
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

export async function getById(storeName, id) {
  await openDB();
  return new Promise((resolve, reject) => {
    const request = getStore(storeName).get(id);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

export async function put(storeName, item) {
  await openDB();
  return new Promise((resolve, reject) => {
    const request = getStore(storeName, 'readwrite').put(item);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

export async function remove(storeName, id) {
  await openDB();
  return new Promise((resolve, reject) => {
    const request = getStore(storeName, 'readwrite').delete(id);
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error);
  });
}

export async function getByIndex(storeName, indexName, value) {
  await openDB();
  return new Promise((resolve, reject) => {
    const request = getStore(storeName).index(indexName).getAll(value);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

export async function clear(storeName) {
  await openDB();
  return new Promise((resolve, reject) => {
    const request = getStore(storeName, 'readwrite').clear();
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error);
  });
}
