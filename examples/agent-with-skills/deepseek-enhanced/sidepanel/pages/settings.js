/**
 * sidepanel/pages/settings.js
 * 设置面板
 */

import { sendToSW, createEl } from '../app.js';

const SettingsPanel = {
  async render(container) {
    container.innerHTML = '';

    let settings = {};
    try {
      settings = await sendToSW({ type: 'getSettings', keys: null });
    } catch (e) {
      console.error('Load settings error:', e);
    }

    const wrapper = createEl('div', { className: 'settings-panel' });

    // --- API Key ---
    const apiSection = createEl('div', { className: 'settings-section' });
    apiSection.appendChild(createEl('h3', { className: 'settings-section-title', textContent: '🔑 API 设置' }));

    const apiKeyGroup = createEl('div', { className: 'form-group' });
    apiKeyGroup.appendChild(createEl('label', { className: 'form-label', textContent: 'DeepSeek API Key' }));
    const apiKeyInput = createEl('input', {
      className: 'form-input',
      type: 'password',
      placeholder: 'sk-...',
      value: settings.token || '',
    });
    apiKeyGroup.appendChild(apiKeyInput);
    apiSection.appendChild(apiKeyGroup);
    wrapper.appendChild(apiSection);

    // --- 行为设置 ---
    const behaviorSection = createEl('div', { className: 'settings-section' });
    behaviorSection.appendChild(createEl('h3', { className: 'settings-section-title', textContent: '⚡ 行为设置' }));

    const autoSaveGroup = createEl('div', { className: 'form-group' });
    const autoSaveLabel = createEl('label', {
      style: { display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' },
    });
    const autoSaveCheckbox = createEl('input', { type: 'checkbox' });
    autoSaveCheckbox.checked = settings.autoSaveMemory !== false;
    autoSaveLabel.appendChild(autoSaveCheckbox);
    autoSaveLabel.appendChild(document.createTextNode('AI 自动保存记忆'));
    autoSaveGroup.appendChild(autoSaveLabel);
    autoSaveGroup.appendChild(createEl('p', {
      style: { fontSize: '11px', color: '#94a3b8', marginTop: '4px' },
      textContent: '开启后,AI 会在对话中自动识别并保存重要信息到记忆系统',
    }));
    behaviorSection.appendChild(autoSaveGroup);
    wrapper.appendChild(behaviorSection);

    // --- 数据管理 ---
    const dataSection = createEl('div', { className: 'settings-section' });
    dataSection.appendChild(createEl('h3', { className: 'settings-section-title', textContent: '💾 数据管理' }));

    const exportBtn = createEl('button', { className: 'btn btn-outline', textContent: '📥 导出所有数据', style: { marginRight: '8px' } });
    exportBtn.addEventListener('click', async () => {
      try {
        const [memories, skills, items, categories] = await Promise.all([
          sendToSW({ type: 'getMemories' }),
          sendToSW({ type: 'getSkills' }),
          sendToSW({ type: 'getKnowledgeItems' }),
          sendToSW({ type: 'getKnowledgeCategories' }),
        ]);
        const data = { memories, skills, knowledgeItems: items, knowledgeCategories: categories, exportDate: new Date().toISOString() };
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `deepseek-enhanced-backup-${new Date().toISOString().slice(0, 10)}.json`;
        a.click();
        URL.revokeObjectURL(url);
      } catch (e) {
        alert('导出失败: ' + e.message);
      }
    });
    dataSection.appendChild(exportBtn);

    const importBtn = createEl('button', { className: 'btn btn-outline', textContent: '📤 导入数据' });
    const fileInput = createEl('input', { type: 'file', accept: '.json', style: { display: 'none' } });
    importBtn.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', async (e) => {
      try {
        const file = e.target.files[0];
        if (!file) return;
        const text = await file.text();
        const data = JSON.parse(text);

        if (data.memories) {
          for (const m of data.memories) {
            await sendToSW({ type: 'saveMemory', memory: m });
          }
        }
        if (data.skills) {
          for (const s of data.skills) {
            await sendToSW({ type: 'saveSkill', skill: s });
          }
        }
        if (data.knowledgeItems) {
          for (const i of data.knowledgeItems) {
            await sendToSW({ type: 'saveKnowledgeItem', item: i });
          }
        }
        if (data.knowledgeCategories) {
          for (const c of data.knowledgeCategories) {
            await sendToSW({ type: 'saveKnowledgeCategory', category: c });
          }
        }
        alert('导入成功!请刷新页面查看');
      } catch (e) {
        alert('导入失败: ' + e.message);
      }
    });
    dataSection.appendChild(fileInput);
    dataSection.appendChild(importBtn);

    const resetBtn = createEl('button', {
      className: 'btn btn-danger',
      textContent: '🗑️ 清空所有数据',
      style: { marginLeft: '8px' },
    });
    resetBtn.addEventListener('click', async () => {
      const confirmed = confirm('确定要清空所有数据吗?此操作不可撤销!\n\n请在弹出框再次确认。');
      if (!confirmed) return;
      const doubleConfirmed = confirm('再次确认:删除所有记忆, Skill 和知识库数据?');
      if (!doubleConfirmed) return;

      const [memories, skills, items, categories] = await Promise.all([
        sendToSW({ type: 'getMemories' }),
        sendToSW({ type: 'getSkills' }),
        sendToSW({ type: 'getKnowledgeItems' }),
        sendToSW({ type: 'getKnowledgeCategories' }),
      ]);

      await Promise.all([
        ...memories.map(m => sendToSW({ type: 'deleteMemory', id: m.id })),
        ...skills.map(s => sendToSW({ type: 'deleteSkill', id: s.id })),
        ...items.map(i => sendToSW({ type: 'deleteKnowledgeItem', id: i.id })),
        ...categories.map(c => sendToSW({ type: 'deleteKnowledgeCategory', id: c.id })),
      ]);

      alert('所有数据已清空');
    });
    dataSection.appendChild(resetBtn);
    wrapper.appendChild(dataSection);

    // --- 保存按钮 ---
    const saveBar = createEl('div', { style: { marginTop: '24px', textAlign: 'center' } });
    const saveBtn = createEl('button', { className: 'btn btn-primary', textContent: '💾 保存设置' });
    saveBtn.addEventListener('click', async () => {
      try {
        await sendToSW({
          type: 'saveSettings',
          settings: {
            token: apiKeyInput.value.trim(),
            autoSaveMemory: autoSaveCheckbox.checked,
          },
        });
        alert('设置已保存');
      } catch (e) {
        alert('保存失败: ' + e.message);
      }
    });
    saveBar.appendChild(saveBtn);
    wrapper.appendChild(saveBar);

    container.appendChild(wrapper);
  },
};

export default SettingsPanel;
