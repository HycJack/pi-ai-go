/**
 * 工具注册 + 执行路由
 * 新增工具只需在这里 TOOLS 对象中添加一个函数
 */

import { MemoryStore } from '../db/memory-store.js';
import { KnowledgeStore } from '../db/knowledge-store.js';

const TOOLS = {
  async memory_save(params) {
    const result = await MemoryStore.save({
      type: params.type || 'topic',
      name: params.name,
      content: params.content,
      tags: params.tags || [],
      pinned: params.pinned || false,
    });
    return { success: true, id: result.id, action: 'saved' };
  },

  async memory_update(params) {
    const result = await MemoryStore.update(params.id, {
      type: params.type,
      name: params.name,
      content: params.content,
      tags: params.tags,
      pinned: params.pinned,
    });
    return { success: true, id: result.id, action: 'updated' };
  },

  async memory_delete(params) {
    await MemoryStore.delete(params.id);
    return { success: true, id: params.id, action: 'deleted' };
  },

  async knowledge_save(params) {
    const result = await KnowledgeStore.saveItem({
      title: params.title,
      content: params.content,
      category: params.category || '',
      tags: params.tags || [],
      source: 'ai',
    });
    return { success: true, id: result.id, action: 'saved' };
  },

  async knowledge_search(params) {
    const results = await KnowledgeStore.search(params.query, params.limit || 5);
    return { success: true, results };
  },

  async web_search(params) {
    // 使用 DuckDuckGo API(无需 key),或后续扩展
    try {
      const response = await fetch(
        `https://api.duckduckgo.com/?q=${encodeURIComponent(params.query)}&format=json&no_html=1`,
        { signal: AbortSignal.timeout(10000) }
      );
      const data = await response.json();
      const results = (data.RelatedTopics || [])
        .slice(0, 5)
        .map(t => ({
          title: t.Text ? t.Text.substring(0, 80) : '',
          url: t.FirstURL || '',
          snippet: t.Text || '',
        }));
      return { success: true, query: params.query, results };
    } catch (e) {
      return { success: false, error: e.message, query: params.query };
    }
  },

  async web_fetch(params) {
    try {
      const response = await fetch(params.url, {
        signal: AbortSignal.timeout(15000),
      });
      const text = await response.text();
      // 提取纯文本(简单处理,去掉 HTML 标签)
      const plainText = text
        .replace(/<script[^>]*>[\s\S]*?<\/script>/gi, '')
        .replace(/<style[^>]*>[\s\S]*?<\/style>/gi, '')
        .replace(/<[^>]+>/g, ' ')
        .replace(/\s+/g, ' ')
        .trim()
        .substring(0, 5000);
      return { success: true, url: params.url, content: plainText };
    } catch (e) {
      return { success: false, error: e.message, url: params.url };
    }
  },
};

/**
 * 获取所有工具的定义(用于注入到 system prompt)
 */
export function getToolDefinitions() {
  const names = Object.keys(TOOLS);
  return names.map(name => ({
    name,
    description: getToolDescription(name),
  }));
}

function getToolDescription(name) {
  const descriptions = {
    memory_save: '保存一条长期记忆。参数:type("user"|"feedback"|"topic"|"reference"), name(标题), content(内容), tags(标签数组), pinned(是否置顶)',
    memory_update: '更新一条已存在的记忆。参数:id(记忆ID), 以及要更新的字段',
    memory_delete: '删除一条记忆。参数:id(记忆ID)',
    knowledge_save: '保存一条知识到知识库。参数:title, content, category, tags',
    knowledge_search: '搜索知识库。参数:query, limit',
    web_search: '搜索互联网。参数:query',
    web_fetch: '获取网页内容。参数:url',
  };
  return descriptions[name] || '未知工具';
}

/**
 * 执行工具调用
 */
export async function executeTool(toolName, params) {
  const fn = TOOLS[toolName];
  if (!fn) return { error: `Unknown tool: ${toolName}` };
  try {
    return await fn(params);
  } catch (e) {
    return { error: e.message };
  }
}
