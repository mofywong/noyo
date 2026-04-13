package importer

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Bind 将 RawData 中的某个 Sheet 绑定到结构体切片
// sheetName: Excel页签名称
// target: 指向结构体切片的指针 (e.g. *[]ModbusDeviceRow)
// 结构体字段必须包含 tag `import:"KeyName"`
func Bind(data ImportRawData, sheetName string, target interface{}) error {
	// 1. Check target type
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Slice {
		return errors.New("target must be a pointer to a slice")
	}
	sliceVal := val.Elem()
	elemType := sliceVal.Type().Elem() // Struct type

	// 2. Get rows
	rows, ok := data[sheetName]
	if !ok {
		// Sheet not found is acceptable (empty slice), unless required?
		// For now, return nil
		return nil
	}

	// 3. Cache struct field tags
	// Map: KeyName -> FieldIndex
	fieldMap := make(map[string]int)
	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		tag := field.Tag.Get("import")
		if tag != "" {
			fieldMap[tag] = i
		}
	}

	// 4. Iterate rows and bind
	for _, rowMap := range rows {
		newElem := reflect.New(elemType).Elem()

		for key, value := range rowMap {
			fieldIdx, exists := fieldMap[key]
			if !exists {
				continue
			}

			field := newElem.Field(fieldIdx)
			if !field.CanSet() {
				continue
			}

			// Type Conversion
			if err := setFieldValue(field, value); err != nil {
				return fmt.Errorf("failed to set field %s with value %s: %v", key, value, err)
			}
		}

		sliceVal.Set(reflect.Append(sliceVal, newElem))
	}

	return nil
}

func setFieldValue(field reflect.Value, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil // Leave as zero value
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Handle float strings like "1.0" coming from Excel
		if strings.Contains(value, ".") {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			field.SetInt(int64(f))
		} else {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(i)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if strings.Contains(value, ".") {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			field.SetUint(uint64(f))
		} else {
			i, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetUint(i)
		}
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)
	case reflect.Bool:
		// Handle "Yes/No", "True/False", "是/否"
		lower := strings.ToLower(value)
		if lower == "yes" || lower == "true" || lower == "是" || lower == "1" {
			field.SetBool(true)
		} else {
			field.SetBool(false)
		}
	default:
		return fmt.Errorf("unsupported type: %s", field.Kind())
	}
	return nil
}
