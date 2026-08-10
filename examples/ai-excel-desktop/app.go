package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/xuri/excelize/v2"
)

// ============================================================================
// 数据结构（保留原有结构定义，供前后端共享）
// ============================================================================

// SheetData 表示一个工作表的完整数据
type SheetData struct {
	Name    string            `json:"name"`
	Headers []string          `json:"headers"`
	Rows    [][]interface{}   `json:"rows"`
	Types   map[string]string `json:"types"` // numeric | categorical | date
	Total   int               `json:"total"`
}

// FileInfo 文件元信息
type FileInfo struct {
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Size     int64    `json:"size"`
	Sheets   []string `json:"sheets"`
	Selected string   `json:"selected"`
}

// ColumnStats 列统计信息
type ColumnStats struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Count     int     `json:"count"`
	NullCount int     `json:"nullCount"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Mean      float64 `json:"mean"`
	Median    float64 `json:"median"`
	StdDev    float64 `json:"stdDev"`
	Unique    int     `json:"unique"`
}

// ChartConfig AI 生成的图表配置
type ChartConfig struct {
	Type   string   `json:"type"`   // bar | line | pie | scatter | histogram | area
	XAxis  string   `json:"xAxis"`
	YAxes  []string `json:"yAxes"`
	Title  string   `json:"title"`
	Bin    int      `json:"bin"`    // 直方图分箱数
	Sample int      `json:"sample"`
}

// ============================================================================
// 文件操作
// ============================================================================

// OpenFileDialog 打开文件选择对话框
func (a *App) OpenFileDialog() (*FileInfo, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 Excel 或 CSV 文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel 文件 (*.xlsx;*.xls)", Pattern: "*.xlsx;*.xls"},
			{DisplayName: "CSV 文件 (*.csv)", Pattern: "*.csv"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if selection == "" {
		return nil, nil // 用户取消
	}
	return a.LoadFile(selection)
}

// LoadFile 加载指定路径的文件
func (a *App) LoadFile(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %v", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	fi := &FileInfo{
		Path: path,
		Name: filepath.Base(path),
		Size: info.Size(),
	}

	switch ext {
	case ".xlsx", ".xls":
		f, err := excelize.OpenFile(path)
		if err != nil {
			return nil, fmt.Errorf("打开 Excel 失败: %v", err)
		}
		defer f.Close()
		fi.Sheets = f.GetSheetList()
		if len(fi.Sheets) > 0 {
			fi.Selected = fi.Sheets[0]
		}
	case ".csv":
		fi.Sheets = []string{"Sheet1"}
		fi.Selected = "Sheet1"
	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	a.dataMu.Lock()
	a.fileInfo = fi
	a.dataMu.Unlock()
	return fi, nil
}

// LoadSheet 加载指定工作表的数据
func (a *App) LoadSheet(sheetName string) (*SheetData, error) {
	a.dataMu.RLock()
	fi := a.fileInfo
	a.dataMu.RUnlock()
	if fi == nil {
		return nil, fmt.Errorf("未加载文件")
	}

	ext := strings.ToLower(filepath.Ext(fi.Path))
	var data *SheetData
	var err error

	switch ext {
	case ".xlsx", ".xls":
		data, err = a.readExcel(fi.Path, sheetName)
	case ".csv":
		data, err = a.readCSV(fi.Path)
	default:
		return nil, fmt.Errorf("不支持的格式")
	}

	if err != nil {
		return nil, err
	}

	a.dataMu.Lock()
	a.sheetData = data
	a.dataMu.Unlock()
	return data, nil
}

// readExcel 读取 Excel 工作表
func (a *App) readExcel(path, sheetName string) (*SheetData, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("工作表为空")
	}

	headers := make([]string, len(rows[0]))
	for i, h := range rows[0] {
		if h == "" {
			headers[i] = fmt.Sprintf("列%d", i+1)
		} else {
			headers[i] = h
		}
	}

	data := &SheetData{
		Name:    sheetName,
		Headers: headers,
		Types:   make(map[string]string),
	}

	rowData := make([][]interface{}, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := make([]interface{}, len(headers))
		for j := 0; j < len(headers); j++ {
			if j < len(rows[i]) {
				row[j] = parseValue(rows[i][j])
			}
		}
		rowData = append(rowData, row)
	}
	data.Rows = rowData
	data.Total = len(rowData)
	data.Types = analyzeTypes(data)
	return data, nil
}

// readCSV 读取 CSV 文件
func (a *App) readCSV(path string) (*SheetData, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("CSV 为空")
	}

	parseLine := func(line string) []string {
		var fields []string
		var current strings.Builder
		inQuote := false
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c == '"' {
				if inQuote && i+1 < len(line) && line[i+1] == '"' {
					current.WriteByte('"')
					i++
				} else {
					inQuote = !inQuote
				}
			} else if c == ',' && !inQuote {
				fields = append(fields, current.String())
				current.Reset()
			} else {
				current.WriteByte(c)
			}
		}
		fields = append(fields, current.String())
		return fields
	}

	headers := parseLine(lines[0])
	for i, h := range headers {
		if h == "" {
			headers[i] = fmt.Sprintf("列%d", i+1)
		}
	}

	data := &SheetData{
		Name:    "Sheet1",
		Headers: headers,
		Types:   make(map[string]string),
	}

	rowData := make([][]interface{}, 0, len(lines)-1)
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		cells := parseLine(line)
		row := make([]interface{}, len(headers))
		for j := 0; j < len(headers); j++ {
			if j < len(cells) {
				row[j] = parseValue(cells[j])
			}
		}
		rowData = append(rowData, row)
	}
	data.Rows = rowData
	data.Total = len(rowData)
	data.Types = analyzeTypes(data)
	return data, nil
}

// parseValue 尝试将字符串解析为数字或保持字符串
func parseValue(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var intVal int64
	if n, err := fmt.Sscanf(s, "%d", &intVal); n == 1 && err == nil && fmt.Sprintf("%d", intVal) == s {
		return intVal
	}
	var fVal float64
	if _, err := fmt.Sscanf(s, "%f", &fVal); err == nil {
		test := fmt.Sprintf("%g", fVal)
		if test == s || strings.Replace(s, ".", "", -1) != s {
			return fVal
		}
	}
	return s
}

// analyzeTypes 分析每列的数据类型
func analyzeTypes(data *SheetData) map[string]string {
	types := make(map[string]string)
	sampleSize := 1000
	if len(data.Rows) < sampleSize {
		sampleSize = len(data.Rows)
	}

	for colIdx, header := range data.Headers {
		numericCount := 0
		dateCount := 0
		total := 0
		for r := 0; r < sampleSize; r++ {
			val := data.Rows[r][colIdx]
			if val == nil {
				continue
			}
			total++
			switch v := val.(type) {
			case int64, float64:
				numericCount++
			case string:
				if isNumericString(v) {
					numericCount++
				} else if isDateString(v) {
					dateCount++
				}
			}
		}
		if total == 0 {
			types[header] = "categorical"
		} else if numericCount*100/total > 70 {
			types[header] = "numeric"
		} else if dateCount*100/total > 70 {
			types[header] = "date"
		} else {
			types[header] = "categorical"
		}
	}
	return types
}

func isNumericString(s string) bool {
	if s == "" {
		return false
	}
	_, err := fmt.Sscanf(s, "%f", new(float64))
	return err == nil
}

func isDateString(s string) bool {
	layouts := []string{
		"2006-01-02", "2006/01/02", "01/02/2006", "01-02-2006",
		"2006-01-02 15:04:05", "2006/01/02 15:04",
	}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

// ============================================================================
// 数据分析
// ============================================================================

// AnalyzeColumns 分析所有列的统计信息
func (a *App) AnalyzeColumns() ([]*ColumnStats, error) {
	a.dataMu.RLock()
	data := a.sheetData
	a.dataMu.RUnlock()
	if data == nil {
		return nil, fmt.Errorf("未加载数据")
	}
	result := make([]*ColumnStats, 0, len(data.Headers))

	for colIdx, header := range data.Headers {
		stats := &ColumnStats{Name: header, Type: data.Types[header]}
		values := make([]float64, 0)
		uniqueSet := make(map[string]bool)
		nullCount := 0

		for _, row := range data.Rows {
			if colIdx >= len(row) {
				nullCount++
				continue
			}
			val := row[colIdx]
			if val == nil || val == "" {
				nullCount++
				continue
			}
			uniqueSet[fmt.Sprintf("%v", val)] = true
			switch v := val.(type) {
			case int64:
				values = append(values, float64(v))
			case float64:
				values = append(values, v)
			case string:
				var f float64
				if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
					values = append(values, f)
				}
			}
		}

		stats.Count = len(data.Rows)
		stats.NullCount = nullCount
		stats.Unique = len(uniqueSet)

		if len(values) > 0 && (stats.Type == "numeric" || stats.Type == "date") {
			sort.Float64s(values)
			stats.Min = values[0]
			stats.Max = values[len(values)-1]
			stats.Mean = mean(values)
			stats.Median = median(values)
			stats.StdDev = stdDev(values)
		}
		result = append(result, stats)
	}
	return result, nil
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func median(v []float64) float64 {
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (v[n/2-1] + v[n/2]) / 2
	}
	return v[n/2]
}

func stdDev(v []float64) float64 {
	n := len(v)
	if n == 0 {
		return 0
	}
	m := mean(v)
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(n))
}

// GetDataSummary 获取数据摘要（供 AI 上下文使用）
func (a *App) GetDataSummary() (string, error) {
	a.dataMu.RLock()
	data := a.sheetData
	fi := a.fileInfo
	a.dataMu.RUnlock()
	if data == nil {
		return "", fmt.Errorf("未加载数据")
	}
	var sb strings.Builder
	if fi != nil {
		sb.WriteString(fmt.Sprintf("数据集: %s\n", fi.Name))
	}
	sb.WriteString(fmt.Sprintf("工作表: %s\n", data.Name))
	sb.WriteString(fmt.Sprintf("总行数: %d, 总列数: %d\n", data.Total, len(data.Headers)))
	sb.WriteString("列信息:\n")
	for _, h := range data.Headers {
		sb.WriteString(fmt.Sprintf("  - %s (%s)\n", h, data.Types[h]))
	}
	return sb.String(), nil
}

// ============================================================================
// 导出
// ============================================================================

// ExportExcel 导出数据到 Excel
func (a *App) ExportExcel(headers []string, rows [][]interface{}) (string, error) {
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 Excel 文件",
		DefaultFilename: fmt.Sprintf("export_%s.xlsx", time.Now().Format("20060102_150405")),
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil || savePath == "" {
		return "", err
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for r, row := range rows {
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E0E7FF"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	f.SetCellStyle(sheet, "A1", "Z1", style)

	if err := f.SaveAs(savePath); err != nil {
		return "", err
	}
	return savePath, nil
}

// ExportCSV 导出 CSV
func (a *App) ExportCSV(headers []string, rows [][]interface{}) (string, error) {
	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存 CSV 文件",
		DefaultFilename: fmt.Sprintf("export_%s.csv", time.Now().Format("20060102_150405")),
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil || savePath == "" {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(strings.Join(headers, ",") + "\n")
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, v := range row {
			s := fmt.Sprintf("%v", v)
			if strings.Contains(s, ",") || strings.Contains(s, "\"") {
				s = "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
			}
			cells[i] = s
		}
		sb.WriteString(strings.Join(cells, ",") + "\n")
	}
	content := "\xEF\xBB\xBF" + sb.String()
	if err := os.WriteFile(savePath, []byte(content), 0644); err != nil {
		return "", err
	}
	return savePath, nil
}

// ============================================================================
// 辅助
// ============================================================================

// SampleData 返回采样后的行数据（大数据图表用）
func (a *App) SampleData(maxRows int) ([][]interface{}, error) {
	a.dataMu.RLock()
	data := a.sheetData
	a.dataMu.RUnlock()
	if data == nil {
		return nil, fmt.Errorf("未加载数据")
	}
	rows := data.Rows
	if len(rows) <= maxRows {
		return rows, nil
	}
	step := len(rows) / maxRows
	sampled := make([][]interface{}, 0, maxRows)
	for i := 0; i < len(rows); i += step {
		sampled = append(sampled, rows[i])
	}
	return sampled, nil
}

// GetSheetData 获取当前已加载的工作表数据
func (a *App) GetSheetData() *SheetData {
	a.dataMu.RLock()
	defer a.dataMu.RUnlock()
	return a.sheetData
}

// GetFileInfo 获取当前文件信息
func (a *App) GetFileInfo() *FileInfo {
	a.dataMu.RLock()
	defer a.dataMu.RUnlock()
	return a.fileInfo
}
