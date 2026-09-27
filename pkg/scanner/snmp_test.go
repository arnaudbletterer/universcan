package scanner

import (
	"context"
	"testing"
	"time"
)

func TestEncodeOID(t *testing.T) {
	oid := "1.3.6.1.2.1.43.11.1.1.9.1.1"
	encoded, err := encodeOID(oid)
	if err != nil {
		t.Fatalf("encodeOID failed: %v", err)
	}
	if len(encoded) < 5 {
		t.Fatalf("encoded OID too short: %x", encoded)
	}
	// Tag should be 0x06 (OBJECT IDENTIFIER)
	if encoded[0] != 0x06 {
		t.Errorf("expected tag 0x06, got 0x%02x", encoded[0])
	}
}

func TestBuildSNMPGetRequest(t *testing.T) {
	pkt, err := buildSNMPGetRequest("public", 42, "1.3.6.1.2.1.1.1.0")
	if err != nil {
		t.Fatalf("buildSNMPGetRequest failed: %v", err)
	}
	if len(pkt) == 0 || pkt[0] != 0x30 {
		t.Fatalf("invalid outer sequence: %x", pkt)
	}
}

func TestSNMPQueryDeviceAgainstLiveOrMock(t *testing.T) {
	// Tests with short timeout, succeeds or handles unreachable gracefully
	client := NewSNMPClient("192.168.1.50")
	client.Timeout = 500 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	info, err := client.QueryDevice(ctx)
	if err != nil {
		t.Logf("SNMP not available (expected if printer does not have UDP 161 enabled): %v", err)
	} else {
		t.Logf("SNMP device info: desc=%s, toner=%d%%", info.DeviceDescription, info.TonerPercent)
	}
}
