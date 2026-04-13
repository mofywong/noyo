package importer

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// InjectionData 注入到模板的数据 (e.g. 产品列表)
type InjectionData map[string]interface{}

// GenerateTemplate 生成 Excel 模板
func GenerateTemplate(layouts []SheetLayout, data InjectionData) (*excelize.File, error) {
	f := excelize.NewFile()

	// Create styles
	styleRequired, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#DDEBF7"}, Pattern: 1}, // Light Blue
		Font: &excelize.Font{Bold: true},
	})
	styleReadonly, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#F2F2F2"}, Pattern: 1}, // Grey
	})
	styleDefault, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})

	// Pre-calculate column letters for references: Sheet -> Key -> Letter (e.g. "A")
	colMap := make(map[string]map[string]string)

	// First pass: Create sheets and headers to build colMap
	// We need to loop layouts twice: first to setup structure, second to setup validation (which might depend on structure)

	// Track helper columns: Sheet -> Format -> ColumnLetter
	helperCols := make(map[string]string)
	// Track next available column index per sheet
	sheetNextCol := make(map[string]int)

	for _, sheet := range layouts {
		colMap[sheet.Name] = make(map[string]string)
		for i, col := range sheet.Columns {
			colName, _ := excelize.ColumnNumberToName(i + 1)
			colMap[sheet.Name][col.Key] = colName
		}
		sheetNextCol[sheet.Name] = len(sheet.Columns) + 1
	}

	// Remove default "Sheet1" if we are creating our own
	if len(layouts) > 0 {
		if layouts[0].Name != "Sheet1" {
			f.SetSheetName("Sheet1", layouts[0].Name)
		}
	}

	// Prepare Reference Sheet tracking
	refSheetCreated := false
	refColIndex := 1

	for i, sheet := range layouts {
		sheetName := sheet.Name
		if i > 0 || sheetName != f.GetSheetName(0) {
			f.NewSheet(sheetName)
		}

		if sheet.IsHidden {
			f.SetSheetVisible(sheetName, false)
		}

		// Set Headers
		for j, col := range sheet.Columns {
			colName, _ := excelize.ColumnNumberToName(j + 1)
			cell := colName + "1"
			f.SetCellValue(sheetName, cell, col.Header)

			// Set Width
			width := col.Width
			if width == 0 {
				width = 20
			}
			f.SetColWidth(sheetName, colName, colName, width)

			// Set Style
			var styleID int
			switch col.Style {
			case "required":
				styleID = styleRequired
			case "readonly":
				styleID = styleReadonly
			default:
				styleID = styleDefault
			}
			f.SetCellStyle(sheetName, cell, cell, styleID)
		}
	}

	// Second pass: Add Validations
	for _, sheet := range layouts {
		for j, col := range sheet.Columns {
			if col.Validation == nil {
				continue
			}

			colName, _ := excelize.ColumnNumberToName(j + 1)
			// Apply to rows 2-1000
			rangeStr := fmt.Sprintf("%s2:%s1000", colName, colName)

			dv := excelize.NewDataValidation(true)
			dv.Sqref = rangeStr

			switch col.Validation.Type {
			case "unique":
				dv.Type = "custom"
				// COUNTIF(Range, Cell) < 2 means count is 0 or 1.
				// Formula needs to be relative to the top-left cell of Sqref (Row 2)
				formula := fmt.Sprintf("=COUNTIF($%s$2:$%s$1000, %s2)<2", colName, colName, colName)
				dv.Formula1 = formula

			case "list":
				dv.Type = "list"
				if len(col.Validation.Options) > 0 {
					// Limitation: Excel validation list string cannot exceed 255 chars
					// Better to put in Reference sheet if long. For now, try direct.
					// If options are simple (Yes, No), direct is fine.
					quotedOptions := make([]string, len(col.Validation.Options))
					for k, v := range col.Validation.Options {
						quotedOptions[k] = v // excelize handles quoting usually? No, SetDropList takes []string
					}
					dv.SetDropList(quotedOptions)
					f.AddDataValidation(sheet.Name, dv)
				}

			case "provider":
				dv.Type = "list"
				// Write data to Reference sheet
				if !refSheetCreated {
					f.NewSheet("Reference")
					f.SetSheetVisible("Reference", false)
					refSheetCreated = true
				}

				dataKey := col.Validation.ProviderKey
				if val, ok := data[dataKey]; ok {
					// Assume val is []string or specific struct list converted to string
					// We need to know what kind of data it is.
					// For simplicity, let's assume the caller prepared []string in `data`.
					// Or we handle specific keys here.

					var options []string
					switch v := val.(type) {
					case []string:
						options = v
					case []interface{}:
						for _, item := range v {
							options = append(options, fmt.Sprint(item))
						}
					}

					if len(options) > 0 {
						// Write to Reference Sheet column
						refColName, _ := excelize.ColumnNumberToName(refColIndex)
						for r, opt := range options {
							f.SetCellValue("Reference", fmt.Sprintf("%s%d", refColName, r+1), opt)
						}

						// Create formula referencing this range
						formula := fmt.Sprintf("'Reference'!$%s$1:$%s$%d", refColName, refColName, len(options))
						dv.SetSqrefDropList(formula)
						f.AddDataValidation(sheet.Name, dv)

						refColIndex++
					}
				}

			case "reference":
				dv.Type = "list"
				targetSheet := col.Validation.RefSheetName
				targetKey := col.Validation.RefColumnKey
				displayFormat := col.Validation.DisplayFormat

				if targetSheet != "" && targetKey != "" {
					targetCol, ok := colMap[targetSheet][targetKey]
					if !ok {
						continue
					}

					refCol := targetCol
					// Default count formula: COUNTA (works for static values)
					countFormula := fmt.Sprintf("COUNTA('%s'!$%s$2:$%s$1000)", targetSheet, refCol, refCol)

					if displayFormat == "name_code" {
						helperKey := targetSheet + "|name_code"
						if existingHelperCol, exists := helperCols[helperKey]; exists {
							refCol = existingHelperCol
						} else {
							// Try to find Name and Code columns
							nameCol := colMap[targetSheet]["name"]
							codeCol := colMap[targetSheet]["code"]

							if nameCol != "" && codeCol != "" {
								nextIdx := sheetNextCol[targetSheet]
								sheetNextCol[targetSheet]++
								newColName, _ := excelize.ColumnNumberToName(nextIdx)

								// Write formula to rows 2-1000
								// Formula: =IF(Code="", "", Name & " (" & Code & ")")
								for r := 2; r <= 1000; r++ {
									cell := fmt.Sprintf("%s%d", newColName, r)
									formula := fmt.Sprintf("=IF(%s%d=\"\",\"\",%s%d & \" (\" & %s%d & \")\")", codeCol, r, nameCol, r, codeCol, r)
									f.SetCellFormula(targetSheet, cell, formula)
								}

								f.SetColVisible(targetSheet, newColName, false)
								helperCols[helperKey] = newColName
								refCol = newColName
							}
						}
						// Update count formula to ignore empty strings returned by formula
						countFormula = fmt.Sprintf("COUNTIF('%s'!$%s$2:$%s$1000, \"?*\")", targetSheet, refCol, refCol)
					}

					// Formula: =OFFSET(Sheet!$Col$2, 0, 0, COUNT..., 1)
					formula := fmt.Sprintf("=OFFSET('%s'!$%s$2, 0, 0, %s, 1)", targetSheet, refCol, countFormula)
					dv.SetSqrefDropList(formula)
					f.AddDataValidation(sheet.Name, dv)
				}
			}
		}
	}

	return f, nil
}

// FillData 填充数据到模板
func FillData(f *excelize.File, layouts []SheetLayout, data ImportRawData) error {
	for _, sheet := range layouts {
		rows, ok := data[sheet.Name]
		if !ok || len(rows) == 0 {
			continue
		}

		// Map Key -> ColIndex (0-based)
		keyMap := make(map[string]int)
		for i, col := range sheet.Columns {
			keyMap[col.Key] = i
		}

		// Start writing from Row 2
		startRow := 2
		for i, rowData := range rows {
			rowNum := startRow + i

			for key, val := range rowData {
				colIdx, exists := keyMap[key]
				if !exists {
					continue
				}

				colName, _ := excelize.ColumnNumberToName(colIdx + 1)
				cell := fmt.Sprintf("%s%d", colName, rowNum)
				f.SetCellValue(sheet.Name, cell, val)
			}
		}
	}
	return nil
}
