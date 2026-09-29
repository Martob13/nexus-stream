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
	badFrame := make([]byte, 12+(2*8))
	badFrame[0] = 0xFF
	_, err := CallRustEngine(badFrame)
	if err == nil {
		t.Error("Expected error on invalid magic bytes")
	}
}
