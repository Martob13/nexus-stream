package frame

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestEncodeBinaryFrame_Valid(t *testing.T) {
	samples := []float64{1.5, -2.5}
	ts := uint64(1700000000)
	buf, err := EncodeBinaryFrame(ts, samples)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if buf[0] != MagicByte0 || buf[1] != MagicByte1 {
		t.Errorf("Invalid magic bytes: %x %x", buf[0], buf[1])
	}
	if binary.LittleEndian.Uint64(buf[2:10]) != ts {
		t.Errorf("Timestamp mismatch")
	}
	if binary.LittleEndian.Uint16(buf[10:12]) != 2 {
		t.Errorf("Sample count mismatch")
	}
	if len(buf) != HeaderSize+(2*8) {
		t.Errorf("Expected buffer len %d, got %d", HeaderSize+(2*8), len(buf))
	}
}

func TestEncodeBinaryFrame_Boundaries(t *testing.T) {
	if _, err := EncodeBinaryFrame(1, []float64{}); err == nil {
		t.Error("Expected error on 0 samples")
	}

	overflow := make([]float64, MaxSamplesPerFrame+1)
	if _, err := EncodeBinaryFrame(1, overflow); err == nil {
		t.Error("Expected error on >65535 samples")
	}

	if _, err := EncodeBinaryFrame(1, []float64{1.0, math.NaN()}); err == nil {
		t.Error("Expected error on NaN")
	}

	if _, err := EncodeBinaryFrame(1, []float64{1.0, math.Inf(1)}); err == nil {
		t.Error("Expected error on +Inf")
	}

	if _, err := EncodeBinaryFrame(1, []float64{1.0, math.Inf(-1)}); err == nil {
		t.Error("Expected error on -Inf")
	}
}
