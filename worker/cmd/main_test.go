package main

import (
"math"
"testing"
)

func TestRustIntegrationViaCGO(t *testing.T) {
samples := []float64{1.0, -2.0, 3.0, -4.0, 5.0}
frame := encodeBinaryFrame(1695900000, samples)

rms, peak, err := callRustEngine(frame)
if err != nil {
t.Fatalf("Unexpected Rust CGO error: %v", err)
}

if peak != 5.0 {
t.Errorf("Expected peak 5.0, got %f", peak)
}

expectedRMS := math.Sqrt(55.0 / 5.0)
if math.Abs(rms-expectedRMS) > 1e-5 {
t.Errorf("Expected RMS %f, got %f", expectedRMS, rms)
}
}
