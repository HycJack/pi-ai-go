/**
 * XML 工具调用解析 + 剥离
 *
 * 支持的格式:
 *   <tool_call>tool_name{"key":"value"}</tool_call>
 *   <tool_call>tool_name</tool_call>
 */

const TOOL_CALL_REGEX = /<tool_call>(\w+)(\{[\s\S]*?\})?<\/tool_call>/g;

/**
 * 从文本中解析所有工具调用
 * @param {string} text
 * @returns {Array<{tool: string, params: Object}>}
 */
export function parseToolCalls(text) {
  const calls = [];
  const re = new RegExp(TOOL_CALL_REGEX.source, 'g');
  let match;
  while ((match = re.exec(text)) !== null) {
    const toolName = match[1];
    let params = {};
    if (match[2]) {
      try {
        params = JSON.parse(match[2]);
      } catch (e) {
        console.warn('[DeepSeek Enhanced] JSON parse error for tool:', toolName, e);
        params = { raw: match[2] };
      }
    }
    calls.push({ tool: toolName, params });
  }
  return calls;
}

/**
 * 从文本中移除所有工具调用标签
 * @param {string} text
 * @returns {string}
 */
export function stripToolCalls(text) {
  return text.replace(TOOL_CALL_REGEX, '').trim();
}

/**
 * 检查文本中是否包含工具调用
 * @param {string} text
 * @returns {boolean}
 */
export function hasToolCalls(text) {
  return new RegExp(TOOL_CALL_REGEX.source, 'g').test(text);
}
