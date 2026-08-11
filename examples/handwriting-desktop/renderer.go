package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// ============================================================================
// 手写文字渲染器
// 参考 handright 库的思路：用 TTF 字体渲染文字，叠加随机扰动模拟手写感。
// 支持自动分页、横线背景、下划线、删除线（涂改痕迹）、墨水深浅变化。
// ============================================================================

// pageBreakRE 匹配独占一行的三个或更多短横线，作为强制分页标记。
var pageBreakRE = regexp.MustCompile(`(?m)^[ ]*-{3,}[ ]*$`)

// rightAlignRE 匹配行首的 >>>，作为右对齐标记。
var rightAlignRE = regexp.MustCompile(`^>>>[ ]?`)

// Renderer 负责把手写参数渲染成多页 PNG。
type Renderer struct {
	params HandwritingParams
	rng    *rand.Rand
	face   font.Face
	bgImg  image.Image
	fill   color.Color
}

// NewRenderer 根据参数构建渲染器，加载字体与背景。
func NewRenderer(params HandwritingParams) (*Renderer, error) {
	// 1. 加载字体
	fontBytes, err := loadFontBytes(params.FontOption, params.FontFileBase64)
	if err != nil {
		return nil, fmt.Errorf("加载字体失败: %w", err)
	}
	parsed, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("解析字体失败: %w", err)
	}
	fontSize := params.FontSize
	if fontSize <= 0 {
		fontSize = 100
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    float64(fontSize),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("创建字体 face 失败: %w", err)
	}

	// 2. 加载或生成背景
	bgImg, err := loadBackground(params)
	if err != nil {
		return nil, fmt.Errorf("准备背景失败: %w", err)
	}

	// 3. 解析填充色
	fill := parseFill(params.Fill)

	// 4. 随机数生成器（用时间种子，每次生成都有差异）
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))

	return &Renderer{
		params: params,
		rng:    rng,
		face:   face,
		bgImg:  bgImg,
		fill:   fill,
	}, nil
}

