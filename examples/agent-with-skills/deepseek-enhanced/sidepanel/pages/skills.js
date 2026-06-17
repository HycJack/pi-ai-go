/**
 * sidepanel/pages/skills.js
 * Skill 管理页面
 */

import { sendToSW, escapeHtml, createEl } from '../app.js';

export default function SkillsPage() {
  const container = document.createElement('div');
  let skills = [];

  render();

  async function render() {
    container.innerHTML = `
      <div id="skillList" class="skill-list"></div>
    `;

    await loadSkills();
    renderList();
  }

  async function loadSkills() {
    try {
      skills = await sendToSW({ type: 'getSkills' });
    } catch (e) {
      console.error('Load skills error:', e);
      skills = [];
    }
  }

  function renderList() {
    const list = container.querySelector('#skillList');

    if (skills.length === 0) {
      list.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">🎯</div>
          <div class="empty-state-text">暂无可用的 Skill</div>
        </div>
      `;
      return;
    }

    list.innerHTML = skills.map(s => `
      <div class="skill-card ${s.isBuiltin ? 'builtin' : ''}" data-id="${s.id}">
        <div class="skill-card-name">
          ${s.isBuiltin ? '🔧' : '✏️'} ${escapeHtml(s.name)}
          ${s.isBuiltin ? '<span class="memory-card-tag" style="margin-left:8px;">内置</span>' : ''}
        </div>
        <div class="skill-card-desc">${escapeHtml(s.description || '')}</div>
        <div class="skill-card-trigger">/${s.id}</div>
        ${!s.isBuiltin ? `
          <div style="margin-top:8px;">
            <button class="btn btn-danger btn-xs delete-skill-btn">删除</button>
          </div>
        ` : ''}
      </div>
    `).join('');

    // 绑定删除按钮事件
    list.querySelectorAll('.delete-skill-btn').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        const card = e.target.closest('.skill-card');
        const id = card.dataset.id;
        if (confirm('确定要删除这个自定义 Skill 吗?')) {
          await sendToSW({ type: 'deleteSkill', id });
          await loadSkills();
          renderList();
        }
      });
    });
  }

  return {
    container,
    refresh: async () => {
      await loadSkills();
      renderList();
    },
  };
}
