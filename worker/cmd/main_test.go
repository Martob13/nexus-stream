package main

import (
	"math"
	"testing"
)

func TestRustIntegrationViaCGO_Valid(t *testing.T) {
	samples := []float64{1.0, -2.0, 3.0, -4.0, 5.0}
	frame, err := encodeBinaryFrame(1695900000, samples)
	if err != nil {
		t.Fatalf("Failed to encode valid frame: %v", err)
	}

	rms, peak, err := callRustEngine(frame)
	if err != nil {
		t.Fatalf("FFI error on valid frame: %v", err)
	}

	if peak != 5.0 {
		t.Errorf("Expected peak 5.0, got %f", peak)
	}

	expectedRMS := math.Sqrt(55.0 / 5.0)
	if math.Abs(rms-expectedRMS) > 1e-5 {
		t.Errorf("Expected RMS %f, got %f", expectedRMS, rms)
	}
}

func TestEncodeBinaryFrame_EmptyAndOverflow(t *testing.T) {
	_, err := encodeBinaryFrame(1234, []float64{})
	if err == nil {
		t.Error("Expected error on empty samples, got nil")
	}

	overflowSamples := make([]float64, MaxSamplesPerFrame+1)
	_, err = encodeBinaryFrame(1234, overflowSamples)
	if err == nil {
		t.Error("Expected error on >65535 samples, got nil")
	}
}

func TestEncodeBinaryFrame_NaNAndInf(t *testing.T) {
	_, err := encodeBinaryFrame(1234, []float64{1.0, math.NaN()})
	if err == nil {
		t.Error("Expected error on NaN sample")
	}

	_, err = encodeBinaryFrame(1234, []float64{1.0, math.Inf(1)})
	if err == nil {
		t.Error("Expected error on +Inf sample")
	}
}

func TestCallRustEngine_CorruptMagic(t *testing.T) {
	badFrame := make([]byte, 12+(2*8))
	badFrame[0] = 0x00 // Invalid magic
	badFrame[1] = 0x00
	_, _, err := callRustEngine(badFrame)
	if err == nil {
		t.Error("Expected error on corrupt magic bytes from Rust kernel")
	}
}

func TestCallRustEngine_ShortBuffer(t *testing.T) {
	shortFrame := []byte{0x55, 0xAA, 0x01}
	_, _, err := callRustEngine(shortFrame)
	if err == nil {
		t.Error("Expected error on buffer smaller than header")
	}
}
