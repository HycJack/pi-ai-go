/**
 * sidepanel/pages/memories.js
 * 记忆管理页面
 */

import { sendToSW, formatDate, escapeHtml, createEl } from '../app.js';

export default function MemoriesPage() {
  const container = document.createElement('div');
  let memories = [];
  let searchQuery = '';

  render();

  async function render() {
    container.innerHTML = `
      <div class="search-bar">
        <input type="text" class="form-input" placeholder="🔍 搜索记忆..." id="memSearch">
      </div>
      <div id="memoryList" class="memory-list"></div>
      <div class="fab-bar">
        <button class="btn btn-primary" id="addMemoryBtn">
          + 添加记忆
        </button>
      </div>
    `;

    container.querySelector('#memSearch').addEventListener('input', (e) => {
      searchQuery = e.target.value.toLowerCase();
      renderList();
    });

    container.querySelector('#addMemoryBtn').addEventListener('click', () => {
      showMemoryModal();
    });

    await loadMemories();
    renderList();
  }

  async function loadMemories() {
    try {
      memories = await sendToSW({ type: 'getMemories' });
    } catch (e) {
      console.error('Load memories error:', e);
      memories = [];
    }
  }

  function renderList() {
    const list = container.querySelector('#memoryList');
    const filtered = searchQuery
      ? memories.filter(m =>
          (m.name || '').toLowerCase().includes(searchQuery) ||
          (m.content || '').toLowerCase().includes(searchQuery) ||
          (m.tags || []).some(t => t.toLowerCase().includes(searchQuery))
        )
      : memories;

    if (filtered.length === 0) {
      list.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">🧠</div>
          <div class="empty-state-text">${searchQuery ? '没有匹配的记忆' : '还没有记忆,点击下方按钮添加'}</div>
        </div>
      `;
      return;
    }

    list.innerHTML = filtered.map(m => `
      <div class="memory-card ${m.pinned ? 'pinned' : ''}" data-id="${m.id}">
        <div class="memory-card-header">
          <span class="memory-card-name">
            ${m.pinned ? '📌' : ''} ${escapeHtml(m.name || '未命名')}
          </span>
          <span class="memory-card-type ${m.type}">${typeLabel(m.type)}</span>
        </div>
        <div class="memory-card-content">${escapeHtml(m.content || '')}</div>
        <div class="memory-card-tags">
          ${(m.tags || []).map(t => `<span class="memory-card-tag">${escapeHtml(t)}</span>`).join('')}
        </div>
        <div class="memory-card-actions">
          <button class="btn btn-outline btn-xs edit-btn">编辑</button>
          <button class="btn btn-outline btn-xs pin-btn">${m.pinned ? '取消置顶' : '置顶'}</button>
          <button class="btn btn-danger btn-xs delete-btn">删除</button>
        </div>
      </div>
    `).join('');

    // 绑定事件
    list.querySelectorAll('.memory-card').forEach(card => {
      const id = Number(card.dataset.id);
      card.querySelector('.edit-btn')?.addEventListener('click', (e) => {
        e.stopPropagation();
        const mem = memories.find(m => m.id === id);
        if (mem) showMemoryModal(mem);
      });
      card.querySelector('.pin-btn')?.addEventListener('click', async (e) => {
        e.stopPropagation();
        const mem = memories.find(m => m.id === id);
        if (mem) {
          await sendToSW({ type: 'updateMemory', id, memory: { pinned: !mem.pinned } });
          await loadMemories();
          renderList();
        }
      });
      card.querySelector('.delete-btn')?.addEventListener('click', async (e) => {
        e.stopPropagation();
        if (confirm('确定要删除这条记忆吗?')) {
          await sendToSW({ type: 'deleteMemory', id });
          await loadMemories();
          renderList();
        }
      });
    });
  }

  function typeLabel(type) {
    const map = { user: '身份偏好', feedback: '行为反馈', topic: '讨论要点', reference: '参考资料' };
    return map[type] || type;
  }

  // ==================== 记忆编辑模态框 ====================

  function showMemoryModal(memory = null) {
    const isEdit = !!memory;
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';

    const typeOptions = [
      { value: 'user', label: '身份偏好' },
      { value: 'feedback', label: '行为反馈' },
      { value: 'topic', label: '讨论要点' },
      { value: 'reference', label: '参考资料' },
    ];

    overlay.innerHTML = `
      <div class="modal">
        <h3 class="modal-title">${isEdit ? '编辑记忆' : '添加记忆'}</h3>
        <form id="memoryForm">
          <div class="form-group">
            <label class="form-label">类型</label>
            <select name="type" class="form-select">
              ${typeOptions.map(o => `<option value="${o.value}" ${(memory?.type === o.value) ? 'selected' : ''}>${o.label}</option>`).join('')}
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">标题</label>
            <input name="name" type="text" class="form-input" value="${escapeHtml(memory?.name || '')}" placeholder="简短标题,如:用户是前端开发">
          </div>
          <div class="form-group">
            <label class="form-label">内容</label>
            <textarea name="content" class="form-textarea" rows="4" placeholder="详细内容...">${escapeHtml(memory?.content || '')}</textarea>
          </div>
          <div class="form-group">
            <label class="form-label">标签(逗号分隔)</label>
            <input name="tags" type="text" class="form-input" value="${(memory?.tags || []).join(', ')}" placeholder="React, TypeScript, 前端">
          </div>
          <div class="form-group">
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
              <input name="pinned" type="checkbox" ${memory?.pinned ? 'checked' : ''}> 置顶
            </label>
          </div>
          <div class="modal-actions">
            <button type="button" class="btn btn-outline cancel-btn">取消</button>
            <button type="submit" class="btn btn-primary">${isEdit ? '保存' : '添加'}</button>
          </div>
        </form>
      </div>
    `;

    document.body.appendChild(overlay);

    const form = overlay.querySelector('#memoryForm');

    overlay.querySelector('.cancel-btn').addEventListener('click', () => {
      overlay.remove();
    });

    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) overlay.remove();
    });

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const fd = new FormData(form);
      const data = {
        type: fd.get('type'),
        name: fd.get('name'),
        content: fd.get('content'),
        tags: fd.get('tags').split(',').map(t => t.trim()).filter(Boolean),
        pinned: fd.get('pinned') === 'on',
      };

      try {
        if (isEdit) {
          await sendToSW({ type: 'updateMemory', id: memory.id, memory: data });
        } else {
          await sendToSW({ type: 'saveMemory', memory: data });
        }
        overlay.remove();
        await loadMemories();
        renderList();
      } catch (err) {
        alert('保存失败: ' + err.message);
      }
    });
  }

  return {
    container,
    refresh: async () => {
      await loadMemories();
      renderList();
    },
  };
}
