package main

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/Martob13/nexus-stream/worker/pkg/ffi"
	"github.com/Martob13/nexus-stream/worker/pkg/frame"
)

func goBaseline(samples []float64) (float64, float64) {
	var sumSq, peak float64
	for _, v := range samples {
		sumSq += v * v
		a := math.Abs(v)
		if a > peak {
			peak = a
		}
	}
	return math.Sqrt(sumSq / float64(len(samples))), peak
}

func runBenchmarkSuite(frameSamples int, iterations int) {
	totalSamples := frameSamples * iterations
	samples := make([]float64, frameSamples)
	for i := 0; i < frameSamples; i++ {
		samples[i] = rand.Float64()*2.0 - 1.0
	}
	rawFrame, _ := frame.EncodeBinaryFrame(uint64(time.Now().Unix()), samples)

	fmt.Printf("\n--- Benchmark Scenario: Frame Size = %d samples (%d KB) | Total = %d samples ---\n",
		frameSamples, (frameSamples*8)/1024, totalSamples)

	// 1. Go Baseline
	startGo := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = goBaseline(samples)
	}
	elapsedGo := time.Since(startGo)
	throughputGo := float64(totalSamples) / elapsedGo.Seconds() / 1_000_000
	fmt.Printf("1. Pure Go Loop         : %.2f MSamples/sec | Latency/frame: %s\n",
		throughputGo, elapsedGo/time.Duration(iterations))

	// 2. Shared CGO Rust Kernel
	startRust := time.Now()
	for i := 0; i < iterations; i++ {
		metrics, err := ffi.CallRustEngine(rawFrame)
		if err != nil || metrics.Peak == 0 {
			panic("Rust execution failed")
		}
	}
	elapsedRust := time.Since(startRust)
	throughputRust := float64(totalSamples) / elapsedRust.Seconds() / 1_000_000
	fmt.Printf("2. Go -> CGO -> Rust FFI: %.2f MSamples/sec | Latency/frame: %s\n",
		throughputRust, elapsedRust/time.Duration(iterations))

	// 3. Framing Cost
	startFrame := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = frame.EncodeBinaryFrame(123456, samples)
	}
	elapsedFrame := time.Since(startFrame)
	fmt.Printf("3. Binary Framing Cost  : %s per frame\n", elapsedFrame/time.Duration(iterations))
}

func main() {
	fmt.Println("===============================================================")
	fmt.Println("       NEXUS-STREAM BENCHMARK & SYSTEM ENVIRONMENT REPORT      ")
	fmt.Printf("   OS: %s | Arch: %s | CPUs: %d | Go: %s\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version())
	fmt.Println("===============================================================")

	// Scenario A: Standard telemetry packet (128 samples ≈ 1 KB)
	runBenchmarkSuite(128, 50000)

	// Scenario B: High-density batch packet (16,384 samples ≈ 128 KB)
	runBenchmarkSuite(16384, 500)
	fmt.Println("===============================================================")
}
