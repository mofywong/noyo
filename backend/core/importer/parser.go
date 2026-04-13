package importer

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Parse 这是一个通用的解析函数，它不关心具体业务逻辑，只负责将 Excel 转换为 ImportRawData
func Parse(reader io.Reader) (ImportRawData, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(ImportRawData)
	sheets := f.GetSheetList()

	for _, sheetName := range sheets {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("read sheet %s failed: %v", sheetName, err)
		}

		if len(rows) < 2 {
			continue // Skip empty or header-only sheets
		}

		// Row 1 is Header. We need to map Header Name -> Column Index.
		// However, we don't have the Layout here to know the Key.
		// So we return Map[HeaderName] -> Value.
		// Wait, `ImportRawData` is `[]map[string]string` where key is `ColumnMeta.Key`.
		// But Parser doesn't know the Key mapping!

		// Solution: The Parser should probably just return `[]map[HeaderName]string`
		// AND let the Binder or Plugin map HeaderName to Key using Layout?
		// OR, we pass Layout to Parse.

		// Let's pass Layout to Parse? No, keep it generic.
		// If we look at `importer.Bind`, it expects `map[string]string` where key is the Struct Tag.
		// The Struct Tag corresponds to `ColumnMeta.Key`.
		// But the Excel file only has `ColumnMeta.Header`.

		// So we have a missing link: Header -> Key mapping.
		// The Plugin knows the Layout (Key <-> Header).

		// Let's change Parse to return raw rows with Header names,
		// OR we pass a "Header Map" to Parse.

		// Let's stick to the Design: The Plugin calls `Parse`.
		// The Plugin has the Layout.
		// So the Plugin can resolve this.

		// But wait, `Bind` uses struct tags.
		// If `Bind` expects keys like "device_name", but Excel has "设备名称",
		// we need a translation step.

		// Let's make `Parse` simpler: Just return `map[SheetName][]map[ColIndex]Value`? No, too low level.
		// Let's return `map[SheetName][]map[HeaderName]Value`.
		// Then `Bind` or a helper `Normalize` can convert `HeaderName` -> `Key`.

		// Let's check `types.go`:
		// type ImportRawData map[string][]map[string]string

		// I will implement Parse to return map[HeaderName]Value.
		// And I will add a helper `Normalize(data, layout)` to convert to Key.

		sheetData := make([]map[string]string, 0, len(rows)-1)
		headers := rows[0]

		for i := 1; i < len(rows); i++ {
			row := rows[i]
			rowMap := make(map[string]string)
			hasData := false

			for j, cellVal := range row {
				if j >= len(headers) {
					continue
				}
				header := strings.TrimSpace(headers[j])
				if header == "" {
					continue
				}

				val := strings.TrimSpace(cellVal)
				if val != "" {
					hasData = true
				}
				rowMap[header] = val
			}

			if hasData {
				sheetData = append(sheetData, rowMap)
			}
		}
		result[sheetName] = sheetData
	}

	return result, nil
}

// Normalize converts Header-based maps to Key-based maps using the Layout
func Normalize(raw ImportRawData, layouts []SheetLayout) ImportRawData {
	normalized := make(ImportRawData)

	// Create Header -> Key mapping per sheet
	headerToKey := make(map[string]map[string]string)
	for _, sheet := range layouts {
		m := make(map[string]string)
		for _, col := range sheet.Columns {
			m[col.Header] = col.Key

			// Auto-mapping: Key -> Key (Support English headers / direct keys)
			m[col.Key] = col.Key

			// Auto-mapping: Clean Header (remove " (必填)", " (选填)" etc)
			// e.g. "设备编码 (必填)" -> "设备编码"
			if idx := strings.Index(col.Header, " ("); idx > 0 {
				cleanHeader := col.Header[:idx]
				m[cleanHeader] = col.Key
			}
		}
		headerToKey[sheet.Name] = m
	}

	for sheetName, rows := range raw {
		var targetSheetName string
		var mapping map[string]string

		// 1. Try exact match
		if m, ok := headerToKey[sheetName]; ok {
			targetSheetName = sheetName
			mapping = m
		} else {
			// 2. Try fuzzy match for Sheet Name
			// e.g. "PollingGroups" matches "Polling Groups"
			inputClean := cleanSheetName(sheetName)
			for layoutName, m := range headerToKey {
				if inputClean == cleanSheetName(layoutName) {
					targetSheetName = layoutName
					mapping = m
					break
				}
			}
		}

		if mapping == nil {
			// Skip unknown sheets
			continue
		}

		newRows := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			newRow := make(map[string]string)
			for header, val := range row {
				// 1. Exact match (Full Header or Key)
				if key, found := mapping[header]; found {
					newRow[key] = val
					continue
				}

				// 2. Fuzzy match: Try to clean the input header too
				// e.g. Input "设备编码(必填)" -> Clean "设备编码" -> Match mapping["设备编码"]
				cleanInput := cleanHeader(header)
				if key, found := mapping[cleanInput]; found {
					newRow[key] = val
					continue
				}
			}
			newRows = append(newRows, newRow)
		}
		// Store using the canonical (Layout) sheet name so Bind can find it
		normalized[targetSheetName] = newRows
	}

	return normalized
}

func cleanSheetName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "(", "") // Just remove parens, don't remove content!
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "（", "")
	s = strings.ReplaceAll(s, "）", "")
	return s
}

func cleanHeader(h string) string {
	// Remove (...) or （...）
	if idx := strings.Index(h, "("); idx > 0 {
		return strings.TrimSpace(h[:idx])
	}
	if idx := strings.Index(h, "（"); idx > 0 {
		return strings.TrimSpace(h[:idx])
	}
	// Also handle case where ( is at start? Unlikely for headers like "Name (Required)"
	return h
}
