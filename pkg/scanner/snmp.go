package scanner

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// SNMPClient performs lightweight, pure-Go SNMP v1/v2c queries over UDP 161.
type SNMPClient struct {
	IP        string
	Port      int
	Community string
	Timeout   time.Duration
}

// NewSNMPClient returns a new SNMPClient with sane defaults.
func NewSNMPClient(ip string) *SNMPClient {
	return &SNMPClient{
		IP:        ip,
		Port:      161,
		Community: "public",
		Timeout:   1200 * time.Millisecond,
	}
}

// SNMPInfo holds metrics polled via SNMP.
type SNMPInfo struct {
	DeviceDescription string
	DisplayBuffer     string
	TonerLevel        int
	TonerMaxCapacity  int
	TonerPercent      int
}

// Common Printer MIB OIDs (RFC 3805 / RFC 1213 / Host Resources MIB)
const (
	OIDSysDescr           = "1.3.6.1.2.1.1.1.0"
	OIDDeviceDescr        = "1.3.6.1.2.1.25.3.2.1.3.1"
	OIDConsoleDisplay     = "1.3.6.1.2.1.43.16.5.1.2.1.1"
	OIDMarkerSuppliesMax  = "1.3.6.1.2.1.43.11.1.1.8.1.1"
	OIDMarkerSuppliesCur  = "1.3.6.1.2.1.43.11.1.1.9.1.1"
	OIDMarkerSuppliesDesc = "1.3.6.1.2.1.43.11.1.1.6.1.1"
)

// QueryDevice queries toner, status, and description over SNMP.
func (c *SNMPClient) QueryDevice(ctx context.Context) (*SNMPInfo, error) {
	addr := fmt.Sprintf("%s:%d", c.IP, c.Port)
	conn, err := net.DialTimeout("udp", addr, c.Timeout)
	if err != nil {
		return nil, fmt.Errorf("snmp dial failed: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(c.Timeout))
	}

	oids := []string{
		OIDDeviceDescr,
		OIDConsoleDisplay,
		OIDMarkerSuppliesMax,
		OIDMarkerSuppliesCur,
	}

	info := &SNMPInfo{}

	for idx, oid := range oids {
		reqPkt, err := buildSNMPGetRequest(c.Community, int32(idx+1), oid)
		if err != nil {
			continue
		}

		if _, err := conn.Write(reqPkt); err != nil {
			continue
		}

		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			continue
		}

		val, err := parseSNMPResponse(buf[:n])
		if err != nil {
			continue
		}

		switch oid {
		case OIDDeviceDescr:
			if s, ok := val.(string); ok {
				info.DeviceDescription = strings.TrimSpace(s)
			}
		case OIDConsoleDisplay:
			if s, ok := val.(string); ok {
				info.DisplayBuffer = strings.TrimSpace(s)
			}
		case OIDMarkerSuppliesMax:
			if v, ok := toInt(val); ok {
				info.TonerMaxCapacity = v
			}
		case OIDMarkerSuppliesCur:
			if v, ok := toInt(val); ok {
				info.TonerLevel = v
			}
		}
	}

	if info.TonerMaxCapacity > 0 && info.TonerLevel >= 0 {
		info.TonerPercent = (info.TonerLevel * 100) / info.TonerMaxCapacity
		if info.TonerPercent > 100 {
			info.TonerPercent = 100
		}
	}

	return info, nil
}

func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case uint32:
		return int(val), true
	case uint64:
		return int(val), true
	case string:
		i, err := strconv.Atoi(val)
		return i, err == nil
	}
	return 0, false
}

// --- Lightweight ASN.1 BER Encoder/Decoder ---

func buildSNMPGetRequest(community string, reqID int32, oid string) ([]byte, error) {
	encodedOID, err := encodeOID(oid)
	if err != nil {
		return nil, err
	}

	// VarBind: SEQUENCE { OID, NULL }
	varBindVal := append(encodedOID, 0x05, 0x00) // NULL tag + length 0
	varBind := encodeTLV(0x30, varBindVal)

	// VarBindList: SEQUENCE OF VarBind
	varBindList := encodeTLV(0x30, varBind)

	// PDU: GetRequest-PDU (0xA0) { reqID, errStatus(0), errIndex(0), varBindList }
	pduPayload := bytes.Buffer{}
	pduPayload.Write(encodeInteger(int64(reqID)))
	pduPayload.Write(encodeInteger(0))
	pduPayload.Write(encodeInteger(0))
	pduPayload.Write(varBindList)

	pdu := encodeTLV(0xA0, pduPayload.Bytes())

	// SNMP Message: SEQUENCE { version(1 for v2c), community, PDU }
	msgPayload := bytes.Buffer{}
	msgPayload.Write(encodeInteger(1)) // SNMPv2c = 1
	msgPayload.Write(encodeOctetString([]byte(community)))
	msgPayload.Write(pdu)

	return encodeTLV(0x30, msgPayload.Bytes()), nil
}