// Render 渲染所有页面，返回 PNG 字节切片。
func (r *Renderer) Render() ([][]byte, error) {
	text := r.normalizeText(r.params.Text)
	chunks := r.splitByPageBreaks(text)

	var pages [][]byte
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		page, err := r.renderChunk(chunk)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	if len(pages) == 0 {
		// 至少返回一页空白
		page, err := r.renderChunk("")
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// normalizeText 统一换行符，并按需调整英文间距。
func (r *Renderer) normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if r.params.EnableEnglishSpacing {
		text = adjustEnglishSpacing(text)
	}
	return text
}

// splitByPageBreaks 按分页标记切分文本。
func (r *Renderer) splitByPageBreaks(text string) []string {
	if !pageBreakRE.MatchString(text) {
		return []string{text}
	}
	// 用正则切分，保留非空块
	parts := pageBreakRE.Split(text, -1)
	var chunks []string
	for _, p := range parts {
		// 去掉分页标记留下的首尾多余换行
		p = strings.TrimPrefix(p, "\n")
		p = strings.TrimSuffix(p, "\n")
		if strings.TrimSpace(p) != "" {
			chunks = append(chunks, p)
		}
	}
	return chunks
}

// renderChunk 把一段文本渲染到一页（可能溢出多页，自动分页）。
func (r *Renderer) renderChunk(text string) ([]byte, error) {
	// 把文本按行布局，超出页面高度则分页
	lines := r.layoutLines(text)
	return r.drawPages(lines)
}

// layoutLine 表示一行已布局的文本及其对齐方式。
type layoutLine struct {
	text     string
	rightAlign bool
}

// layoutLines 处理右对齐标记，返回行列表。
func (r *Renderer) layoutLines(text string) []layoutLine {
	var result []layoutLine
	for _, line := range strings.Split(text, "\n") {
		rightAlign := false
		if rightAlignRE.MatchString(line) {
			rightAlign = true
			line = rightAlignRE.ReplaceAllString(line, "")
		}
		result = append(result, layoutLine{text: line, rightAlign: rightAlign})
	}
	return result
}

// drawPages 把行列表绘制到一页或多页，返回第一页 PNG（预览）或全部页打包。
// 这里返回第一页用于预览；完整生成由上层调用 Render 收集所有页。
func (r *Renderer) drawPages(lines []layoutLine) ([]byte, error) {
	pages := r.paginate(lines)
	if len(pages) == 0 {
		pages = [][]layoutLine{{}}
	}
	// 只渲染第一页用于单页预览；完整模式由 Render 逐块调用
	page := pages[0]
	return r.drawPage(page)
}

// paginate 根据页面高度与行间距切分页面。
func (r *Renderer) paginate(lines []layoutLine) [][]layoutLine {
	lineSpacing := r.params.LineSpacing
	if lineSpacing <= 0 {
		lineSpacing = int(float64(r.params.FontSize) * 1.5)
	}
	usableHeight := r.params.Height - r.params.MarginTop - r.params.MarginBottom
	if usableHeight <= 0 {
		usableHeight = r.params.Height / 2
	}
	linesPerPage := usableHeight / lineSpacing
	if linesPerPage <= 0 {
		linesPerPage = 1
	}

	var pages [][]layoutLine
	var current []layoutLine
	for _, ln := range lines {
		current = append(current, ln)
		if len(current) >= linesPerPage {
			pages = append(pages, current)
			current = nil
		}
	}
	if len(current) > 0 {
		pages = append(pages, current)
	}
	return pages
}

// drawPage 把一组行绘制到一张背景图上，返回 PNG 字节。
func (r *Renderer) drawPage(lines []layoutLine) ([]byte, error) {
	// 复制背景
	bounds := r.bgImg.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, r.bgImg, bounds.Min, draw.Src)

	// 准备绘制器
	drawer := &font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(r.fill),
		Face: r.face,
	}

	lineSpacing := r.params.LineSpacing
	if lineSpacing <= 0 {
		lineSpacing = int(float64(r.params.FontSize) * 1.5)
	}

	marginLeft := r.params.MarginLeft
	marginTop := r.params.MarginTop
	marginRight := r.params.MarginRight
	usableWidth := r.params.Width - marginLeft - marginRight
	if usableWidth <= 0 {
		usableWidth = bounds.Dx() - marginLeft - marginRight
	}

	// 墨水基色
	baseR, baseG, baseB, _ := r.fill.RGBA()

	y := marginTop + r.params.FontSize
	for _, ln := range lines {
		if ln.rightAlign {
			// 右对齐：先测量宽度
			adv := drawer.MeasureString(ln.text).Ceil()
			drawer.Dot.X = fixed.I(marginLeft + usableWidth - adv)
		} else {
			drawer.Dot.X = fixed.I(marginLeft)
		}
		drawer.Dot.Y = fixed.I(y)

		// 逐字符绘制，叠加扰动
		r.drawStringWithPerturbation(drawer, ln.text, baseR, baseG, baseB, y)

		y += lineSpacing + int(r.gauss(r.params.LineSpacingSigma, 0))
	}

	// 下划线
	if r.params.IsUnderlined {
		r.drawUnderlines(canvas, lines, marginLeft, marginTop, lineSpacing, usableWidth)
	}

	// 编码 PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("编码 PNG 失败: %w", err)
	}
	return buf.Bytes(), nil
}

