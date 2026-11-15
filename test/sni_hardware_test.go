package test

import (
	"bytes"
	"os"
	"testing"

	"goto_splitter/sni"
)

func TestSuperMetroidHardwareMemoryRead(t *testing.T) {
	addr := os.Getenv("GOTOSPLITTER_SNI_ADDRESS")
	if addr == "" {
		t.Skip("set GOTOSPLITTER_SNI_ADDRESS to run the hardware smoke test")
	}

	client := sni.NewClient(addr)
	if err := client.Connect(); err != nil {
		t.Fatalf("connect to SNI: %v", err)
	}
	defer client.Close()

	t.Logf("attached to %q", client.AttachedDevice())
	for _, sample := range []struct {
		offset        uint32
		size          int
		verifyAliases bool
	}{
		{offset: 0x0998, size: 1, verifyAliases: true},
		{offset: 0x0DE2, size: 1, verifyAliases: true},
		{offset: 0x079B, size: 2, verifyAliases: true},
		{offset: 0x09DA, size: 2},
		{offset: 0x09DC, size: 2},
		{offset: 0x09DE, size: 2},
		{offset: 0x09E0, size: 2},
	} {
		var firstValue []byte
		for i, address := range []uint32{sample.offset, 0x7E0000 + sample.offset, 0xF50000 + sample.offset} {
			data, err := client.ReadMemory(address, sample.size)
			if err != nil {
				t.Fatalf("read memory at 0x%X: %v", address, err)
			}
			if len(data) != sample.size {
				t.Fatalf("read memory at 0x%X: got %d bytes, want %d", address, len(data), sample.size)
			}
			t.Logf("WRAM request 0x%X = % X", address, data)
			if i == 0 {
				firstValue = data
			} else if sample.verifyAliases && !bytes.Equal(data, firstValue) {
				t.Fatalf("WRAM alias 0x%X returned % X, want % X from 0x%X", address, data, firstValue, sample.offset)
			}
		}
	}
}
