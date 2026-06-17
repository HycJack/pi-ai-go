/**
 * sidepanel/pages/knowledge.js
 * 知识库管理页面
 */

import { sendToSW, formatDate, escapeHtml, createEl } from '../app.js';

export default function KnowledgePage() {
  const container = document.createElement('div');
  let items = [];
  let categories = [];
  let activeCategory = 'all';
  let searchQuery = '';

  render();

  async function render() {
    container.innerHTML = `
      <div class="search-bar">
        <input type="text" class="form-input" placeholder="🔍 搜索知识库..." id="kwSearch">
      </div>
      <div id="kwCategoryNav" class="category-nav"></div>
      <div id="kwList" class="knowledge-list"></div>
      <div class="fab-bar">
        <button class="btn btn-outline" id="addCatBtn" style="margin-right:8px;">+ 分类</button>
        <button class="btn btn-primary" id="addKwBtn">+ 添加知识</button>
      </div>
    `;

    container.querySelector('#kwSearch').addEventListener('input', (e) => {
      searchQuery = e.target.value.toLowerCase();
      renderList();
    });

    container.querySelector('#addKwBtn').addEventListener('click', () => {
      showKnowledgeModal();
    });

    container.querySelector('#addCatBtn').addEventListener('click', () => {
      showCategoryModal();
    });

    await loadData();
    renderCategoryNav();
    renderList();
  }

  async function loadData() {
    try {
      [items, categories] = await Promise.all([
        sendToSW({ type: 'getKnowledgeItems' }),
        sendToSW({ type: 'getKnowledgeCategories' }),
      ]);
    } catch (e) {
      console.error('Load knowledge error:', e);
      items = [];
      categories = [];
    }
  }

  function renderCategoryNav() {
    const nav = container.querySelector('#kwCategoryNav');
    const chips = [
      { id: 'all', name: '全部', color: '#64748b' },
      ...categories,
    ];

    nav.innerHTML = chips.map(c => `
      <span class="category-chip ${activeCategory === c.id ? 'active' : ''}"
            data-cat="${c.id}"
            style="${c.color && activeCategory === c.id ? `background:${c.color};border-color:${c.color};` : ''}">
        ${escapeHtml(c.name)}
      </span>
    `).join('');

    nav.querySelectorAll('.category-chip').forEach(chip => {
      chip.addEventListener('click', () => {
        activeCategory = chip.dataset.cat;
        renderCategoryNav();
        renderList();
      });
    });
  }

  function renderList() {
    const list = container.querySelector('#kwList');

    let filtered = items;
    if (activeCategory !== 'all') {
      filtered = filtered.filter(i => i.category === activeCategory);
    }
    if (searchQuery) {
      filtered = filtered.filter(i =>
        (i.title || '').toLowerCase().includes(searchQuery) ||
        (i.content || '').toLowerCase().includes(searchQuery) ||
        (i.tags || []).some(t => t.toLowerCase().includes(searchQuery))
      );
    }

    if (filtered.length === 0) {
      list.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">📚</div>
          <div class="empty-state-text">
            ${searchQuery ? '没有匹配的知识条目' : '知识库为空,点击下方按钮添加'}
          </div>
        </div>
      `;
      return;
    }

    list.innerHTML = filtered.map(item => {
      const cat = categories.find(c => c.id === item.category);
      return `
      <div class="knowledge-card" data-id="${item.id}">
        <div class="knowledge-card-title">${escapeHtml(item.title || '未命名')}</div>
        <div class="knowledge-card-meta">
          ${cat ? `<span style="color:${cat.color || '#6366f1'}">📁 ${escapeHtml(cat.name)}</span>` : ''}
          <span>${formatDate(item.updatedAt)}</span>
        </div>
        ${(item.tags || []).length ? `
          <div class="memory-card-tags" style="margin-top:4px;">
            ${item.tags.map(t => `<span class="memory-card-tag">${escapeHtml(t)}</span>`).join('')}
          </div>
        ` : ''}
        <div class="memory-card-actions" style="margin-top:8px;">
          <button class="btn btn-outline btn-xs edit-btn">编辑</button>
          <button class="btn btn-danger btn-xs delete-btn">删除</button>
        </div>
      </div>
    `}).join('');

    // 绑定事件
    list.querySelectorAll('.knowledge-card').forEach(card => {
      const id = Number(card.dataset.id);
      card.querySelector('.edit-btn')?.addEventListener('click', (e) => {
        e.stopPropagation();
        const item = items.find(i => i.id === id);
        if (item) showKnowledgeModal(item);
      });
      card.querySelector('.delete-btn')?.addEventListener('click', async (e) => {
        e.stopPropagation();
        if (confirm('确定要删除这个知识点吗?')) {
          await sendToSW({ type: 'deleteKnowledgeItem', id });
          await loadData();
          renderList();
        }
      });
    });
  }

  // ==================== 知识编辑模态框 ====================

  function showKnowledgeModal(item = null) {
    const isEdit = !!item;
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';

    const catOptions = categories.map(c =>
      `<option value="${c.id}" ${(item?.category === c.id) ? 'selected' : ''}>${escapeHtml(c.name)}</option>`
    ).join('');

    overlay.innerHTML = `
      <div class="modal">
        <h3 class="modal-title">${isEdit ? '编辑知识' : '添加知识'}</h3>
        <form id="kwForm">
          <div class="form-group">
            <label class="form-label">标题</label>
            <input name="title" type="text" class="form-input" value="${escapeHtml(item?.title || '')}" placeholder="知识标题">
          </div>
          <div class="form-group">
            <label class="form-label">分类</label>
            <select name="category" class="form-select">
              <option value="">无分类</option>
              ${catOptions}
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">内容(Markdown)</label>
            <textarea name="content" class="form-textarea" rows="8" placeholder="支持 Markdown 格式...">${escapeHtml(item?.content || '')}</textarea>
          </div>
          <div class="form-group">
            <label class="form-label">标签(逗号分隔)</label>
            <input name="tags" type="text" class="form-input" value="${(item?.tags || []).join(', ')}" placeholder="关键词, 便于检索">
          </div>
          <div class="form-group">
            <label class="form-label">来源</label>
            <input name="source" type="text" class="form-input" value="${escapeHtml(item?.source || 'manual')}" placeholder="manual / url / ai">
          </div>
          <div class="modal-actions">
            <button type="button" class="btn btn-outline cancel-btn">取消</button>
            <button type="submit" class="btn btn-primary">${isEdit ? '保存' : '添加'}</button>
          </div>
        </form>
      </div>
    `;

    document.body.appendChild(overlay);

    overlay.querySelector('.cancel-btn').addEventListener('click', () => overlay.remove());
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });

    overlay.querySelector('#kwForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const fd = new FormData(e.target);
      const data = {
        title: fd.get('title'),
        content: fd.get('content'),
        category: fd.get('category') || '',
        tags: fd.get('tags').split(',').map(t => t.trim()).filter(Boolean),
        source: fd.get('source') || 'manual',
      };

      try {
        if (isEdit) {
          await sendToSW({ type: 'updateKnowledgeItem', id: item.id, item: data });
        } else {
          await sendToSW({ type: 'saveKnowledgeItem', item: data });
        }
        overlay.remove();
        await loadData();
        renderCategoryNav();
        renderList();
      } catch (err) {
        alert('保存失败: ' + err.message);
      }
    });
  }

  // ==================== 分类编辑模态框 ====================

  function showCategoryModal(cat = null) {
    const isEdit = !!cat;
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';

    overlay.innerHTML = `
      <div class="modal">
        <h3 class="modal-title">${isEdit ? '编辑分类' : '添加分类'}</h3>
        <form id="catForm">
          <div class="form-group">
            <label class="form-label">名称</label>
            <input name="name" type="text" class="form-input" value="${escapeHtml(cat?.name || '')}" placeholder="分类名称">
          </div>
          <div class="form-group">
            <label class="form-label">描述</label>
            <input name="description" type="text" class="form-input" value="${escapeHtml(cat?.description || '')}" placeholder="简短描述">
          </div>
          <div class="form-group">
            <label class="form-label">颜色</label>
            <input name="color" type="color" class="form-input" value="${cat?.color || '#6366f1'}" style="width:60px;padding:2px;">
          </div>
          <div class="modal-actions">
            <button type="button" class="btn btn-outline cancel-btn">取消</button>
            <button type="submit" class="btn btn-primary">${isEdit ? '保存' : '添加'}</button>
          </div>
        </form>
      </div>
    `;

    document.body.appendChild(overlay);

    overlay.querySelector('.cancel-btn').addEventListener('click', () => overlay.remove());
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });

    overlay.querySelector('#catForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const fd = new FormData(e.target);
      const data = {
        name: fd.get('name'),
        description: fd.get('description'),
        color: fd.get('color'),
      };

      try {
        if (isEdit) {
          const merged = { ...cat, ...data };
          await sendToSW({ type: 'saveKnowledgeCategory', category: merged });
        } else {
          await sendToSW({ type: 'saveKnowledgeCategory', category: data });
        }
        overlay.remove();
        await loadData();
        renderCategoryNav();
        renderList();
      } catch (err) {
        alert('保存失败: ' + err.message);
      }
    });
  }

  return {
    container,
    refresh: async () => {
      await loadData();
      renderCategoryNav();
      renderList();
    },
  };
}