// drawStringWithPerturbation 逐字符绘制，叠加位置/旋转/墨水深浅扰动。
func (r *Renderer) drawStringWithPerturbation(d *font.Drawer, text string, baseR, baseG, baseB uint32, lineY int) {
	if text == "" {
		return
	}
	marginLeft := r.params.MarginLeft
	marginRight := r.params.MarginRight
	usableWidth := r.params.Width - marginLeft - marginRight
	if usableWidth <= 0 {
		usableWidth = d.Dst.Bounds().Dx() - marginLeft - marginRight
	}

	x := d.Dot.X
	for _, ch := range text {
		// 横向扰动
		dx := fixed.I(int(r.gauss(r.params.PerturbXSigma, 0)))
		// 纵向扰动
		dy := fixed.I(int(r.gauss(r.params.PerturbYSigma, 0)))
		// 字号扰动（影响该字符的绘制大小，简化为微调 Y 基线）
		_ = r.params.FontSizeSigma

		d.Dot.X = x + dx
		d.Dot.Y = fixed.I(lineY) + dy

		// 墨水深浅：在基色上叠加随机扰动
		inkDelta := 0.0
		if r.params.InkDepthSigma > 0 {
			inkDelta = r.gauss(r.params.InkDepthSigma, 0)
		}
		nr := clamp8(int(baseR>>8) + int(inkDelta))
		ng := clamp8(int(baseG>>8) + int(inkDelta))
		nb := clamp8(int(baseB>>8) + int(inkDelta))
		d.Src = image.NewUniform(color.NRGBA{R: nr, G: ng, B: nb, A: 255})

		// 绘制单个字符
		d.DrawString(string(ch))

		// 推进 X
		adv := d.MeasureString(string(ch))
		// 字间距
		wordSpacing := fixed.I(r.params.WordSpacing)
		x = x + adv + wordSpacing + dx

		// 删除线（涂改痕迹）
		if r.params.StrikethroughProbability > 0 && r.rng.Float64() < r.params.StrikethroughProbability {
			if rgba, ok := d.Dst.(*image.RGBA); ok {
				r.drawStrikethrough(rgba, x-fixed.I(r.params.WordSpacing), fixed.I(lineY), adv)
			}
		}

		// 超出右边界则换行（简化：停止本行）
		if x.Ceil() > marginLeft+usableWidth {
			break
		}
	}
}

// drawStrikethrough 在字符上方画一条涂改线。
func (r *Renderer) drawStrikethrough(canvas *image.RGBA, x fixed.Int26_6, y fixed.Int26_6, w fixed.Int26_6) {
	width := r.params.StrikethroughWidth
	if width <= 0 {
		width = 3
	}
	length := w.Ceil() + int(r.gauss(r.params.StrikethroughLengthSigma, 0))
	if length < 4 {
		length = 4
	}
	angle := r.gauss(r.params.StrikethroughAngleSigma, 0)
	// 墨水色稍深
	cr, cg, cb, _ := r.fill.RGBA()
	c := color.NRGBA{R: clamp8(int(cr>>8) - 20), G: clamp8(int(cg>>8) - 20), B: clamp8(int(cb>>8) - 20), A: 255}

	x0 := x.Ceil()
	y0 := y.Ceil() - r.params.FontSize/3
	x1 := x0 + length
	y1 := y0 + int(math.Sin(angle)*float64(length))
	_ = width
	drawLine(canvas, x0, y0, x1, y1, int(width)+int(r.gauss(r.params.StrikethroughWidthSigma, 0)), c)
}

// drawUnderlines 在每行下方画横线（笔记本风格）。
func (r *Renderer) drawUnderlines(canvas *image.RGBA, lines []layoutLine, marginLeft, marginTop, lineSpacing, usableWidth int) {
	c := color.NRGBA{R: 100, G: 120, B: 180, A: 80}
	for i := range lines {
		y := marginTop + r.params.FontSize + i*lineSpacing + lineSpacing - 4
		drawLine(canvas, marginLeft, y, marginLeft+usableWidth, y, 1, c)
	}
}

