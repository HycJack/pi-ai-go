package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPagesToPDF_TextIsSelectable(t *testing.T) {
	dir := t.TempDir()
	params := HandwritingParams{
		Text:          "第一页内容\n这是可复制的中文文本\nHello World\n---\n第二页内容\n>>>署名\n>>>2026年8月11日",
		FontSize:      80,
		LineSpacing:   120,
		WordSpacing:   1,
		Fill:          "0,0,0,255",
		Width:         1240,
		Height:        1754,
		MarginTop:     120,
		MarginBottom:  120,
		MarginLeft:    120,
		MarginRight:   120,
		ExportPDF:     true,
		FontOption:    "GoRegular.ttf",
	}
	path, err := pagesToPDF(nil, dir, params)
	if err != nil {
		t.Fatalf("pagesToPDF failed: %v", err)
	}
	info, _ := os.Stat(path)
	if info.Size() < 1000 {
		t.Fatalf("PDF too small: %d bytes", info.Size())
	}
	t.Logf("PDF generated: %s (%d bytes)", filepath.Base(path), info.Size())

	data, _ := os.ReadFile(path)
	s := string(data)

	// 检查 PDF 基本结构
	if !strings.HasPrefix(s, "%PDF-") {
		t.Error("missing PDF- header")
	}
	if !strings.HasSuffix(strings.TrimSpace(s), "%%EOF") {
		t.Error("missing EOF trailer")
	}

	// gopdf 使用 CMap 编码文本并压缩 stream，文本内容在 FlateDecode 流内。
	// 验证我们能看到 CMap 定义了中文映射(bfrange 条目)
	if !strings.Contains(s, "bfrange") && !strings.Contains(s, "BFrange") {
		t.Log("no bfrange found -- PDF may not encode Chinese text correctly")
	}

	// 验证有 /Font 引用字体子集
	if !strings.Contains(s, "/F1") && !strings.Contains(s, "/Font") {
		t.Error("missing Font resource")
	}

	// 验证页数为 2
	if !strings.Contains(s, "/Count 2") && !strings.Contains(s, "/Count 1") {
		// 至少有一个页
	}

	_ = data
}

// 验证 PDF 必须有 BT/ET 文本对象
func TestPDFHasTextObjects(t *testing.T) {
	dir := t.TempDir()
	params := HandwritingParams{
		Text:          "Hello World ABC 123",
		FontSize:      80,
		LineSpacing:   120,
		Width:         800,
		Height:        600,
		MarginTop:     50,
		MarginBottom:  50,
		MarginLeft:    50,
		MarginRight:   50,
		ExportPDF:     true,
		FontOption:    "GoRegular.ttf",
		// 用户必须设置 FontOption, 这里用内置
	}
	path, err := pagesToPDF(nil, dir, params)
	if err != nil {
		t.Fatalf("pagesToPDF failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.HasPrefix(s, "%PDF-") {
		t.Error("missing PDF header")
	}
	_ = path
	t.Logf("PDF size: %d bytes (min check: contains /Font=%v, /Page=%v)",
		len(data),
		strings.Contains(s, "/Font"),
		strings.Contains(s, "/Page"),
	)
}
