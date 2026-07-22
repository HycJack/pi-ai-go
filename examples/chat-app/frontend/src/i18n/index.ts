/**
 * i18n.ts — 简单的中英文国际化工具
 * 默认中文，支持切换到英文
 */

export type Locale = 'zh' | 'en';

export const LOCALE_LABELS: { value: Locale; label: string }[] = [
  { value: 'zh', label: '中文' },
  { value: 'en', label: 'English' },
];

/** 翻译键 → 各语言文本 */
type Dict = Record<string, { zh: string; en: string }>;

const dict: Dict = {
  // App 通用
  'app.title': { zh: 'Pi-AI Chat', en: 'Pi-AI Chat' },
  'app.ready': { zh: '就绪', en: 'Ready' },
  'app.newChat': { zh: '新建对话', en: 'New chat' },
  'app.search': { zh: '搜索对话…', en: 'Search conversations…' },
  'app.settings': { zh: '设置', en: 'Settings' },
  'app.collapseSidebar': { zh: '收起侧边栏', en: 'Collapse sidebar' },
  'app.expandSidebar': { zh: '展开侧边栏', en: 'Expand sidebar' },
  'app.runOnboarding': { zh: '重新引导', en: 'Run onboarding' },
  'app.onboardingDone': { zh: '设置已完成，开始使用吧！', en: 'Setup complete. Ready to go!' },

  // 输入框
  'input.placeholder': { zh: '输入消息…', en: 'Message Pi-AI…' },
  'input.send': { zh: '发送', en: 'Send' },
  'input.stop': { zh: '停止', en: 'Stop' },
  'input.generating': { zh: '生成中…', en: 'Generating…' },
  'input.captureScreen': { zh: '截屏', en: 'Capture screenshot' },
  'input.capturing': { zh: '截图中…', en: 'Capturing…' },
  'input.attachImage': { zh: '添加图片', en: 'Attach image' },
  'input.selectRegion': { zh: '选择截图区域', en: 'Select screenshot region' },
  'input.captureFull': { zh: '全屏截图', en: 'Full screen' },
  'input.captureRegion': { zh: '区域截图', en: 'Region' },
  'input.cancelCapture': { zh: '取消', en: 'Cancel' },
  'input.confirmCapture': { zh: '确认截图', en: 'Confirm' },

  // 消息
  'msg.you': { zh: '你', en: 'You' },
  'msg.assistant': { zh: '助手', en: 'Assistant' },
  'msg.copy': { zh: '复制', en: 'Copy' },
  'msg.copied': { zh: '已复制', en: 'Copied' },
  'msg.speak': { zh: '朗读', en: 'Speak' },
  'msg.stopSpeak': { zh: '停止朗读', en: 'Stop speaking' },
  'msg.canceled': { zh: '已停止生成', en: 'Generation canceled' },
  'msg.thinking': { zh: '思考中…', en: 'Thinking…' },
  'msg.generating': { zh: '生成中…', en: 'generating…' },

  // 设置面板
  'settings.title': { zh: '设置', en: 'Settings' },
  'settings.general': { zh: '通用', en: 'General' },
  'settings.system': { zh: '系统', en: 'System' },
  'settings.providers': { zh: '提供者', en: 'Providers' },
  'settings.addProvider': { zh: '添加提供者', en: 'Add Provider' },
  'settings.noProviders': { zh: '暂无提供者配置。点击"添加提供者"开始。', en: 'No providers configured. Click "Add Provider" to get started.' },
  'settings.providerType': { zh: '类型', en: 'Type' },
  'settings.displayName': { zh: '显示名称', en: 'Display Name' },
  'settings.displayNameOptional': { zh: '显示名称（可选）', en: 'Display Name (optional)' },
  'settings.baseUrl': { zh: '基础 URL', en: 'Base URL' },
  'settings.apiKey': { zh: 'API 密钥', en: 'API Key' },
  'settings.apiKeyPool': { zh: '密钥池', en: 'API Keys Pool' },
  'settings.addKey': { zh: '添加密钥', en: 'Add Key' },
  'settings.removeKey': { zh: '删除密钥', en: 'Remove key' },
  'settings.removeProvider': { zh: '删除提供者', en: 'Remove provider' },
  'settings.active': { zh: '当前', en: 'Active' },
  'settings.model': { zh: '模型', en: 'Model' },
  'settings.selectModel': { zh: '选择模型', en: 'Select a model' },
  'settings.refreshModels': { zh: '刷新模型列表', en: 'Refresh models' },
  'settings.maxTokens': { zh: '最大 Token', en: 'Max Tokens' },
  'settings.temperature': { zh: '温度', en: 'Temperature' },
  'settings.tts': { zh: '语音朗读', en: 'Text-to-Speech' },
  'settings.enableTts': { zh: '启用语音朗读', en: 'Enable TTS' },
  'settings.ttsVoice': { zh: '语音', en: 'Voice' },
  'settings.agentMode': { zh: 'Agent 模式', en: 'Agent Mode' },
  'settings.enableAgent': { zh: '启用 Agent 模式', en: 'Enable agent mode' },
  'settings.autoLearn': { zh: '自动学习', en: 'Auto Learn' },
  'settings.autoLearnDesc': { zh: '自动从对话中学习', en: 'Automatically learn from conversations' },
  'settings.autoCompact': { zh: '自动压缩', en: 'Auto Compact' },
  'settings.autoCompactDesc': { zh: '自动压缩上下文', en: 'Automatically compact context' },
  'settings.skillsDir': { zh: '技能目录', en: 'Skills Directory' },
  'settings.browse': { zh: '浏览', en: 'Browse' },
  'settings.save': { zh: '保存', en: 'Save' },
  'settings.saved': { zh: '已保存', en: 'Saved' },
  'settings.cancel': { zh: '取消', en: 'Cancel' },
  'settings.add': { zh: '添加', en: 'Add' },
  'settings.language': { zh: '语言', en: 'Language' },

  // Onboarding
  'onboarding.welcomeTitle': { zh: '欢迎使用 Pi-AI Chat', en: 'Welcome to Pi-AI Chat' },
  'onboarding.welcomeSubtitle': { zh: '选择语言开始使用', en: 'Choose your language to get started.' },
  'onboarding.getStarted': { zh: '开始使用', en: 'Get Started' },
  'onboarding.next': { zh: '下一步', en: 'Next' },
  'onboarding.back': { zh: '上一步', en: 'Back' },
  'onboarding.finish': { zh: '完成', en: 'Finish' },
  'onboarding.skip': { zh: '跳过', en: 'Skip' },

  // 空状态
  'empty.title': { zh: '开始你的第一次对话', en: 'Start your first conversation' },
  'empty.subtitle': { zh: '输入消息或截图开始', en: 'Type a message or capture a screenshot to begin' },
  'empty.heroTitle': { zh: '你好，我是 Pi-AI。', en: "Hi, I'm Pi-AI." },
  'empty.heroSubtitle': { zh: '今天我能帮你做什么？', en: 'How can I help you today?' },

  // 快捷建议
  'suggestion.quantum': { zh: '解释量子计算', en: 'Explain quantum computing' },
  'suggestion.poem': { zh: '写一首诗', en: 'Write a poem' },
  'suggestion.coding': { zh: '编程帮助', en: 'Help with coding' },
  'suggestion.trip': { zh: '规划旅行', en: 'Plan a trip' },
  'suggestion.learn': { zh: '学点新东西', en: 'Learn something new' },
  'suggestion.ideas': { zh: '生成创意', en: 'Generate ideas' },

  // 面包屑
  'breadcrumb.conversations': { zh: '对话', en: 'Conversations' },
  'breadcrumb.contextStats': { zh: '上下文统计', en: 'Context stats' },

  // 删除对话确认
  'chat.deleteConfirm': { zh: '确定删除此对话？', en: 'Delete this chat?' },
  'chat.deleteYes': { zh: '删除', en: 'Delete' },
  'chat.deleteNo': { zh: '取消', en: 'Cancel' },
  'chat.delete': { zh: '删除对话', en: 'Delete chat' },

  // 滚动到底部
  'chat.jumpToLatest': { zh: '跳转到最新', en: 'Jump to latest' },
  'chat.newUpdate': { zh: '条新更新', en: 'new update' },
  'chat.newUpdates': { zh: '条新更新', en: 'new updates' },

  // 侧边栏
  'sidebar.ready': { zh: '就绪', en: 'Ready' },
  'sidebar.history': { zh: '历史记录', en: 'History' },
  'sidebar.noConversations': { zh: '暂无对话', en: 'No conversations yet' },
  'sidebar.workingDir': { zh: '工作目录', en: 'Working directory' },
  'sidebar.notSet': { zh: '未设置', en: 'Not set' },
  'sidebar.agent': { zh: 'Pi-AI Agent', en: 'Pi-AI Agent' },

  // 思考深度
  'thinking.off': { zh: '关闭', en: 'Off' },
  'thinking.low': { zh: '低', en: 'Low' },
  'thinking.medium': { zh: '中', en: 'Medium' },
  'thinking.high': { zh: '高', en: 'High' },
  'thinking.depth': { zh: '思考深度', en: 'Thinking depth' },
  'thinking.title': { zh: '思考', en: 'Thinking' },

  // 工具调用
  'tool.calls': { zh: '工具调用', en: 'Tool calls' },
  'tool.running': { zh: '运行中', en: 'running' },
  'tool.done': { zh: '完成', en: 'done' },

  // 输入框补充
  'input.selectModel': { zh: '选择模型', en: 'Select model' },
  'input.switchModel': { zh: '切换模型', en: 'Switch model' },
  'input.currentModel': { zh: '当前模型', en: 'Current model' },
  'input.removeAttachment': { zh: '移除附件', en: 'Remove attachment' },
  'input.altPreview': { zh: '预览', en: 'preview' },
  'input.altAttachment': { zh: '附件', en: 'attachment' },
  'input.altImage': { zh: '图片', en: 'image' },

  // 代码块
  'code.copy': { zh: '复制', en: 'Copy' },
  'code.copied': { zh: '已复制', en: 'Copied' },
  'code.viewSource': { zh: '查看代码', en: 'View source' },
  'code.viewPreview': { zh: '查看预览', en: 'View preview' },
  'code.htmlPreview': { zh: 'HTML 预览', en: 'HTML Preview' },
  'code.mermaidLoading': { zh: '正在渲染图表…', en: 'Rendering diagram...' },
  'code.mermaidError': { zh: 'Mermaid 渲染错误', en: 'Mermaid render error' },
  'code.clickToExpand': { zh: '点击放大', en: 'Click to expand' },

  // 设置面板补充
  'settings.close': { zh: '关闭', en: 'Close' },
  'settings.languageLabel': { zh: '语言 / Language', en: 'Language' },
  'settings.runOnboarding': { zh: '重新运行引导', en: 'Run onboarding' },
  'settings.keysConfigured': { zh: '个密钥已配置', en: 'keys configured' },
  'settings.useSingleKey': { zh: '留空则使用上方的单个 API 密钥', en: 'Leave empty to use single API key above' },
  'settings.about': { zh: '关于', en: 'About' },
  'settings.aboutText': { zh: 'Pi-AI Chat v0.1.0 — 基于 Go + React 构建的多提供者 AI 聊天界面。', en: 'Pi-AI Chat v0.1.0 — Multi-provider AI chat interface built with Go + React.' },
  'settings.savedExclaim': { zh: '已保存！', en: 'Saved!' },

  // 记忆
  'memory.title': { zh: '记忆', en: 'Memory' },
  'memory.empty': { zh: '暂无记忆条目', en: 'No memory entries' },
  'memory.add': { zh: '添加记忆', en: 'Add Memory' },
  'memory.addBtn': { zh: '添加', en: 'Add' },
  'memory.delete': { zh: '删除', en: 'Delete' },
  'memory.close': { zh: '关闭', en: 'Close' },
  'memory.key': { zh: '键', en: 'Key' },
  'memory.value': { zh: '值', en: 'Value' },
  'memory.category': { zh: '分类（可选）', en: 'Category (optional)' },
  'memory.required': { zh: '键和值不能为空', en: 'Key and value are required' },
};

