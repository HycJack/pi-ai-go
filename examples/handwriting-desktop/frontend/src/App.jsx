import React, { useState, useRef, useEffect } from "react";

// =============================================================================
// 手写文字生成器 — 主应用
// 参考 handwriting-web (Vue) 的功能设计 + 现代 Wails 桌面 UI
// =============================================================================

// Wails runtime（先检查全局绑定）
const Go = window?.go?.main?.App;
const EventsOn = window?.runtime?.EventsOn || (() => {});
const EventsOff = window?.runtime?.EventsOff || (() => {});

// 默认参数（匹配 handwriting-web）
const DEFAULTS = {
  fontSize: 80,
  lineSpacing: 120,
  width: 2481,
  height: 3507,
  marginTop: 100,
  marginBottom: 100,
  marginLeft: 100,
  marginRight: 100,
  wordSpacing: 1,
  lineSpacingSigma: 1,
  fontSizeSigma: 1,
  wordSpacingSigma: 2,
  perturbXSigma: 2,
  perturbYSigma: 2,
  perturbThetaSigma: 0.03,
  inkDepthSigma: 20,
  strikethroughProbability: 0.003,
  strikethroughLengthSigma: 2,
  strikethroughWidthSigma: 1.5,
  strikethroughAngleSigma: 2,
  strikethroughWidth: 6,
  isUnderlined: true,
  enableEnglishSpacing: false,
};

