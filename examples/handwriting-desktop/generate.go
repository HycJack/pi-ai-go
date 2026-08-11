package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// 字体管理
// ============================================================================

var (
	builtinFontsMu   sync.Mutex
	builtinFontsOnce  bool
	builtinFontsList  []FontInfo
)

// releaseBuiltinFonts 把内置字体释放到字体目录（首次运行）。
// 这里我们只放一个 goregular 作为示例，用户可自行添加 TTF。
func (a *App) releaseBuiltinFonts() {
	builtinFontsMu.Lock()
	defer builtinFontsMu.Unlock()
	if builtinFontsOnce {
		return
	}
	builtinFontsOnce = true

	// 写入内置 goregular 字体
	target := filepath.Join(a.fontAssetsDir, "GoRegular.ttf")
	if _, err := os.Stat(target); os.IsNotExist(err) {
		// 使用 goregular.TTF
		if err := os.WriteFile(target, goregularTTF, 0644); err != nil {
			LogError("[fonts] failed to write GoRegular.ttf: %v", err)
		}
	}
	a.refreshFontList()
}

// refreshFontList 扫描字体目录，更新内置字体列表。
func (a *App) refreshFontList() {
	entries, err := os.ReadDir(a.fontAssetsDir)
	if err != nil {
		LogWarn("[fonts] read dir failed: %v", err)
		return
	}
	var list []FontInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".ttf") || strings.HasSuffix(lower, ".otf") {
			list = append(list, FontInfo{
				Name: name,
				Path: filepath.Join(a.fontAssetsDir, name),
			})
		}
	}
	builtinFontsList = list
}

// GetFonts 返回可用字体列表。
func (a *App) GetFonts() []FontInfo {
	builtinFontsMu.Lock()
	defer builtinFontsMu.Unlock()
	a.refreshFontList()
	return builtinFontsList
}

// SaveFont 让前端上传字体文件（base64），保存到字体目录。
func (a *App) SaveFont(name string, base64Data string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("字体名不能为空")
	}
	// 防止路径穿越
	name = filepath.Base(name)
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("解析字体 base64 失败: %w", err)
	}
	target := filepath.Join(a.fontAssetsDir, name)
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", fmt.Errorf("保存字体失败: %w", err)
	}
	a.refreshFontList()
	LogInfo("[fonts] saved font: %s (%d bytes)", name, len(data))
	return target, nil
}

// DeleteFont 删除指定字体文件。
func (a *App) DeleteFont(name string) error {
	name = filepath.Base(name)
	target := filepath.Join(a.fontAssetsDir, name)
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("删除字体失败: %w", err)
	}
	a.refreshFontList()
	return nil
}

// ============================================================================
// 生成手写图片
// ============================================================================

// Generate 是前端调用的主入口：根据参数生成手写图片。
// 通过 Wails 事件实时推送进度。
func (a *App) Generate(jsonStr string) (*GenerateResult, error) {
	var params HandwritingParams
	if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
		LogError("[generate] parse error: %v", err)
		return &GenerateResult{Error: fmt.Sprintf("参数解析失败: %v", err)}, nil
	}

	LogInfo("[generate] text=%d chars fontSize=%d preview=%v pdf=%v",
		len(params.Text), params.FontSize, params.Preview, params.ExportPDF)

	// 参数校验
	if strings.TrimSpace(params.Text) == "" {
		return &GenerateResult{Error: "文本内容不能为空"}, nil
	}

	// 推送开始事件
	runtime.EventsEmit(a.ctx, "generate-progress", map[string]interface{}{
		"stage": "rendering", "message": "正在生成手写图像", "progress": 10,
	})

	// 构建渲染器
	renderer, err := NewRenderer(params)
	if err != nil {
		LogError("[generate] renderer error: %v", err)
		return &GenerateResult{Error: err.Error()}, nil
	}

	// 渲染所有页面
	pages, err := renderer.Render()
	if err != nil {
		LogError("[generate] render error: %v", err)
		return &GenerateResult{Error: err.Error()}, nil
	}

	LogInfo("[generate] rendered %d pages", len(pages))

	runtime.EventsEmit(a.ctx, "generate-progress", map[string]interface{}{
		"stage": "packaging", "message": "正在打包结果", "progress": 80,
	})

	// 预览模式：返回 base64 图片数组
	if params.Preview {
		// 单页预览：只返回第一页
		if !params.FullPreview && len(pages) > 0 {
			runtime.EventsEmit(a.ctx, "generate-progress", map[string]interface{}{
				"stage": "finalizing", "message": "正在返回预览结果", "progress": 100,
			})
			return &GenerateResult{Images: []string{pagesToBase64(pages[:1])[0]}}, nil
		}
		// 完整预览：返回所有页
		runtime.EventsEmit(a.ctx, "generate-progress", map[string]interface{}{
			"stage": "finalizing", "message": "正在返回预览结果", "progress": 100,
		})
		return &GenerateResult{Images: pagesToBase64(pages)}, nil
	}

	// PDF 导出
	if params.ExportPDF {
		pdfPath, err := pagesToPDF(pages, a.dataDir, params)
		if err != nil {
			return &GenerateResult{Error: fmt.Sprintf("生成 PDF 失败: %v", err)}, nil
		}
		runtime.EventsEmit(a.ctx, "generate-progress", map[string]interface{}{
			"stage": "finalizing", "message": "正在返回 PDF 结果", "progress": 100,
		})
		return &GenerateResult{PDFPath: pdfPath}, nil
	}

	// 完整模式：打包 ZIP
	zipBytes, err := pagesToZIP(pages)
	if err != nil {
		return &GenerateResult{Error: fmt.Sprintf("打包 ZIP 失败: %v", err)}, nil
	}
	zipPath := filepath.Join(a.dataDir, fmt.Sprintf("handwriting_%d.zip", time.Now().Unix()))
	if err := os.WriteFile(zipPath, zipBytes, 0644); err != nil {
		return &GenerateResult{Error: fmt.Sprintf("保存 ZIP 失败: %v", err)}, nil
	}

	runtime.EventsEmit(a.ctx, "generate-progress", map[string]interface{}{
		"stage": "finalizing", "message": "正在返回 ZIP 结果", "progress": 100,
	})
	return &GenerateResult{ZipPath: zipPath}, nil
}

// SaveBase64Image 把 base64 图片保存到临时文件，供前端下载。
func (a *App) SaveBase64Image(base64Data string, filename string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("解析 base64 失败: %w", err)
	}
	filename = filepath.Base(filename)
	target := filepath.Join(a.dataDir, filename)
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}
	return target, nil
}
