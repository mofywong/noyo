package codec

import (
	"encoding/hex"
	"testing"
)

func TestCalculateCRC16(t *testing.T) {
	// Example from Modbus Spec or online calculator
	// 01 03 00 00 00 0A -> CRC C5 CD
	// Let's try a known string.
	// Request: 01 03 00 00 00 0A
	// Raw CRC calculation should be 0xCDC5.
	// Low Byte: C5, High Byte: CD.
	// When written as Little Endian (Low Byte first), it becomes C5 CD.

	data, _ := hex.DecodeString("01030000000A")
	expectedCRC := uint16(0xCDC5) // Value 52677

	crc := CalculateCRC16(data)

	if crc != expectedCRC {
		t.Errorf("Expected CRC %X, got %X", expectedCRC, crc)
	}
}

func TestEvaluateWrite(t *testing.T) {
	// Case 1: Simple Math (e.g. 25.5 -> 255)
	expression := "x * 10"
	x := 25.5
	val, err := EvaluateWrite(expression, x)
	if err != nil {
		t.Fatalf("EvaluateWrite failed: %v", err)
	}

	if v, ok := val.(float64); !ok || v != 255.0 {
		t.Errorf("Expected 255.0, got %v", val)
	}
}

func TestExtract(t *testing.T) {
	// Mock data: 00 01 00 02 00 03 00 04 (8 bytes = 4 registers)
	data := []byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04}

	// Case 1: uint16 at index 1 (00 02)
	rule := ExtractRule{
		RegisterIndex: 1,
		DataType:      "uint16",
		ByteOrder:     "ABCD",
	}
	val, err := Extract(data, rule)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if v, ok := val.(uint16); !ok || v != 2 {
		t.Errorf("Expected 2, got %v", val)
	}

	// Case 2: float32 at index 0 (00 01 00 02) -> very small number
	// 123.456 = 0x42F6E979 (IEEE 754)
	floatData, _ := hex.DecodeString("42F6E979")
	ruleFloat := ExtractRule{
		RegisterIndex: 0,
		DataType:      "float32",
		ByteOrder:     "ABCD",
	}
	valFloat, err := Extract(floatData, ruleFloat)
	if err != nil {
		t.Fatalf("Extract float failed: %v", err)
	}
	// Use epsilon for float comparison
	if v, ok := valFloat.(float64); !ok {
		t.Errorf("Expected float64, got %T", valFloat)
	} else {
		diff := v - 123.456
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("Expected 123.456, got %v", v)
		}
	}
}

func TestEvaluateRead(t *testing.T) {
	// Case 1: Simple Math
	expr := "x * 10 + 5"
	x := 2.0
	val, err := EvaluateRead(expr, x)
	if err != nil {
		t.Fatalf("EvaluateRead failed: %v", err)
	}

	// Expect float64
	if v, ok := val.(float64); !ok {
		// expr might return int if the result is integer-like?
		// But 2.0 is float.
		// If it returns int, we can accept it too.
		if vInt, ok := val.(int); ok {
			if vInt != 25 {
				t.Errorf("Expected 25, got %d", vInt)
			}
		} else if vInt64, ok := val.(int64); ok {
			if vInt64 != 25 {
				t.Errorf("Expected 25, got %d", vInt64)
			}
		} else {
			t.Errorf("Expected float64, got %T: %v", val, val)
		}
	} else if v != 25.0 {
		t.Errorf("Expected 25.0, got %v", v)
	}

	// Case 2: Bit extraction using uint16 + expression
	// Register value: 0x0004 (binary 0100) -> bit 2 is 1
	data := []byte{0x00, 0x04}
	rule := ExtractRule{
		RegisterIndex: 0,
		DataType:      "uint16",
		ByteOrder:     "ABCD",
	}
	rawVal, _ := Extract(data, rule) // should be 4
	exprStr := "x == 4"
	finalVal, err := EvaluateRead(exprStr, rawVal)
	if err != nil {
		t.Fatalf("EvaluateRead failed: %v", err)
	}
	if v, ok := finalVal.(bool); !ok || v != true {
		t.Errorf("Expected true, got %v", finalVal)
	}
}

func TestExtractFC1FC2(t *testing.T) {
	// Mock data for FC1/FC2 (single coil/input)
	data := []byte{0x01} // Bit 0 is 1
	rule := ExtractRule{
		RegisterIndex: 0,
		DataType:      "bool",
		BitIndex:      5, // Should be ignored if we enforce 0 in task_factory, but here we test Extract directly
	}

	// If BitIndex is 0
	rule.BitIndex = 0
	val, _ := Extract(data, rule)
	if val.(bool) != true {
		t.Errorf("FC1/FC2 bit 0: expected true, got %v", val)
	}

	// If BitIndex is 1 (out of range for 1 byte data if not careful, but Extract handles it)
	rule.BitIndex = 1
	val, _ = Extract(data, rule)
	if val.(bool) != false {
		t.Errorf("FC1/FC2 bit 1: expected false, got %v", val)
	}
}