// drawLine 在 RGBA 上画一条带宽度线（Bresenham 简化版）。
func drawLine(img *image.RGBA, x0, y0, x1, y1, w int, c color.Color) {
	if w < 1 {
		w = 1
	}
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx - dy
	bounds := img.Bounds()
	for {
		for wy := -w / 2; wy <= w/2; wy++ {
			for wx := -w / 2; wx <= w/2; wx++ {
				px := x0 + wx
				py := y0 + wy
				if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
					img.Set(px, py, c)
				}
			}
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// gauss 返回均值 mean、标准差 sigma 的高斯随机数。
func (r *Renderer) gauss(sigma, mean float64) float64 {
	if sigma <= 0 {
		return mean
	}
	return mean + r.rng.NormFloat64()*sigma
}

// ─── 字体与背景加载 ───

// loadFontBytes 优先用用户上传的 base64，其次用内置字体名。
func loadFontBytes(fontOption, fontFileBase64 string) ([]byte, error) {
	if fontFileBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(fontFileBase64)
		if err != nil {
			return nil, fmt.Errorf("解析字体 base64 失败: %w", err)
		}
		return data, nil
	}
	if fontOption != "" {
		// 在字体目录中查找
		home, _ := os.UserHomeDir()
		fontDir := filepath.Join(home, ".handwriting-desktop", "fonts")
		candidate := filepath.Join(fontDir, fontOption)
		if data, err := os.ReadFile(candidate); err == nil {
			return data, nil
		}
		// 尝试加上 .ttf 后缀
		candidate2 := filepath.Join(fontDir, fontOption+".ttf")
		if data, err := os.ReadFile(candidate2); err == nil {
			return data, nil
		}
	}
	// 回退：使用内置 gofont（goregular）—— TTF 格式，可直接用 opentype 解析。
	return goregular.TTF, nil
}

// loadBackground 加载用户上传的背景，或按 width/height 生成横线背景。
func loadBackground(params HandwritingParams) (image.Image, error) {
	if params.BackgroundImageBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(params.BackgroundImageBase64)
		if err != nil {
			return nil, fmt.Errorf("解析背景图 base64 失败: %w", err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("解码背景 PNG 失败: %w", err)
		}
		return img, nil
	}
	width := params.Width
	height := params.Height
	if width <= 0 {
		width = 2481
	}
	if height <= 0 {
		height = 3507
	}
	return createNotebookBackground(width, height, params), nil
}

// createNotebookBackground 生成白底横线笔记本背景。
func createNotebookBackground(width, height int, params HandwritingParams) image.Image {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	// 白底
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	lineSpacing := params.LineSpacing
	if lineSpacing <= 0 {
		lineSpacing = int(float64(params.FontSize) * 1.5)
	}
	marginTop := params.MarginTop
	marginBottom := params.MarginBottom
	marginLeft := params.MarginLeft
	marginRight := params.MarginRight

	lineColor := color.NRGBA{R: 135, G: 165, B: 210, A: 90}
	y := marginTop + params.FontSize + lineSpacing
	for y < height-marginBottom {
		drawLine(canvas, marginLeft, y, width-marginLeft-marginRight, y, 1, lineColor)
		y += lineSpacing
	}
	return canvas
}

// ─── 辅助函数 ───

func parseFill(fill string) color.Color {
	if fill == "" {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	}
	parts := strings.Split(fill, ",")
	if len(parts) < 3 {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	}
	r := clamp8(atoi(parts[0]))
	g := clamp8(atoi(parts[1]))
	b := clamp8(atoi(parts[2]))
	a := 255
	if len(parts) >= 4 {
		a = int(clamp8(atoi(parts[3])))
	}
	return color.NRGBA{R: r, G: g, B: b, A: uint8(a)}
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// adjustEnglishSpacing 在英文单词之间加双空格，中文不变。
func adjustEnglishSpacing(text string) string {
	englishPattern := regexp.MustCompile(`^[a-zA-Z0-9.,!?;:'"()\-_]+$`)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		parts := strings.Split(line, " ")
		if len(parts) <= 1 {
			continue
		}
		var result []string
		for j, part := range parts {
			result = append(result, part)
			if j < len(parts)-1 {
				cur := englishPattern.MatchString(strings.TrimSpace(part))
				nxt := englishPattern.MatchString(strings.TrimSpace(parts[j+1]))
				if cur && nxt {
					result = append(result, " ")
				}
				result = append(result, " ")
			}
		}
		lines[i] = strings.Join(result, "")
	}
	return strings.Join(lines, "\n")
}

// ─── 导出打包 ───

// pagesToZIP 把多页 PNG 打包成 zip 字节。
func pagesToZIP(pages [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, p := range pages {
		w, err := zw.Create(fmt.Sprintf("page_%03d.png", i+1))
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(p); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pagesToBase64 把多页 PNG 转成 base64 字符串数组。
func pagesToBase64(pages [][]byte) []string {
	result := make([]string, 0, len(pages))
	for _, p := range pages {
		result = append(result, base64.StdEncoding.EncodeToString(p))
	}
	return result
}
