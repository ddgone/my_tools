package bxn_point_cloud_renew

// 输入文件读取：支持 csv / txt / xls / xlsx。
//   - csv/txt：自动识别 UTF-8 BOM / UTF-16 BOM / GBK(GB18030) 编码
//   - xls/xlsx：Office / WPS 保存的表格直接读取（内部即 Unicode，无编码问题）
//   - 自动判断第一行是表头还是数据
//   - 第四列为框 ID 过滤（可选）：多个框 CSV 里用双引号包裹逗号分隔，
//     或 excel 单元格内直接逗号分隔；宽容兼容不带引号的多列写法

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// readInputFile 统一入口：按扩展名分发，返回记录列表。
// 每条记录固定 4 列：[0]老包目录 [1]新项目目录 [2]项目ID [3]框ID过滤(可为空)。
func readInputFile(path string, out io.Writer) ([][]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xlsx":
		return readXlsx(path, out)
	case ".xls":
		return readXls(path, out)
	default:
		// .csv / .txt 及其他文本格式
		return readCsvLike(path, out)
	}
}

// ==================== csv / txt ====================

// readCsvLike 读取 csv/txt：处理 BOM/GBK 编码后按 CSV 规则解析。
func readCsvLike(path string, out io.Writer) ([][]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	utf8Data, err := toUTF8(raw)
	if err != nil {
		return nil, fmt.Errorf("decode %s failed: %w", path, err)
	}

	r := csv.NewReader(bytes.NewReader(utf8Data))
	r.FieldsPerRecord = -1 // 允许变长列
	r.LazyQuotes = true    // 宽容处理引号
	r.TrimLeadingSpace = true

	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse csv line %d failed: %w", len(rows)+1, err)
		}
		rows = append(rows, rec)
	}
	return normalizeRows(rows, out), nil
}

// toUTF8 编码检测与转换：
// UTF-8 BOM / UTF-16LE BOM / UTF-16BE BOM / 无 BOM 时 UTF-8 优先、退化到 GB18030。
func toUTF8(raw []byte) ([]byte, error) {
	// UTF-8 BOM
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return raw[3:], nil
	}
	// UTF-16LE BOM
	if bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) {
		return decodeUTF16(raw[2:], true)
	}
	// UTF-16BE BOM
	if bytes.HasPrefix(raw, []byte{0xFE, 0xFF}) {
		return decodeUTF16(raw[2:], false)
	}
	// 无 BOM：合法 UTF-8 直接用，否则按 GB18030（GBK 超集，兼容 WPS/Office 中文另存）
	if utf8.Valid(raw) {
		return raw, nil
	}
	return simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
}

// decodeUTF16 手工解码 UTF-16（含代理对），输出 UTF-8。
func decodeUTF16(data []byte, little bool) ([]byte, error) {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	u16 := make([]uint16, len(data)/2)
	for i := range u16 {
		if little {
			u16[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
		} else {
			u16[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
		}
	}
	runes := utf16.Decode(u16)
	return []byte(string(runes)), nil
}

// ==================== xlsx ====================

func readXlsx(path string, out io.Writer) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx failed: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheet in xlsx: %s", path)
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("read xlsx rows failed: %w", err)
	}
	return normalizeRows(rows, out), nil
}

// ==================== xls (Office 97-2003 / WPS) ====================

func readXls(path string, out io.Writer) ([][]string, error) {
	wb, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("open xls failed: %w", err)
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("no sheet in xls: %s", path)
	}

	var rows [][]string
	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := sheet.Row(i)
		if row == nil {
			continue
		}
		lastCol := row.LastCol()
		if lastCol <= 0 {
			continue
		}
		cols := make([]string, 0, lastCol)
		for j := 0; j < lastCol; j++ {
			cols = append(cols, strings.TrimSpace(row.Col(j)))
		}
		// 去掉尾部空单元格
		end := len(cols)
		for end > 0 && strings.TrimSpace(cols[end-1]) == "" {
			end--
		}
		if end > 0 {
			rows = append(rows, cols[:end])
		}
	}
	return normalizeRows(rows, out), nil
}

// ==================== 行归一化与表头识别 ====================

