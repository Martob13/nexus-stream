package main

/*
#cgo CFLAGS: -I../core-rs/include
#cgo LDFLAGS: ${SRCDIR}/../core-rs/target/release/libcore_parser.a -lpthread -ldl -lm
#include "core_parser.h"
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"time"
	"unsafe"
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

func rustCGO(frame []byte) (float64, float64, error) {
	var cMetrics C.SignalMetrics
	rawPtr := (*C.uint8_t)(unsafe.Pointer(&frame[0]))
	frameLen := C.size_t(len(frame))

	ret := C.parse_and_compute_metrics(rawPtr, frameLen, &cMetrics)
	if ret != 0 {
		return 0, 0, fmt.Errorf("rust core failure: %d", int(ret))
	}
	return float64(cMetrics.rms), float64(cMetrics.peak), nil
}

func encodeFrame(timestamp uint64, samples []float64) []byte {
	buf := make([]byte, 12+(len(samples)*8))
	buf[0] = 0x55
	buf[1] = 0xAA
	binary.LittleEndian.PutUint64(buf[2:10], timestamp)
	binary.LittleEndian.PutUint16(buf[10:12], uint16(len(samples)))
	for i, s := range samples {
		binary.LittleEndian.PutUint64(buf[12+(i*8):12+(i*8)+8], math.Float64bits(s))
	}
	return buf
}

func main() {
	const frameSamples = 16384
	const iterations = 500
	totalSamples := frameSamples * iterations

	samples := make([]float64, frameSamples)
	for i := 0; i < frameSamples; i++ {
		samples[i] = rand.Float64()*2.0 - 1.0
	}
	frame := encodeFrame(uint64(time.Now().Unix()), samples)

	fmt.Println("===============================================================")
	fmt.Println("       NEXUS-STREAM BENCHMARK: GO BASELINE vs CGO RUST KERNEL   ")
	fmt.Printf("   Frame: %d samples | Iterations: %d | Total: %d samples\n", frameSamples, iterations, totalSamples)
	fmt.Println("===============================================================")

	startGo := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = goBaseline(samples)
	}
	elapsedGo := time.Since(startGo)
	throughputGo := float64(totalSamples) / elapsedGo.Seconds() / 1_000_000

	fmt.Printf("1. Go Pure Baseline     : %.2f MSamples/sec | Latency/frame: %s\n",
		throughputGo, elapsedGo/iterations)

	startRust := time.Now()
	for i := 0; i < iterations; i++ {
		_, _, _ = rustCGO(frame)
	}
	elapsedRust := time.Since(startRust)
	throughputRust := float64(totalSamples) / elapsedRust.Seconds() / 1_000_000

	fmt.Printf("2. Go -> CGO -> Rust FFI: %.2f MSamples/sec | Latency/frame: %s\n",
		throughputRust, elapsedRust/iterations)

	startFrame := time.Now()
	for i := 0; i < iterations; i++ {
		_ = encodeFrame(123456, samples)
	}
	elapsedFrame := time.Since(startFrame)
	fmt.Printf("3. Binary Framing Cost  : %s per frame\n", elapsedFrame/iterations)
	fmt.Println("===============================================================")
}
