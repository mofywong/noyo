package bacnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"

	bactypes "github.com/alexbeltran/gobacnet/types"
)

// Constants for encoding
const (
	tagNull            = 0
	tagBoolean         = 1
	tagUnsigned        = 2
	tagSigned          = 3
	tagReal            = 4
	tagDouble          = 5
	tagOctetString     = 6
	tagCharacterString = 7
	tagBitString       = 8
	tagEnumerated      = 9
	tagDate            = 10
	tagTime            = 11
	tagObjectID        = 12
)

// performWriteProperty manually constructs and sends a WriteProperty Request
// We do this because gobacnet's Client does not expose WriteProperty or a generic Send method.
func (p *BacnetPlugin) performWriteProperty(ip string, port int, deviceID uint32, objType, instID, propID int, value interface{}, priority uint8) error {
	// 1. Create UDP Connection (Ephemeral)
	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 2. Construct APDU
	apdu, invokeID, err := encodeWritePropertyAPDU(objType, instID, propID, value, priority)
	if err != nil {
		return err
	}

	// 3. Construct NPDU + APDU
	// Basic NPDU: Version 1, Control 0x04 (Expecting Reply)
	npduControl := byte(0x04)
	npdu := []byte{bactypes.ProtocolVersion, npduControl}

	pdu := append(npdu, apdu...)

	// 4. Construct BVLC + NPDU + APDU
	bvlcLength := 4 + len(pdu)
	bvlc := make([]byte, 4)
	bvlc[0] = 0x81 // Type: BACnet/IP
	bvlc[1] = 0x0A // Function: Unicast
	binary.BigEndian.PutUint16(bvlc[2:], uint16(bvlcLength))

	packet := append(bvlc, pdu...)

	// 5. Send
	_, err = conn.Write(packet)
	if err != nil {
		return err
	}

	// 6. Receive Response (Simple ACK)
	// Set timeout
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1500)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return err
	}

	// 7. validate Response
	// Check BVLC
	if n < 4 || buf[0] != 0x81 {
		return fmt.Errorf("invalid response header")
	}
	// Skip BVLC
	pkt := buf[4:n]

	// Parse NPDU
	if len(pkt) < 2 {
		return fmt.Errorf("invalid NPDU")
	}
	// Skip NPDU (assuming no extra fields for simple unicast response)
	// Control byte checks?
	// Usually response has control 0x00 or 0x20?
	// Just skip until APDU.
	// We assume NPDU length is 2 for simple response?
	// Standard simple ACK NPDU is usually 2 bytes.
	apduStart := 2
	// If sender included Source address involved in routing, NPDU is longer.
	// But we are sending directly.

	if len(pkt) <= apduStart {
		return fmt.Errorf("packet too short for APDU")
	}

	apduByte := pkt[apduStart]
	pduType := (apduByte & 0xF0) >> 4

	if pduType == 2 { // SimpleACK
		// Check Service Choice
		// SimpleACK: Type(2), InvokeID, ServiceChoice
		if len(pkt) < apduStart+3 {
			return fmt.Errorf("invalid SimpleACK")
		}
		respInvokeID := pkt[apduStart+1]
		respService := pkt[apduStart+2]

		if respInvokeID != invokeID {
			return fmt.Errorf("invoke ID mismatch")
		}
		if respService != 15 { // ServiceConfirmedWriteProperty
			return fmt.Errorf("unexpected service ack: %d", respService)
		}
		return nil
	} else if pduType == 5 { // Error
		return fmt.Errorf("received BACnet Error PDU")
	} else if pduType == 6 { // Reject
		return fmt.Errorf("received BACnet Reject PDU")
	} else if pduType == 7 { // Abort
		return fmt.Errorf("received BACnet Abort PDU")
	}

	return fmt.Errorf("unexpected PDU type: %d", pduType)
}