// normalizeRows 清洗原始行：跳过空行、识别并跳过表头、归一化为 4 列。
func normalizeRows(rows [][]string, out io.Writer) [][]string {
	var records [][]string
	for i, cols := range rows {
		// 跳过全空行
		if len(cols) == 0 {
			continue
		}
		// 第一行像表头则跳过
		if i == 0 && looksLikeHeader(cols) {
			fmt.Fprintf(out, "[INPUT] First row looks like a header, skipped: %s\n",
				strings.Join(cols, " | "))
			continue
		}
		rec := normalizeRecord(cols)
		if rec != nil {
			records = append(records, rec)
		}
	}
	return records
}

// normalizeRecord 单行归一化为固定 4 列，无效行返回 nil。
// 第 4 列之后的列宽容合并为框 ID 列表（兼容不带引号的多框写法）。
func normalizeRecord(cols []string) []string {
	if len(cols) < 2 {
		return nil
	}
	oldDir := strings.TrimSpace(cols[0])
	newDir := strings.TrimSpace(cols[1])
	projectID := ""
	if len(cols) >= 3 {
		projectID = strings.TrimSpace(cols[2])
	}
	frameFilter := ""
	if len(cols) >= 4 {
		frameFilter = strings.Join(cols[3:], ",")
	}
	if oldDir == "" || newDir == "" {
		return nil
	}
	return []string{oldDir, newDir, projectID, frameFilter}
}

// looksLikeHeader 判断首行是否为表头：
//   - 前 4 列中出现短的关键词单元格（如"标题"、"老包路径"、"项目ID"）
//   - 或第一列不像路径（无分隔符且磁盘上不存在）
func looksLikeHeader(cols []string) bool {
	keywords := []string{
		"标题", "表头", "老包", "旧包", "新包", "新项目", "项目", "框",
		"路径", "目录", "过滤", "header", "title", "path", "dir", "frame", "project",
	}
	for i, c := range cols {
		if i >= 4 {
			break
		}
		c = strings.TrimSpace(c)
		if c == "" || len([]rune(c)) > 20 || strings.ContainsAny(c, "\\/") {
			continue
		}
		lc := strings.ToLower(c)
		for _, kw := range keywords {
			if strings.Contains(lc, kw) {
				return true
			}
		}
	}
	c1 := strings.TrimSpace(cols[0])
	if c1 == "" {
		return false
	}
	if !strings.ContainsAny(c1, "\\/") && !dirExists(c1) {
		return true
	}
	return false
}

// ==================== 框 ID 过滤解析 ====================

// parseFrameFilterIDs 解析第四列框 ID 列表：
// 容忍中文逗号、外层引号、多余空格；空串返回 nil（不过滤）。
func parseFrameFilterIDs(spec string) []string {
	spec = strings.TrimSpace(spec)
	// 去掉外层成对引号（CSV 转义已在解析时处理，这里兜底）
	if len(spec) >= 2 {
		if (spec[0] == '"' && spec[len(spec)-1] == '"') ||
			(spec[0] == '\'' && spec[len(spec)-1] == '\'') {
			spec = spec[1 : len(spec)-1]
		}
	}
	if spec == "" {
		return nil
	}
	spec = strings.ReplaceAll(spec, "，", ",")
	var ids []string
	seen := make(map[string]bool)
	for _, id := range strings.Split(spec, ",") {
		id = strings.TrimSpace(id)
		id = strings.Trim(id, `"'`)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// filterFramesByID 按框 ID 列表过滤项目查询结果；filter 为空返回全部。
// 指定的框 ID 在查询结果中不存在时返回错误（避免静默漏框）。
func filterFramesByID(frames []taskFrame, filterIDs []string) ([]taskFrame, error) {
	if len(filterIDs) == 0 {
		return frames, nil
	}
	want := make(map[string]bool)
	for _, id := range filterIDs {
		want[id] = true
	}
	var out []taskFrame
	for _, f := range frames {
		if want[f.id] {
			delete(want, f.id)
			out = append(out, f)
		}
	}
	if len(want) > 0 {
		var missing []string
		for id := range want {
			missing = append(missing, id)
		}
		return nil, fmt.Errorf("frame ids not found in project query result: %s",
			strings.Join(missing, ", "))
	}
	return out, nil
}
