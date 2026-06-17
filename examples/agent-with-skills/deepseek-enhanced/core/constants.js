// DOM 选择器相关常量
export const SELECTORS = {
  // DeepSeek 输入框
  INPUT_AREA: '#chat-input, [contenteditable="true"][data-placeholder], textarea[placeholder*="发消息"], textarea[placeholder*="message"], .chat-input-editor, [role="textbox"]',
  INPUT_CONTAINER: '.chat-input-container, .input-container, [class*="input-area"]',
  // DeepSeek 回复区域
  MESSAGE_LIST: '.chat-list, .message-list, [class*="message-list"], [class*="conversation"]',
  MESSAGE_ITEM: '.message, .chat-message, [class*="message-item"], [class*="bubble"]',
  // AI 回复的 DOM 容器
  AI_MESSAGE: '.message.assistant, .chat-message.assistant, [class*="assistant"], [class*="bot"]',
  // 发送按钮
  SEND_BUTTON: 'button[type="submit"], [data-testid="send"], [class*="send"]',
};

// API 端点
export const API_PATTERNS = {
  CHAT_COMPLETIONS: '/chat/completions',
};

// 数据库名和存储名
export const DB_NAME = 'deepseek-enhanced';
export const DB_VERSION = 1;

export const STORE_NAMES = {
  MEMORIES: 'memories',
  SKILLS: 'skills',
  KNOWLEDGE_ITEMS: 'knowledge_items',
  KNOWLEDGE_CATEGORIES: 'knowledge_categories',
};

// 记忆类型
export const MEMORY_TYPES = {
  USER: 'user',
  FEEDBACK: 'feedback',
  TOPIC: 'topic',
  REFERENCE: 'reference',
};

// 消息类型(Service Worker <-> Content Script)
export const MSG_TYPES = {
  EXECUTE_TOOL: 'EXECUTE_TOOL',
  TOOL_RESULT: 'TOOL_RESULT',
  GET_MEMORIES: 'GET_MEMORIES',
  SAVE_MEMORY: 'SAVE_MEMORY',
  UPDATE_MEMORY: 'UPDATE_MEMORY',
  DELETE_MEMORY: 'DELETE_MEMORY',
  GET_KNOWLEDGE: 'GET_KNOWLEDGE',
  SAVE_KNOWLEDGE: 'SAVE_KNOWLEDGE',
  SEARCH_KNOWLEDGE: 'SEARCH_KNOWLEDGE',
  GET_SKILLS: 'GET_SKILLS',
  SAVE_SKILL: 'SAVE_SKILL',
  DELETE_SKILL: 'DELETE_SKILL',
  GET_SETTINGS: 'GET_SETTINGS',
  SAVE_SETTINGS: 'SAVE_SETTINGS',
};