/** 当前语言（模块级单例，默认中文） */
let currentLocale: Locale = 'zh';

const listeners = new Set<() => void>();

export function getLocale(): Locale {
  return currentLocale;
}

export function setLocale(locale: Locale) {
  if (currentLocale === locale) return;
  currentLocale = locale;
  listeners.forEach((fn) => fn());
}

/** 翻译函数：t('app.title') → 当前语言文本 */
export function t(key: string): string {
  const entry = dict[key];
  if (!entry) return key;
  return entry[currentLocale] ?? entry.zh ?? key;
}

/** 订阅语言变更，返回取消订阅函数 */
export function subscribeLocaleChange(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/** React hook：获取翻译函数，语言变更时触发重渲染 */
import { useSyncExternalStore } from 'react';
export function useT(): (key: string) => string {
  useSyncExternalStore(subscribeLocaleChange, getLocale, getLocale);
  return t;
}

/** 从 localStorage 恢复语言偏好 */
export function loadLocaleFromStorage(): Locale {
  try {
    const saved = localStorage.getItem('pi-ai-locale');
    if (saved === 'zh' || saved === 'en') {
      currentLocale = saved;
    }
  } catch { /* ignore */ }
  return currentLocale;
}

/** 保存语言偏好到 localStorage */
export function saveLocaleToStorage(locale: Locale) {
  try {
    localStorage.setItem('pi-ai-locale', locale);
  } catch { /* ignore */ }
}
