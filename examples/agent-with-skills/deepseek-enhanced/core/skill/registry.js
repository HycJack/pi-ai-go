/**
 * Skill 注册表
 * 内置 Skill + 自定义 Skill 的统一入口
 */

import { SkillStore } from '../db/skill-store.js';
import { BUILTIN_SKILLS } from './builtins/index.js';

/**
 * 获取 Skill(先查自定义,再查内置)
 * @param {string} name
 * @returns {Object|null}
 */
export function getSkill(name) {
  return BUILTIN_SKILLS[name] || null;
}

/**
 * 获取所有可用 Skill
 * @returns {Promise<Array>}
 */
export async function getAllSkills() {
  const customSkills = await SkillStore.all();
  const builtinList = Object.entries(BUILTIN_SKILLS).map(([id, skill]) => ({
    id,
    name: skill.name,
    description: skill.description,
    isBuiltin: true,
  }));
  return [...builtinList, ...customSkills];
}
