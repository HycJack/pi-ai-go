package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signintech/gopdf"
)

// pagesToPDF 把文本内容生成一个**可复制文本**的 PDF 文件。
func pagesToPDF(pages [][]byte, dataDir string, params HandwritingParams) (string, error) {
	_ = pages

	if strings.TrimSpace(params.Text) == "" {
		return "", fmt.Errorf("文本内容不能为空")
	}

	fontBytes, err := loadFontBytes(params.FontOption, params.FontFileBase64)
	if err != nil {
		return "", fmt.Errorf("加载字体失败: %w", err)
	}

	fontSize := params.FontSize
	if fontSize <= 0 {
		fontSize = 100
	}
	lineSpacing := params.LineSpacing
	if lineSpacing <= 0 {
		lineSpacing = int(float64(fontSize) * 1.5)
	}
	pageW := params.Width
	pageH := params.Height
	if pageW <= 0 {
		pageW = 2481
	}
	if pageH <= 0 {
		pageH = 3507
	}
	marginLeft := float64(params.MarginLeft)
	marginTop := float64(params.MarginTop)
	marginBottom := float64(params.MarginBottom)
	usableWidth := float64(pageW) - marginLeft - float64(params.MarginRight)

	text := strings.ReplaceAll(params.Text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var pdfObj gopdf.GoPdf
	pdfObj.Start(gopdf.Config{
		Unit:     gopdf.UnitPT,
		PageSize: gopdf.Rect{W: float64(pageW), H: float64(pageH)},
	})

	var container gopdf.FontContainer
	if err := container.AddTTFFontData("handwriting", fontBytes); err != nil {
		return "", fmt.Errorf("容器添加字体失败: %w", err)
	}
	if err := pdfObj.AddTTFFontFromFontContainer("handwriting", &container); err != nil {
		return "", fmt.Errorf("添加字体容器失败: %w", err)
	}
	if err := pdfObj.SetFont("handwriting", "", fontSize); err != nil {
		return "", fmt.Errorf("设置字体失败: %w", err)
	}

	pdfObj.AddPage()

	chunks := splitPagesForPDF(text)
	for _, chunk := range chunks {
		drawTextOnPDFPage(&pdfObj, chunk, fontSize, lineSpacing, marginLeft, marginTop, marginBottom, usableWidth, float64(pageH))
	}

	pdfPath := filepath.Join(dataDir, fmt.Sprintf("handwriting_%d.pdf", time.Now().Unix()))
	if err := os.WriteFile(pdfPath, pdfObj.GetBytesPdf(), 0644); err != nil {
		return "", fmt.Errorf("保存 PDF 失败: %w", err)
	}
	return pdfPath, nil
}

// splitPagesForPDF 按 --- 切分文本
func splitPagesForPDF(text string) []string {
	if !pageBreakRE.MatchString(text) {
		return []string{text}
	}
	parts := pageBreakRE.Split(text, -1)
	var chunks []string
	for _, p := range parts {
		p = strings.TrimPrefix(p, "\n")
		p = strings.TrimSuffix(p, "\n")
		if strings.TrimSpace(p) != "" {
			chunks = append(chunks, p)
		}
	}
	return chunks
}

// drawTextOnPDFPage 在当前 PDF 页绘制一段文本
func drawTextOnPDFPage(pdf *gopdf.GoPdf, text string, fontSize, lineSpacing int, marginLeft, marginTop, marginBottom, usableWidth, pageH float64) {
	lineHeight := float64(lineSpacing)
	curY := marginTop + float64(fontSize)

	lines := strings.Split(text, "\n")
	for _, rawLine := range lines {
		rightAlign := false
		line := rawLine
		if rightAlignRE.MatchString(line) {
			rightAlign = true
			line = rightAlignRE.ReplaceAllString(line, "")
		}

		if curY+lineHeight > pageH-marginBottom {
			pdf.AddPage()
			curY = marginTop + float64(fontSize)
		}

		wrapped := wrapTextByWidth(pdf, line, usableWidth)
		for _, wl := range wrapped {
			if curY+lineHeight > pageH-marginBottom {
				pdf.AddPage()
				curY = marginTop + float64(fontSize)
			}
			if rightAlign {
				w, _ := pdf.MeasureTextWidth(wl)
				pdf.SetX(marginLeft + usableWidth - w)
			} else {
				pdf.SetX(marginLeft)
			}
			pdf.SetY(curY)
			if err := pdf.Text(wl); err != nil {
				_ = err
			}
			curY += lineHeight
		}
	}
}

// wrapTextByWidth 按可用宽度切分行
func wrapTextByWidth(pdf *gopdf.GoPdf, text string, maxWidth float64) []string {
	if text == "" {
		return []string{""}
	}
	var result []string
	var cur strings.Builder
	curWidth := 0.0
	for _, r := range text {
		s := string(r)
		rw, _ := pdf.MeasureTextWidth(s)
		if curWidth+rw > maxWidth && cur.Len() > 0 {
			result = append(result, cur.String())
			cur.Reset()
			curWidth = 0
		}
		cur.WriteString(s)
		curWidth += rw
	}
	if cur.Len() > 0 {
		result = append(result, cur.String())
	}
	return result
}
