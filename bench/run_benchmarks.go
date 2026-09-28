package main

import (
"encoding/binary"
"fmt"
"math"
"math/rand"
"time"
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

func main() {
const frameSamples = 32768 // Fits comfortably within uint16 frame header
const iterations = 500
totalProcessed := frameSamples * iterations

samples := make([]float64, frameSamples)
for i := 0; i < frameSamples; i++ {
samples[i] = rand.Float64()*2.0 - 1.0
}

fmt.Println("==========================================================")
fmt.Println("   NEXUS-STREAM BENCHMARK SUITE: SIGNAL FRAME EVALUATION  ")
fmt.Printf("   Frame Size: %d f64 samples | Frames Processed: %d\n", frameSamples, iterations)
fmt.Printf("   Total Dataset Evaluated: %d samples\n", totalProcessed)
fmt.Println("==========================================================")

// Benchmark Go baseline
startGo := time.Now()
for i := 0; i < iterations; i++ {
_, _ = goBaseline(samples)
}
elapsedGo := time.Since(startGo)
throughputGo := float64(totalProcessed) / elapsedGo.Seconds()

fmt.Printf("Go Baseline Throughput : %.2f MSamples/sec (Total: %s)\n", throughputGo/1_000_000, elapsedGo)

// Memory framing test
startFrame := time.Now()
buf := make([]byte, 12+(frameSamples*8))
buf[0] = 0x55
buf[1] = 0xAA
binary.LittleEndian.PutUint64(buf[2:10], uint64(time.Now().Unix()))
binary.LittleEndian.PutUint16(buf[10:12], uint16(frameSamples))
for i, s := range samples {
binary.LittleEndian.PutUint64(buf[12+(i*8):12+(i*8)+8], math.Float64bits(s))
}
elapsedFrame := time.Since(startFrame)
fmt.Printf("Zero-Copy Binary Framing: %s per %d-sample frame\n", elapsedFrame, frameSamples)
fmt.Println("==========================================================")
}