export default function App() {
  // ────── 参数状态 ──────
  const [text, setText] = useState("");
  const [fontSize, setFontSize] = useState(DEFAULTS.fontSize);
  const [lineSpacing, setLineSpacing] = useState(DEFAULTS.lineSpacing);
  const [width, setWidth] = useState(DEFAULTS.width);
  const [height, setHeight] = useState(DEFAULTS.height);
  const [marginTop, setMarginTop] = useState(DEFAULTS.marginTop);
  const [marginBottom, setMarginBottom] = useState(DEFAULTS.marginBottom);
  const [marginLeft, setMarginLeft] = useState(DEFAULTS.marginLeft);
  const [marginRight, setMarginRight] = useState(DEFAULTS.marginRight);
  const [wordSpacing, setWordSpacing] = useState(DEFAULTS.wordSpacing);
  const [isUnderlined, setIsUnderlined] = useState(DEFAULTS.isUnderlined);
  const [enableEnglishSpacing, setEnableEnglishSpacing] = useState(DEFAULTS.enableEnglishSpacing);
  const [isExpanded, setIsExpanded] = useState(false);

  // 高级参数
  const [adv, setAdv] = useState({
    lineSpacingSigma: DEFAULTS.lineSpacingSigma,
    fontSizeSigma: DEFAULTS.fontSizeSigma,
    wordSpacingSigma: DEFAULTS.wordSpacingSigma,
    perturbXSigma: DEFAULTS.perturbXSigma,
    perturbYSigma: DEFAULTS.perturbYSigma,
    perturbThetaSigma: DEFAULTS.perturbThetaSigma,
    inkDepthSigma: DEFAULTS.inkDepthSigma,
    strikethroughProbability: DEFAULTS.strikethroughProbability,
    strikethroughLengthSigma: DEFAULTS.strikethroughLengthSigma,
    strikethroughWidthSigma: DEFAULTS.strikethroughWidthSigma,
    strikethroughAngleSigma: DEFAULTS.strikethroughAngleSigma,
    strikethroughWidth: DEFAULTS.strikethroughWidth,
  });

  // 字体
  const [fonts, setFonts] = useState([]);
  const [selectedFont, setSelectedFont] = useState("LXGWWenKai-Regular.ttf");

  // 预览
  const [previewImages, setPreviewImages] = useState([]);
  const [previewUrls, setPreviewUrls] = useState([]);
  const [currentPage, setCurrentPage] = useState(0);

  // 全局状态
  const [isGenerating, setIsGenerating] = useState(false);
  const [message, setMessage] = useState({ text: "", type: "" });
  const [progress, setProgress] = useState(0);
  const [showAiPanel, setShowAiPanel] = useState(false);

  // AI Agent
  const [agentMsgs, setAgentMsgs] = useState([]);
  const [agentInput, setAgentInput] = useState("");
  const [agentStream, setAgentStream] = useState("");
  const agentBuf = useRef("");
  const chatEndRef = useRef(null);

  // 文本编辑器 Ref
  const textAreaRef = useRef(null);

  // ────── 生命周期 ──────
  useEffect(() => {
    loadFonts();
    loadSettings();

    EventsOn("handwriting-generated", (payload) => {
      try {
        const d = typeof payload === "string" ? JSON.parse(payload) : payload;
        if (d.images?.length) {
          const urls = d.images.map((b) => `data:image/png;base64,${b}`);
          setPreviewUrls(urls);
          setPreviewImages(d.images);
          setCurrentPage(0);
          showMsg(`已生成 ${d.images.length} 页`, "success");
        }
      } catch {}
    });
    EventsOn("agent-text-delta", (d) => {
      agentBuf.current += d;
      setAgentStream(agentBuf.current);
    });
    EventsOn("agent-done", () => {
      if (agentBuf.current) {
        setAgentMsgs((prev) => [...prev, { role: "assistant", content: agentBuf.current }]);
      }
      agentBuf.current = "";
      setAgentStream("");
    });
    EventsOn("agent-error", (e) => showMsg(`AI 错误: ${e}`, "error"));

    return () => {
      EventsOff("handwriting-generated");
      EventsOff("agent-text-delta");
      EventsOff("agent-done");
      EventsOff("agent-error");
    };
  }, []);

  useEffect(() => { chatEndRef.current?.scrollIntoView({ behavior: "smooth" }); }, [agentMsgs, agentStream]);

  // ────── 辅助 ──────
  const showMsg = (text, type = "info") => {
    setMessage({ text, type });
    setTimeout(() => setMessage({ text: "", type: "" }), 4000);
  };

  const loadFonts = async () => {
    try {
      const list = await Go?.GetFonts();
      if (list?.length) setFonts(list);
    } catch (e) { console.log("fonts:", e); }
  };

  const loadSettings = () => {
    const keys = [
      "fontSize", "lineSpacing", "width", "height",
      "marginTop", "marginBottom", "marginLeft", "marginRight",
      "wordSpacing", "isUnderlined", "enableEnglishSpacing",
      ...Object.keys(DEFAULTS).filter(k => k.startsWith("lineSpacingSigma") || k.startsWith("fontSize") || k.startsWith("wordSpacing") || k.startsWith("perturb") || k.startsWith("inkDepth") || k.startsWith("strikethrough")),
    ];
    // 简化：只恢复基本参数
    try {
      setFontSize(Number(localStorage.getItem("hw_fontSize")) || DEFAULTS.fontSize);
      setLineSpacing(Number(localStorage.getItem("hw_lineSpacing")) || DEFAULTS.lineSpacing);
      setWidth(Number(localStorage.getItem("hw_width")) || DEFAULTS.width);
      setHeight(Number(localStorage.getItem("hw_height")) || DEFAULTS.height);
      setMarginTop(Number(localStorage.getItem("hw_marginTop")) || DEFAULTS.marginTop);
      setMarginBottom(Number(localStorage.getItem("hw_marginBottom")) || DEFAULTS.marginBottom);
      setMarginLeft(Number(localStorage.getItem("hw_marginLeft")) || DEFAULTS.marginLeft);
      setMarginRight(Number(localStorage.getItem("hw_marginRight")) || DEFAULTS.marginRight);
      setWordSpacing(Number(localStorage.getItem("hw_wordSpacing")) || DEFAULTS.wordSpacing);
      setIsUnderlined(localStorage.getItem("hw_isUnderlined") !== "false");
      setEnableEnglishSpacing(localStorage.getItem("hw_enableEnglishSpacing") === "true");
      // 迁移：旧默认 GoRegular.ttf 不支持中文，强制切换到霞鹜文楷
      const savedFont = localStorage.getItem("hw_selectedFont");
      if (!savedFont || savedFont === "GoRegular.ttf" || savedFont === "NotoSansCJKsc-Regular.otf") {
        setSelectedFont("LXGWWenKai-Regular.ttf");
        saveSetting("selectedFont", "LXGWWenKai-Regular.ttf");
      } else {
        setSelectedFont(savedFont);
      }
    } catch {}
  };

  const saveSetting = (k, v) => {
    try { localStorage.setItem(`hw_${k}`, String(v)); } catch {}
  };

  const buildParams = (preview, exportPDF = false) => ({
    text,
    fontSize, lineSpacing, wordSpacing,
    fill: "0,0,0,255",
    width: width || 2481,
    height: height || 3507,
    marginTop, marginBottom, marginLeft, marginRight,
    ...adv,
    isUnderlined, enableEnglishSpacing,
    preview, fullPreview: true, exportPDF,
    fontOption: selectedFont,
    fontFileBase64: "", backgroundImageBase64: "",
  });

  const generate = async (preview, pdf = false) => {
    if (!text.trim()) { showMsg("请输入文本内容", "error"); return; }
    setIsGenerating(true);
    setProgress(10);
    showMsg(preview ? "正在生成预览..." : pdf ? "正在导出 PDF..." : "正在生成图片...", "info");

    try {
      const result = await Go?.Generate(JSON.stringify(buildParams(preview, pdf)));
      setProgress(100);
      if (result?.Error) { showMsg(`失败: ${result.Error}`, "error"); return; }
      if (preview && result?.images) {
        const urls = result.images.map((b) => `data:image/png;base64,${b}`);
        setPreviewUrls(urls);
        setPreviewImages(result.images);
        setCurrentPage(0);
        showMsg(`已生成 ${result.images.length} 页预览`, "success");
      } else if (result?.zipPath) {
        showMsg(`ZIP 已保存: ${result.zipPath}`, "success");
      } else if (result?.pdfPath) {
        showMsg(`PDF 已保存: ${result.pdfPath}`, "success");
      }
    } catch (e) {
      showMsg(`生成失败: ${e}`, "error");
    } finally {
      setIsGenerating(false);
      setProgress(0);
    }
  };

  const resetAll = () => {
    setFontSize(DEFAULTS.fontSize);
    setLineSpacing(DEFAULTS.lineSpacing);
    setWidth(DEFAULTS.width);
    setHeight(DEFAULTS.height);
    setMarginTop(DEFAULTS.marginTop);
    setMarginBottom(DEFAULTS.marginBottom);
    setMarginLeft(DEFAULTS.marginLeft);
    setMarginRight(DEFAULTS.marginRight);
    setWordSpacing(DEFAULTS.wordSpacing);
    setIsUnderlined(DEFAULTS.isUnderlined);
    setEnableEnglishSpacing(DEFAULTS.enableEnglishSpacing);
    setAdv({ ...DEFAULTS });
    showMsg("已重置所有参数", "info");
  };

  // ────── AI ──────
  const sendAgent = async () => {
    const msg = agentInput.trim();
    if (!msg) return;
    setAgentInput("");
    setAgentMsgs((prev) => [...prev, { role: "user", content: msg }]);
    setAgentStream("");
    agentBuf.current = "";

    try {
      await Go?.AgentMessage(JSON.stringify({
        message: msg,
        messages: agentMsgs.map((m) => ({ role: m.role, content: m.content })),
      }));
    } catch (e) { showMsg(`AI 请求失败: ${e}`, "error"); }
  };

  const clearAgent = () => {
    setAgentMsgs([]);
    setAgentStream("");
    agentBuf.current = "";
  };

  // ────── 模板文本 ──────
  const templateTexts = [
    { label: "请假条", text: "尊敬的老师：\n您好！\n由于身体不适，我需要请假一天，望批准。\n此致\n敬礼！\n>>>张三\n>>>2026年8月11日" },
    { label: "散文", text: "秋天的午后，阳光透过梧桐叶的缝隙洒下斑驳的光影。\n微风拂过，几片枯叶缓缓飘落，像时光的碎屑。\n远处传来学校的铃声，悠远而亲切。\n记忆里的秋天，总是带着桂花香和糖炒栗子的味道。\n日子就这样静静地流淌，不急不缓。" },
    { label: "诗词", text: "《静夜思》\n床前明月光，疑是地上霜。\n举头望明月，低头思故乡。\n---\n《春晓》\n春眠不觉晓，处处闻啼鸟。\n夜来风雨声，花落知多少。" },
  ];

  const applyTemplate = (t) => {
    setText(t.text);
    showMsg(`已加载「${t.label}」模板`, "info");
  };

  // ────── 渲染 ──────
  return (
    <div className="flex h-screen w-screen overflow-hidden bg-slate-100">

      {/* ─── 左侧面板 ─── */}
      <aside className="w-[400px] min-w-[380px] h-full flex flex-col bg-white border-r border-slate-200 shadow-sm z-10">
        {/* 顶栏 */}
        <header className="h-[52px] px-5 flex items-center gap-2.5 border-b border-slate-100">
          <svg className="w-5 h-5 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
          </svg>
          <h1 className="text-base font-bold text-slate-800">手写文字生成器</h1>
        </header>

        {/* 消息 */}
        {message.text && (
          <div className={`mx-4 mt-3 px-3 py-2 rounded-lg text-xs font-medium message-animate ${
            message.type === "error" ? "bg-red-50 text-red-600" :
            message.type === "success" ? "bg-emerald-50 text-emerald-600" :
            "bg-blue-50 text-blue-600"
          }`}>
            {message.text}
          </div>
        )}

        {/* 参数区（可滚动） */}
        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-5">

          {/* ── 文本输入 ── */}
          <section>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-xs font-semibold text-slate-600">文字内容</label>
            </div>
            <textarea
              ref={textAreaRef}
              className="w-full h-28 resize-none border border-slate-200 rounded-xl px-3.5 py-2.5 text-sm leading-relaxed outline-none transition-all focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 bg-white"
              placeholder="输入手写文字内容...
支持标记：
--- 独占一行 → 强制分页
>>> 行首 → 右对齐"
              value={text}
              onChange={(e) => setText(e.target.value)}
            />
            {/* 模板快速填充 */}
            <div className="flex gap-1.5 mt-2 flex-wrap">
              {templateTexts.map((t, i) => (
                <button key={i} className="btn btn-xs btn-secondary" onClick={() => applyTemplate(t)}>
                  {t.label}
                </button>
              ))}
            </div>
          </section>

          {/* ── 字体 ── */}
          <section>
            <label className="form-label">字体</label>
            <div className="flex gap-2">
              <select className="form-input-sm flex-1" value={selectedFont}
                onChange={(e) => { setSelectedFont(e.target.value); saveSetting("selectedFont", e.target.value); }}>
                {fonts.length > 0 ? fonts.map((f, i) => (
                  <option key={i} value={f.name}>{f.name}</option>
                )) : <option value="LXGWWenKai-Regular.ttf">LXGWWenKai-Regular.ttf (内置)</option>}
              </select>
            </div>
          </section>

          {/* ── 基本参数 ── */}
          <section className="grid grid-cols-2 gap-x-4 gap-y-3.5">
            <ParamSlider label="字号" val={fontSize} min={40} max={200} step={5}
              onChange={(v) => { setFontSize(v); saveSetting("fontSize", v); }} />
            <ParamSlider label="行间距" val={lineSpacing} min={50} max={300} step={5}
              onChange={(v) => { setLineSpacing(v); saveSetting("lineSpacing", v); }} />
            <ParamSlider label="字间距" val={wordSpacing} min={0} max={20} step={1}
              onChange={(v) => { setWordSpacing(v); saveSetting("wordSpacing", v); }} />
            <ParamSlider label="页面宽度" val={width} min={500} max={4000} step={50}
              onChange={(v) => { setWidth(v); saveSetting("width", v); }} />
            <ParamSlider label="页面高度" val={height} min={500} max={5000} step={50}
              onChange={(v) => { setHeight(v); saveSetting("height", v); }} />
          </section>

          {/* ── 边距 ── */}
          <section className="grid grid-cols-2 gap-x-4 gap-y-3">
            <ParamSlider label="上边距" val={marginTop} min={20} max={400} step={10}
              onChange={(v) => { setMarginTop(v); saveSetting("marginTop", v); }} />
            <ParamSlider label="下边距" val={marginBottom} min={20} max={400} step={10}
              onChange={(v) => { setMarginBottom(v); saveSetting("marginBottom", v); }} />
            <ParamSlider label="左边距" val={marginLeft} min={20} max={400} step={10}
              onChange={(v) => { setMarginLeft(v); saveSetting("marginLeft", v); }} />
            <ParamSlider label="右边距" val={marginRight} min={20} max={400} step={10}
              onChange={(v) => { setMarginRight(v); saveSetting("marginRight", v); }} />
          </section>

          {/* ── 选项开关 ── */}
          <section className="flex items-center gap-6">
            <ToggleSwitch label="下划线" checked={isUnderlined}
              onChange={(v) => { setIsUnderlined(v); saveSetting("isUnderlined", v); }} />
            <ToggleSwitch label="英文间距" checked={enableEnglishSpacing}
              onChange={(v) => { setEnableEnglishSpacing(v); saveSetting("enableEnglishSpacing", v); }} />
          </section>

          {/* ── 高级参数折叠 ── */}
          <section>
            <button className="flex items-center gap-1.5 text-xs font-medium text-indigo-500 hover:text-indigo-600 transition-colors"
              onClick={() => setIsExpanded(!isExpanded)}>
              <svg className={`w-3 h-3 transition-transform ${isExpanded ? "rotate-90" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
              {isExpanded ? "收起高级参数" : "展开高级参数"}
            </button>

            {isExpanded && (
              <div className="mt-3 p-4 bg-slate-50 rounded-xl border border-slate-100 space-y-4">
                <section className="grid grid-cols-2 gap-x-4 gap-y-3">
                  <ParamSlider label="行间距扰动" val={adv.lineSpacingSigma} min={0} max={10} step={0.1}
                    onChange={(v) => setAdv((p) => ({ ...p, lineSpacingSigma: v }))} />
                  <ParamSlider label="字号扰动" val={adv.fontSizeSigma} min={0} max={10} step={0.1}
                    onChange={(v) => setAdv((p) => ({ ...p, fontSizeSigma: v }))} />
                  <ParamSlider label="字间距扰动" val={adv.wordSpacingSigma} min={0} max={10} step={0.1}
                    onChange={(v) => setAdv((p) => ({ ...p, wordSpacingSigma: v }))} />
                  <ParamSlider label="X偏移扰动" val={adv.perturbXSigma} min={0} max={20} step={0.5}
                    onChange={(v) => setAdv((p) => ({ ...p, perturbXSigma: v }))} />
                  <ParamSlider label="Y偏移扰动" val={adv.perturbYSigma} min={0} max={20} step={0.5}
                    onChange={(v) => setAdv((p) => ({ ...p, perturbYSigma: v }))} />
                  <ParamSlider label="旋转扰动" val={adv.perturbThetaSigma} min={0} max={0.3} step={0.01}
                    onChange={(v) => setAdv((p) => ({ ...p, perturbThetaSigma: v }))} />
                  <ParamSlider label="墨水深浅" val={adv.inkDepthSigma} min={0} max={100} step={5}
                    onChange={(v) => setAdv((p) => ({ ...p, inkDepthSigma: v }))} />
                </section>

                <div className="border-t border-slate-200 pt-3">
                  <p className="text-xs font-semibold text-slate-500 mb-2">涂改痕迹（删除线）</p>
                  <section className="grid grid-cols-2 gap-x-4 gap-y-3">
                    <ParamSlider label="概率" val={adv.strikethroughProbability} min={0} max={0.05} step={0.001}
                      onChange={(v) => setAdv((p) => ({ ...p, strikethroughProbability: v }))} />
                    <ParamSlider label="线宽度" val={adv.strikethroughWidth} min={1} max={20} step={0.5}
                      onChange={(v) => setAdv((p) => ({ ...p, strikethroughWidth: v }))} />
                    <ParamSlider label="长度扰动" val={adv.strikethroughLengthSigma} min={0} max={10} step={0.1}
                      onChange={(v) => setAdv((p) => ({ ...p, strikethroughLengthSigma: v }))} />
                    <ParamSlider label="角度扰动" val={adv.strikethroughAngleSigma} min={0} max={10} step={0.1}
                      onChange={(v) => setAdv((p) => ({ ...p, strikethroughAngleSigma: v }))} />
                  </section>
                </div>
              </div>
            )}
          </section>

          {/* 占位，顶起底部 */}
          <div className="h-4" />
        </div>

        {/* ── 底部操作栏 ── */}
        <div className="px-5 py-4 border-t border-slate-100 space-y-2.5 bg-white">
          {/* 进度条 */}
          {isGenerating && progress > 0 && (
            <div className="w-full h-1.5 bg-slate-100 rounded-full overflow-hidden">
              <div className="h-full bg-indigo-500 rounded-full transition-all duration-500" style={{ width: `${progress}%` }} />
            </div>
          )}

          <div className="grid grid-cols-3 gap-2">
            <button className="btn btn-md btn-secondary" onClick={resetAll}>
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              重置
            </button>
            <button className="btn btn-md btn-primary" disabled={isGenerating} onClick={() => generate(true)}>
              {isGenerating ? (
                <><span className="animate-spin w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full" /> 生成中...</>
              ) : "预览"}
            </button>
            <button className="btn btn-md btn-success" disabled={isGenerating} onClick={() => generate(false)}>
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              生成图片
            </button>
          </div>
          <button className="btn btn-md w-full bg-violet-500 text-white border-violet-500 hover:bg-violet-600" disabled={isGenerating} onClick={() => generate(false, true)}>
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
            </svg>
            导出 PDF（可复制文本）
          </button>
        </div>
      </aside>

      {/* ─── 右侧预览区 ─── */}
      <main className="flex-1 h-full flex flex-col overflow-hidden bg-slate-100">
        {/* 预览顶栏 */}
        <header className="h-[52px] flex items-center justify-between px-6 bg-white border-b border-slate-200">
          <div className="flex items-center gap-3">
            <h2 className="text-sm font-semibold text-slate-700">手写预览</h2>
            {previewUrls.length > 0 && (
              <span className="text-xs text-slate-400">共 {previewUrls.length} 页</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {/* AI 切换 */}
            <button className={`btn btn-xs ${showAiPanel ? "bg-indigo-100 text-indigo-600 border-indigo-200" : "btn-ghost"}`}
              onClick={() => setShowAiPanel(!showAiPanel)}>
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611A48.309 48.309 0 0112 21c-2.773 0-5.491-.235-8.135-.687-1.718-.293-2.3-2.379-1.067-3.61L5 14.5" />
              </svg>
              AI
            </button>
          </div>
        </header>

        {/* 分页导航 */}
        {previewUrls.length > 1 && (
          <div className="flex items-center justify-center gap-3 py-2 bg-white border-b border-slate-100">
            <button className="btn btn-xs btn-secondary" disabled={currentPage === 0}
              onClick={() => setCurrentPage(Math.max(0, currentPage - 1))}>
              ← 上一页
            </button>
            <span className="text-xs text-slate-500 font-medium">
              {currentPage + 1} / {previewUrls.length}
            </span>
            <button className="btn btn-xs btn-secondary" disabled={currentPage === previewUrls.length - 1}
              onClick={() => setCurrentPage(Math.min(previewUrls.length - 1, currentPage + 1))}>
              下一页 →
            </button>
          </div>
        )}

        {/* 图片显示区 */}
        <div className="flex-1 flex items-start justify-center overflow-auto p-6 bg-slate-100">
          {previewUrls.length > 0 ? (
            <div className="bg-white rounded-xl shadow-lg overflow-hidden">
              <img
                src={previewUrls[currentPage]}
                alt={`第 ${currentPage + 1} 页`}
                className="max-w-full"
                style={{ maxHeight: "calc(100vh - 140px)", objectFit: "contain" }}
              />
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-slate-300">
              <svg className="w-20 h-20 mb-4 opacity-40" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={0.8} d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
              </svg>
              <p className="text-sm">输入文字，点击「预览」查看效果</p>
              <p className="text-xs mt-1">也可使用 AI 助手帮你撰写文本</p>
            </div>
          )}
        </div>
      </main>

      {/* ─── AI 聊天浮动面板 ─── */}
      {showAiPanel && (
        <div className="fixed bottom-4 right-4 w-[380px] h-[460px] bg-white rounded-2xl shadow-2xl border border-slate-200 flex flex-col z-20 overflow-hidden"
          style={{ animation: "slideIn 0.25s ease-out" }}>
          {/* 标题 */}
          <div className="px-4 py-3 bg-gradient-to-r from-indigo-500 to-purple-500 text-white flex items-center justify-between">
            <div className="flex items-center gap-2">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
              </svg>
              <span className="text-sm font-semibold">AI 写作助手</span>
            </div>
            <div className="flex items-center gap-1">
              <button className="btn btn-ghost text-white/80 hover:text-white p-1" onClick={clearAgent}>
                <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
              <button className="btn btn-ghost text-white/80 hover:text-white p-1" onClick={() => setShowAiPanel(false)}>
                <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          {/* 消息列表 */}
          <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
            {agentMsgs.length === 0 && !agentStream && (
              <div className="text-center text-slate-400 text-xs py-8">
                <p>让 AI 帮你撰写或润色文本</p>
                <p className="mt-1">输入需求，AI 会帮你写好并填入文本区</p>
              </div>
            )}
            {agentMsgs.map((m, i) => (
              <div key={i} className={`flex ${m.role === "user" ? "justify-end" : "justify-start"}`}>
                {m.role === "assistant" && (
                  <div className="w-6 h-6 rounded-full bg-gradient-to-br from-indigo-400 to-purple-400 flex items-center justify-center text-white text-[10px] font-bold mr-2 flex-shrink-0 mt-1">AI</div>
                )}
                <div className={`max-w-[80%] ${m.role === "user" ? "bubble-user" : "bubble-assistant"}`}>
                  {m.content}
                </div>
              </div>
            ))}
            {agentStream && (
              <div className="flex justify-start">
                <div className="w-6 h-6 rounded-full bg-gradient-to-br from-indigo-400 to-purple-400 flex items-center justify-center text-white text-[10px] font-bold mr-2 flex-shrink-0 mt-1">AI</div>
                <div className="bubble-assistant max-w-[80%]">
                  {agentStream}
                  <span className="cursor-blink ml-0.5">▌</span>
                </div>
              </div>
            )}
            <div ref={chatEndRef} />
          </div>

          {/* 输入框 */}
          <div className="px-4 py-3 border-t border-slate-100">
            <div className="flex gap-2">
              <textarea className="flex-1 resize-none text-sm border border-slate-200 rounded-xl px-3 py-2 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
                rows={2} placeholder="输入需求，如：写一封请假信..."
                value={agentInput}
                onChange={(e) => setAgentInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendAgent(); } }}
              />
              <button className="btn btn-sm btn-primary self-end px-3" onClick={sendAgent}>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── 子组件 ───

function ParamSlider({ label, val, min, max, step, onChange }) {
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <label className="text-[11px] text-slate-500">{label}</label>
        <span className="text-[11px] font-semibold text-slate-600 tabular-nums">{val}</span>
      </div>
      <input type="range" min={min} max={max} step={step} value={val}
        onChange={(e) => onChange(parseFloat(e.target.value))} />
    </div>
  );
}

function ToggleSwitch({ label, checked, onChange }) {
  return (
    <label className="flex items-center gap-2.5 cursor-pointer select-none">
      <div className={`toggle-checkbox ${checked ? "active" : ""}`}
        onClick={() => onChange(!checked)} />
      <span className="text-xs text-slate-600">{label}</span>
    </label>
  );
}
