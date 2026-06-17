/**
 * 系统提示词模板
 */

/**
 * 构建工具调用格式说明(注入到 system prompt)
 */
export function buildToolInstructions() {
  return `## 工具调用

你可以使用以下工具。工具调用必须严格使用 XML 格式:

格式:<tool_call>工具名{"参数"}</tool_call>

示例:
- 保存记忆:<tool_call>memory_save{"type":"user","name":"职业","content":"用户是前端开发工程师","tags":["职业","开发"]}</tool_call>
- 更新记忆:<tool_call>memory_update{"id":1,"content":"更新后的内容"}</tool_call>
- 删除记忆:<tool_call>memory_delete{"id":3}</tool_call>
- 搜索知识库:<tool_call>knowledge_search{"query":"React Hooks","limit":3}</tool_call>
- 搜索互联网:<tool_call>web_search{"query":"2025年前端趋势"}</tool_call>
- 获取网页:<tool_call>web_fetch{"url":"https://example.com"}</tool_call>

注意事项:
1. 工具 XML 标签放在回复的末尾
2. 每个工具调用单独一行
3. 参数必须是合法的 JSON
4. 如果不需要使用工具,则不要输出 <tool_call> 标签`;
}

/**
 * 构建包含记忆上下文的 system prompt 前缀
 */
export function buildMemoryContext(memories) {
  if (!memories || memories.length === 0) return '';

  const lines = ['## 关于用户的长期记忆'];
  for (const m of memories) {
    const typeLabel = {
      user: '身份/偏好',
      feedback: '行为反馈',
      topic: '讨论要点',
      reference: '参考资料',
    }[m.type] || m.type;

    lines.push(`- [${typeLabel}] ${m.name}: ${m.content.substring(0, 200)}`);
  }
  return lines.join('\n');
}

/**
 * 构建 Skill 上下文
 */
export function buildSkillContext(skill) {
  if (!skill) return '';
  return `## 当前激活的 Skill: ${skill.name}

${skill.template.replace('{{input}}', skill.input || '')}`;
}

/**
 * 构建知识库上下文
 */
export function buildKnowledgeContext(knowledgeItems) {
  if (!knowledgeItems || knowledgeItems.length === 0) return '';

  const lines = ['## 相关知识库内容'];
  for (const item of knowledgeItems) {
    lines.push(`### ${item.title}`);
    lines.push(item.content.substring(0, 500));
    lines.push('');
  }
  return lines.join('\n');
}
