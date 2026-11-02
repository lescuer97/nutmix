package mint

import (
	"bytes"
	"testing"
	"time"
)

func TestNewQuoteUUIDFrom(t *testing.T) {
	entropy := make([]byte, 16)
	for i := range entropy {
		entropy[i] = byte(i * 13)
	}
	now := time.UnixMilli(0x123456789abc)
	id, err := newQuoteUUIDFrom(now, bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	ms := uint64(id[0])<<40 | uint64(id[1])<<32 | uint64(id[2])<<24 | uint64(id[3])<<16 | uint64(id[4])<<8 | uint64(id[5])
	if ms != uint64(now.UnixMilli()) {
		t.Fatalf("timestamp = %x", ms)
	}
	if id[6]>>4 != 7 || id[8]>>6 != 2 {
		t.Fatalf("wrong UUID version/variant: %x", id)
	}
	for i := 6; i < 16; i++ {
		mask := byte(0xff)
		if i == 6 {
			mask = 0x0f
		}
		if i == 8 {
			mask = 0x3f
		}
		if id[i]&mask != entropy[i]&mask {
			t.Fatalf("entropy byte %d not preserved", i)
		}
	}
	if _, err := newQuoteUUIDFrom(now, bytes.NewReader(nil)); err == nil {
		t.Fatal("expected entropy read error")
	}
}