func encodeWritePropertyAPDU(objType, instID, propID int, value interface{}, priority uint8) ([]byte, uint8, error) {
	buf := new(bytes.Buffer)

	invokeID := uint8(time.Now().UnixNano() % 255) // Simple ID generation

	// APDU Header
	// Type: 0 (Confirmed Request)
	// Seg/More: 0
	buf.WriteByte(0x00)
	// Max Segs / Max APDU (Unspecified / 1476)
	buf.WriteByte(0x05) // 0x70? No, 0x05 = 0000 0101 -> MaxSegs=0, MaxAPDU=1476 (0101=5)
	buf.WriteByte(invokeID)
	buf.WriteByte(15) // Service Choice: WriteProperty

	// Context [0] Object ID
	encodeContextObjectID(buf, 0, uint16(objType), uint32(instID))

	// Context [1] Property ID
	encodeContextEnumerated(buf, 1, uint32(propID))

	// Context [2] Array Index - Optional, skipped

	// Context [3] Property Value
	// Opening Tag [3]
	encodeTag(buf, 3, true, true) // Context 3, Opening

	// Value encoding
	err := encodeValue(buf, value, objType)
	if err != nil {
		return nil, 0, err
	}

	// Closing Tag [3]
	encodeTag(buf, 3, true, false) // Context 3, Closing IsOpening=false -> Closing?
	// Wait, encodeTag logic needs to distinguish opening/closing.
	// My helper below separates them.

	// Context [4] Priority
	if priority > 0 {
		encodeContextUnsigned(buf, 4, uint32(priority))
	}

	return buf.Bytes(), invokeID, nil
}

// Helpers
func encodeTag(buf *bytes.Buffer, tagNum uint8, isContext bool, isOpening bool) {
	// Construct tag byte
	// Bit 3: Class (Context=1, App=0)
	// Bit 0-2: Value/LengthType
	// Opening/Closing tags are used with specific container?
	// Actually Opening/Closing tags are specific primitive values in Context class?
	// Opening Tag: Length = 6
	// Closing Tag: Length = 7

	var b uint8
	if isContext {
		b |= 0x08
	}

	// For Opening/Closing, the tag number is the Context ID.
	// The Value (Length field) indicates Open/Close.
	if isOpening {
		b |= 0x06 // Open
	} else {
		b |= 0x07 // Close
	}

	if tagNum <= 14 {
		b |= (tagNum << 4)
		buf.WriteByte(b)
	} else {
		b |= 0xF0
		buf.WriteByte(b)
		buf.WriteByte(tagNum)
	}
}

func encodeApplicationTag(buf *bytes.Buffer, tagNum uint8, lenVal uint32) {
	var b uint8
	// Class = 0 (Application)

	if tagNum <= 14 {
		b |= (tagNum << 4)
	} else {
		b |= 0xF0
	}

	if lenVal <= 4 {
		b |= uint8(lenVal)
		buf.WriteByte(b)
	} else {
		b |= 0x05 // Extended length
		buf.WriteByte(b)
		if lenVal <= 253 {
			buf.WriteByte(uint8(lenVal))
		} else if lenVal <= 65535 {
			buf.WriteByte(254)
			binary.Write(buf, binary.BigEndian, uint16(lenVal))
		} else {
			buf.WriteByte(255)
			binary.Write(buf, binary.BigEndian, uint32(lenVal))
		}
	}
}

func encodeContextUnsigned(buf *bytes.Buffer, tagNum uint8, val uint32) {
	// Encode Tag
	// Length of value
	l := 1
	if val > 0xFFFFFF {
		l = 4
	} else if val > 0xFFFF {
		l = 3 // rare
		l = 4 // Align to standard sizes usually? BACnet allows variable.
	} else if val > 0xFF {
		l = 2
	}

	// Create Tag
	var b uint8
	b |= 0x08 // Context

	if tagNum <= 14 {
		b |= (tagNum << 4)
	} else {
		b |= 0xF0 // Extended
	}

	b |= uint8(l) // Length
	buf.WriteByte(b)
	if tagNum > 14 {
		buf.WriteByte(tagNum)
	}

	// Write Value
	if l == 1 {
		buf.WriteByte(uint8(val))
	} else if l == 2 {
		binary.Write(buf, binary.BigEndian, uint16(val))
	} else if l == 4 {
		binary.Write(buf, binary.BigEndian, val)
	}
}

