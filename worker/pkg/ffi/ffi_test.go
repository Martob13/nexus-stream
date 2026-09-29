package ffi

import (
	"math"
	"testing"

	"github.com/Martob13/nexus-stream/worker/pkg/frame"
)

func TestCallRustEngine_Valid(t *testing.T) {
	samples := []float64{1.0, -2.0, 3.0, -4.0, 5.0}
	raw, err := frame.EncodeBinaryFrame(1695900000, samples)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	metrics, err := CallRustEngine(raw)
	if err != nil {
		t.Fatalf("FFI execution failed: %v", err)
	}

	if metrics.Peak != 5.0 {
		t.Errorf("Expected Peak 5.0, got %f", metrics.Peak)
	}
	expectedRMS := math.Sqrt(55.0 / 5.0)
	if math.Abs(metrics.RMS-expectedRMS) > 1e-5 {
		t.Errorf("Expected RMS %f, got %f", expectedRMS, metrics.RMS)
	}
}

func TestCallRustEngine_CorruptMagic(t *testing.T) {
	badFrame := make([]byte, frame.HeaderSize+(2*8))
	badFrame[0] = 0xFF
	_, err := CallRustEngine(badFrame)
	if err == nil {
		t.Error("Expected error on invalid magic bytes")
	}
}

func TestCallRustEngine_TruncatedHeader(t *testing.T) {
	shortFrame := []byte{0x55, 0xAA, 0x01}
	_, err := CallRustEngine(shortFrame)
	if err == nil {
		t.Error("Expected error on frame smaller than 16-byte header")
	}
}

func TestCallRustEngine_LengthMismatch(t *testing.T) {
	samples := []float64{1.0, 2.0}
	raw, _ := frame.EncodeBinaryFrame(123, samples)
	// Truncate payload by 1 byte
	_, err := CallRustEngine(raw[:len(raw)-1])
	if err == nil {
		t.Error("Expected length mismatch error code -4 from Rust")
	}
}