func encodeTLV(tag byte, val []byte) []byte {
	res := []byte{tag}
	l := len(val)
	if l < 128 {
		res = append(res, byte(l))
	} else if l < 256 {
		res = append(res, 0x81, byte(l))
	} else {
		res = append(res, 0x82, byte(l>>8), byte(l))
	}
	res = append(res, val...)
	return res
}

func encodeInteger(v int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(v))
	// Trim leading zeros, preserving sign
	start := 0
	for start < 7 && buf[start] == 0 && (buf[start+1]&0x80) == 0 {
		start++
	}
	return encodeTLV(0x02, buf[start:])
}

func encodeOctetString(b []byte) []byte {
	return encodeTLV(0x04, b)
}

func encodeOID(oidStr string) ([]byte, error) {
	parts := strings.Split(strings.Trim(oidStr, "."), ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid oid: %s", oidStr)
	}

	first, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, err
	}
	second, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}

	var raw []byte
	raw = append(raw, byte(first*40+second))

	for i := 2; i < len(parts); i++ {
		val, err := strconv.ParseUint(parts[i], 10, 32)
		if err != nil {
			return nil, err
		}
		// Base-128 variable length
		var chunks []byte
		chunks = append(chunks, byte(val&0x7F))
		val >>= 7
		for val > 0 {
			chunks = append([]byte{byte(val&0x7F | 0x80)}, chunks...)
			val >>= 7
		}
		raw = append(raw, chunks...)
	}

	return encodeTLV(0x06, raw), nil
}

func parseSNMPResponse(packet []byte) (any, error) {
	if len(packet) < 4 || packet[0] != 0x30 {
		return nil, fmt.Errorf("invalid snmp packet")
	}

	// Read outer sequence
	_, rem, err := decodeTLV(packet)
	if err != nil {
		return nil, err
	}

	// Version
	_, rem, err = decodeTLV(rem)
	if err != nil {
		return nil, err
	}

	// Community
	_, rem, err = decodeTLV(rem)
	if err != nil {
		return nil, err
	}

	// PDU (0xA2 GetResponse)
	if len(rem) == 0 || rem[0] != 0xA2 {
		return nil, fmt.Errorf("expected GetResponse PDU")
	}
	pduVal, _, err := decodeTLV(rem)
	if err != nil {
		return nil, err
	}

	// In PDU: reqID, errStatus, errIndex, VarBindList
	_, pduRem, _ := decodeTLV(pduVal) // skip reqID
	errStatusVal, pduRem, _ := decodeTLV(pduRem)
	if len(errStatusVal) > 0 && errStatusVal[0] != 0 {
		return nil, fmt.Errorf("snmp error-status %d", errStatusVal[0])
	}
	_, pduRem, _ = decodeTLV(pduRem) // skip errIndex

	// VarBindList
	varBindListVal, _, err := decodeTLV(pduRem)
	if err != nil || len(varBindListVal) == 0 {
		return nil, fmt.Errorf("empty varbind list")
	}

	// First VarBind
	varBindVal, _, err := decodeTLV(varBindListVal)
	if err != nil {
		return nil, err
	}

	// In VarBind: OID, then Value
	_, valTLVs, err := decodeTLV(varBindVal)
	if err != nil || len(valTLVs) == 0 {
		return nil, fmt.Errorf("no value in varbind")
	}

	valTag := valTLVs[0]
	valBytes, _, err := decodeTLV(valTLVs)
	if err != nil {
		return nil, err
	}

	switch valTag {
	case 0x02, 0x41, 0x42, 0x43: // Integer, Counter32, Gauge32, TimeTicks
		var num int64
		for _, b := range valBytes {
			num = (num << 8) | int64(b)
		}
		return num, nil
	case 0x04: // Octet String
		return string(valBytes), nil
	default:
		return string(valBytes), nil
	}
}

func decodeTLV(data []byte) (val []byte, remainder []byte, err error) {
	if len(data) < 2 {
		return nil, nil, fmt.Errorf("data too short for tlv")
	}
	lengthByte := data[1]
	headerLen := 2
	var contentLen int

	if lengthByte < 128 {
		contentLen = int(lengthByte)
	} else {
		numBytes := int(lengthByte & 0x7F)
		headerLen += numBytes
		if len(data) < headerLen {
			return nil, nil, fmt.Errorf("data too short for multi-byte length")
		}
		for i := 0; i < numBytes; i++ {
			contentLen = (contentLen << 8) | int(data[2+i])
		}
	}

	if len(data) < headerLen+contentLen {
		return nil, nil, fmt.Errorf("truncated tlv content")
	}

	val = data[headerLen : headerLen+contentLen]
	remainder = data[headerLen+contentLen:]
	return val, remainder, nil
}
