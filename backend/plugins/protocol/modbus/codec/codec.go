package codec

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/expr-lang/expr"
)

// ExtractRule defines how to extract raw data from bytes
type ExtractRule struct {
	RegisterIndex int    `json:"register_index"` // 0-based register offset
	DataType      string `json:"data_type"`      // int16, uint16, int32, uint32, int64, uint64, float32, float64, bool, string, hex
	ByteOrder     string `json:"byte_order"`     // ABCD, DCBA, CDAB, BADC
	BitIndex      int    `json:"bit_index"`      // 0-15, only for bool
	Length        int    `json:"length"`         // Register count for string/hex
}

// PointConfig defines the parsing rules for a single data point (Legacy)
type PointConfig struct {
	Offset    int    `json:"offset"`
	DataType  string `json:"data_type"`
	ByteOrder string `json:"byte_order"`
}

// Extract extracts raw value from byte slice based on rule
func Extract(data []byte, rule ExtractRule) (interface{}, error) {
	// 1. Calculate Byte Offset and Length
	// RegisterIndex is based on 16-bit registers (2 bytes)
	byteOffset := rule.RegisterIndex * 2

	// Determine length
	byteLength := 0
	switch rule.DataType {
	case "bool":
		// For FC1/FC2 (Coils/Inputs), Modbus returns bits packed in bytes.
		// If we only read 1 coil, the data length might be 1 byte.
		// For FC3/FC4 (Registers), it is always 2 bytes per register.
		if len(data) >= byteOffset+2 {
			byteLength = 2
		} else if len(data) >= byteOffset+1 {
			byteLength = 1
		} else {
			return nil, errors.New("data index out of range for bool")
		}
	case "int16", "uint16":
		byteLength = 2
	case "int32", "uint32", "float32":
		byteLength = 4
	case "int64", "uint64", "float64":
		byteLength = 8
	case "string", "hex":
		if rule.Length <= 0 {
			return nil, errors.New("length required for string/hex")
		}
		byteLength = rule.Length * 2
	default:
		return nil, errors.New("unsupported data type")
	}

	// 2. Check Boundary
	if byteOffset+byteLength > len(data) {
		return nil, errors.New("data index out of range")
	}

	// 3. Extract Raw Bytes
	rawBytes := make([]byte, byteLength)
	copy(rawBytes, data[byteOffset:byteOffset+byteLength])

	// 4. Handle Byte Order
	orderedBytes := reorderBytes(rawBytes, rule.ByteOrder)

	// 5. Parse to Value
	switch rule.DataType {
	case "int16":
		return int16(binary.BigEndian.Uint16(orderedBytes)), nil
	case "uint16":
		return binary.BigEndian.Uint16(orderedBytes), nil
	case "int32":
		return int32(binary.BigEndian.Uint32(orderedBytes)), nil
	case "uint32":
		return binary.BigEndian.Uint32(orderedBytes), nil
	case "int64":
		return int64(binary.BigEndian.Uint64(orderedBytes)), nil
	case "uint64":
		return binary.BigEndian.Uint64(orderedBytes), nil
	case "float32":
		bits := binary.BigEndian.Uint32(orderedBytes)
		return float64(math.Float32frombits(bits)), nil
	case "float64":
		bits := binary.BigEndian.Uint64(orderedBytes)
		return math.Float64frombits(bits), nil
	case "bool":
		// Use BitIndex
		var val uint16
		if len(orderedBytes) == 2 {
			val = binary.BigEndian.Uint16(orderedBytes)
		} else {
			val = uint16(orderedBytes[0])
		}

		if rule.BitIndex < 0 || rule.BitIndex > 15 {
			return nil, errors.New("invalid bit_index")
		}
		return ((val >> rule.BitIndex) & 0x01) == 1, nil
	case "string":
		// Remove trailing nulls? Or just return string
		// Let's trim null bytes
		str := string(orderedBytes)
		// trim nulls
		for idx := 0; idx < len(str); idx++ {
			if str[idx] == 0 {
				return str[:idx], nil
			}
		}
		return str, nil
	case "hex":
		return orderedBytes, nil // Return []byte
	default:
		return nil, errors.New("unsupported data type")
	}
}

