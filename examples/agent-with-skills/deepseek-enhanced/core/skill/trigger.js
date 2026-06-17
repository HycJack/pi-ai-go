/**
 * Skill 触发解析
 * 检测用户输入中的 /skill-name 前缀
 */

import { getSkill } from './registry.js';

/**
 * 匹配用户输入中的 Skill 触发
 * @param {string} userInput
 * @returns {Object|null} { name, template, input } 或 null
 */
export function matchSkill(userInput) {
  const trimmed = userInput.trim();
  const match = trimmed.match(/^\/(\S+)(?:\s+(.+))?/s);
  if (!match) return null;

  const name = match[1].toLowerCase();
  const input = match[2] || trimmed;

  const skill = getSkill(name);
  if (!skill) return null;

  return {
    name: skill.name,
    template: skill.template,
    input,
  };
}