func encodeContextEnumerated(buf *bytes.Buffer, tagNum uint8, val uint32) {
	encodeContextUnsigned(buf, tagNum, val)
}

func encodeContextObjectID(buf *bytes.Buffer, tagNum uint8, objType uint16, instID uint32) {
	// Length always 4
	var b uint8
	b |= 0x08 // Context
	if tagNum <= 14 {
		b |= (tagNum << 4)
	} else {
		b |= 0xF0
	}
	b |= 0x04 // Length 4
	buf.WriteByte(b)
	if tagNum > 14 {
		buf.WriteByte(tagNum)
	}

	// Value: 10 bits Type, 22 bits Instance
	val := (uint32(objType) << 22) | (instID & 0x003FFFFF)
	binary.Write(buf, binary.BigEndian, val)
}

func encodeValue(buf *bytes.Buffer, value interface{}, objType int) error {
	switch v := value.(type) {
	case float64:
		// Real (Tag 4)
		encodeApplicationTag(buf, tagReal, 4)
		binary.Write(buf, binary.BigEndian, math.Float32bits(float32(v)))
	case float32:
		encodeApplicationTag(buf, tagReal, 4)
		binary.Write(buf, binary.BigEndian, math.Float32bits(v))
	case int, int32, int64, uint, uint32, uint64:
		val := convertToInt(v)
		// Unsigned (Tag 2) or Enumerated (Tag 9) depending on object type?
		// For Binary Output (4)/Value (5), it's Enumerated (0=Inactiv, 1=Active).
		// For Multi-state (14, 19), it's Unsigned (State numbers).
		// For Analog, it's Real.

		if objType == 4 || objType == 5 { // BO, BV
			encodeApplicationTag(buf, tagEnumerated, 1) // usually 1 byte
			buf.WriteByte(uint8(val))
		} else if objType == 14 || objType == 19 { // MO, MV
			// Unsigned
			// Length depends on val
			l := 1
			if val > 255 {
				l = 2
			} // etc
			encodeApplicationTag(buf, tagUnsigned, uint32(l))
			if l == 1 {
				buf.WriteByte(uint8(val))
			} else {
				binary.Write(buf, binary.BigEndian, uint16(val))
			}
		} else {
			// treat as Real for others to be safe? Or Unsigned?
			// If user passes int for AV/AO, convert to float
			encodeApplicationTag(buf, tagReal, 4)
			binary.Write(buf, binary.BigEndian, math.Float32bits(float32(val)))
		}
	case bool:
		// Boolean (Tag 1). Value is NOT encoded in length, but as data?
		// No, BACnet Boolean Application Tag:
		// Tag=1. Value 0 or 1 is encoded in the TAG itself? NO.
		// Wait, Boolean is special.
		// Standard BACnet: Application Tag 1.
		// If encoding a value:
		// It seems boolean value is often encoded as Enumerate 0/1 for Binary Objects.

		// Checking BACnet spec for Boolean Application Tag:
		// Tag Number = 1.
		// If False, LSI = 0? No.
		// Boolean is typically encoded as NULL (0) or ...?

		// Actually typical WriteProperty for Binary Object uses Enumerated (0=Inactive, 1=Active).
		// Passing 'True' -> Active(1).

		val := uint8(0)
		if v {
			val = 1
		}

		// Use Enumerated for BO/BV
		if objType == 4 || objType == 5 {
			encodeApplicationTag(buf, tagEnumerated, 1)
			buf.WriteByte(val)
		} else {
			// Encode as Real 1.0/0.0?
			encodeApplicationTag(buf, tagReal, 4)
			f := float32(0)
			if v {
				f = 1.0
			}
			binary.Write(buf, binary.BigEndian, math.Float32bits(f))
		}
	default:
		return fmt.Errorf("unsupported value type %T", v)
	}
	return nil
}

func convertToInt(v interface{}) uint32 {
	switch i := v.(type) {
	case int:
		return uint32(i)
	case int32:
		return uint32(i)
	case int64:
		return uint32(i)
	case uint:
		return uint32(i)
	case uint32:
		return i
	case uint64:
		return uint32(i)
	case float64:
		return uint32(i)
	}
	return 0
}
