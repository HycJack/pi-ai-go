/**
 * Prompt 增强核心逻辑
 * 从 fetch 拦截中调用,为请求注入记忆, Skill, 知识库, 工具定义
 */

import { buildToolInstructions, buildMemoryContext, buildSkillContext, buildKnowledgeContext } from './templates.js';
import { matchMemories } from '../memory/matching.js';
import { matchSkill } from '../skill/trigger.js';
import { searchKnowledge } from '../knowledge/retrieval.js';
import { getToolDefinitions } from '../tools/index.js';

/**
 * 增强请求体
 * @param {Object} body - 原始请求体 { messages, model, ... }
 * @returns {Object} 增强后的请求体
 */
export async function augmentRequest(body) {
  const messages = body.messages || [];
  const lastUserMsg = getLastUserMessage(messages);
  if (!lastUserMsg) return body;

  const userInput = lastUserMsg.content || '';

  // 1. 检测 Skill 触发
  const skillResult = matchSkill(userInput);

  // 2. 匹配相关记忆
  let memories = [];
  try {
    memories = await matchMemories(userInput);
  } catch (e) {
    console.warn('[DeepSeek Enhanced] Memory match error:', e);
  }

  // 3. 搜索知识库
  let knowledgeItems = [];
  try {
    knowledgeItems = await searchKnowledge(userInput, 3);
  } catch (e) {
    console.warn('[DeepSeek Enhanced] Knowledge search error:', e);
  }

  // 4. 工具定义
  const toolDefinitions = getToolDefinitions();

  // 5. 构建增强内容
  const blocks = [];

  // 工具说明(始终注入)
  blocks.push(buildToolInstructions());

  // Skill 上下文
  if (skillResult) {
    const sc = buildSkillContext(skillResult);
    if (sc) blocks.push(sc);
  }

  // 记忆上下文
  const mc = buildMemoryContext(memories);
  if (mc) blocks.push(mc);

  // 知识库上下文
  const kc = buildKnowledgeContext(knowledgeItems);
  if (kc) blocks.push(kc);

  const enhancedContent = blocks.join('\n\n');

  // 6. 注入到 messages
  const newMessages = injectSystemPrompt(messages, enhancedContent);

  return { ...body, messages: newMessages };
}

/**
 * 将增强内容注入到 system prompt
 */
function injectSystemPrompt(messages, enhancedContent) {
  const result = [...messages];
  const systemIdx = result.findIndex(m => m.role === 'system');

  if (systemIdx >= 0) {
    result[systemIdx] = {
      ...result[systemIdx],
      content: result[systemIdx].content + '\n\n' + enhancedContent,
    };
  } else {
    result.unshift({ role: 'system', content: enhancedContent });
  }

  return result;
}

/**
 * 获取最后一条用户消息
 */
function getLastUserMessage(messages) {
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === 'user') {
      return messages[i];
    }
  }
  return null;
}