// EvaluateRead executes a read expression: val = f(x)
func EvaluateRead(expression string, x interface{}) (interface{}, error) {
	if expression == "" {
		return x, nil
	}

	env := map[string]interface{}{
		"x": x,
	}

	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// EvaluateWrite executes a write expression: val = f(x)
// x is the value from Thing Model (e.g. 25.5)
// The result is the value to be written to Modbus register (e.g. 255)
func EvaluateWrite(expression string, x interface{}) (interface{}, error) {
	if expression == "" {
		return x, nil
	}

	env := map[string]interface{}{
		"x": x,
	}

	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// Decode parses the raw bytes according to the configuration
func Decode(data []byte, config PointConfig) (interface{}, error) {
	// 1. Calculate data length based on type
	typeLen := getTypeLength(config.DataType)
	if typeLen == 0 {
		return nil, errors.New("unsupported data type")
	}

	// 2. Check boundary
	start := config.Offset
	end := start + typeLen
	if end > len(data) {
		return nil, errors.New("data index out of range")
	}
	rawBytes := make([]byte, typeLen)
	copy(rawBytes, data[start:end])

	// 3. Handle Byte Order (Reordering)
	// For 16-bit: ABCD (Big), DCBA (Little)
	// For 32-bit: ABCD (Big), DCBA (Little), CDAB (Mid-Big), BADC (Mid-Little)
	orderedBytes := reorderBytes(rawBytes, config.ByteOrder)

	// 4. Parse to Number
	var val float64
	switch config.DataType {
	case "int16":
		val = float64(int16(binary.BigEndian.Uint16(orderedBytes)))
	case "uint16":
		val = float64(binary.BigEndian.Uint16(orderedBytes))
	case "int32":
		val = float64(int32(binary.BigEndian.Uint32(orderedBytes)))
	case "uint32":
		val = float64(binary.BigEndian.Uint32(orderedBytes))
	case "float32":
		bits := binary.BigEndian.Uint32(orderedBytes)
		val = float64(math.Float32frombits(bits))
	case "float64":
		bits := binary.BigEndian.Uint64(orderedBytes)
		val = math.Float64frombits(bits)
	case "bool":
		if rawBytes[0] > 0 {
			return true, nil
		}
		return false, nil
	default:
		return nil, errors.New("unsupported data type for numeric conversion")
	}

	return val, nil
}

// GetTypeLength returns the number of bytes for a given data type
func GetTypeLength(dataType string) int {
	switch dataType {
	case "bool":
		return 1 // Usually coil is 1 bit, but here we assume byte-aligned for simplicity or mapped from register
	case "int16", "uint16":
		return 2
	case "int32", "uint32", "float32":
		return 4
	case "float64":
		return 8
	default:
		return 0
	}
}

func getTypeLength(dataType string) int {
	return GetTypeLength(dataType)
}

func reorderBytes(in []byte, order string) []byte {
	out := make([]byte, len(in))
	copy(out, in)

	if len(in) == 2 {
		switch order {
		case "DCBA": // Little Endian
			out[0], out[1] = in[1], in[0]
		default: // ABCD (Big Endian) - Default
		}
	} else if len(in) == 4 {
		switch order {
		case "DCBA": // Little Endian
			out[0], out[1], out[2], out[3] = in[3], in[2], in[1], in[0]
		case "CDAB": // Mid-Big (Word Swap)
			out[0], out[1], out[2], out[3] = in[2], in[3], in[0], in[1]
		case "BADC": // Mid-Little (Byte Swap)
			out[0], out[1], out[2], out[3] = in[1], in[0], in[3], in[2]
		default: // ABCD (Big Endian) - Default
		}
	} else if len(in) == 8 {
		// For 64-bit, simplistic approach: Reverse if Little Endian
		if order == "DCBA" {
			for i := 0; i < 8; i++ {
				out[i] = in[7-i]
			}
		}
	}

	return out
}

// CalculateCRC16 calculates the Modbus CRC16 checksum
func CalculateCRC16(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if (crc & 0x0001) != 0 {
				crc >>= 1
				crc ^= 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	// Note: The standard algorithm returns the bytes in reversed order compared to the Modbus spec's "Value".
	// Or rather, it returns the value such that BigEndian writing gives "CD C5".
	// But Modbus wants Low Byte first.
	// If we return the calculated state directly, it is 0xCDC5.
	// We want 0xC5CD so that LittleEndian writing gives "CD C5".
	// UPDATE: The user reported that "CD C5" is WRONG and "C5 CD" is CORRECT.
	// To get "C5 CD" with LittleEndian.PutUint16, we need the input to be 0xCDC5 (Low=C5, High=CD).
	// So we should return the raw CRC without swapping.
	return crc
}
