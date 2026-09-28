package main

/*
#cgo CFLAGS: -I../../core-rs/include
#cgo LDFLAGS: ${SRCDIR}/../../core-rs/target/release/libcore_parser.a -lpthread -ldl -lm
#include "core_parser.h"
*/
import "C"

import (
"context"
"encoding/binary"
"encoding/json"
"fmt"
"log"
"math"
"os"
"os/signal"
"sync"
"syscall"
"time"
"unsafe"

"github.com/redis/go-redis/v9"
)

type TelemetryJob struct {
StreamID  string    `json:"stream_id"`
Timestamp uint64    `json:"timestamp"`
Samples   []float64 `json:"samples"`
RateHz    int       `json:"rate_hz"`
}

func encodeBinaryFrame(timestamp uint64, samples []float64) []byte {
const headerSize = 12
payloadSize := len(samples) * 8
buf := make([]byte, headerSize+payloadSize)

buf[0] = 0x55
buf[1] = 0xAA
binary.LittleEndian.PutUint64(buf[2:10], timestamp)
binary.LittleEndian.PutUint16(buf[10:12], uint16(len(samples)))

for i, s := range samples {
bits := math.Float64bits(s)
offset := headerSize + (i * 8)
binary.LittleEndian.PutUint64(buf[offset:offset+8], bits)
}

return buf
}

func callRustEngine(frame []byte) (float64, float64, error) {
if len(frame) == 0 {
return 0, 0, fmt.Errorf("empty frame buffer")
}

var cMetrics C.SignalMetrics
rawPtr := (*C.uint8_t)(unsafe.Pointer(&frame[0]))
frameLen := C.size_t(len(frame))

ret := C.parse_and_compute_metrics(rawPtr, frameLen, &cMetrics)
if ret != 0 {
return 0, 0, fmt.Errorf("rust core execution failed with code: %d", int(ret))
}

return float64(cMetrics.rms), float64(cMetrics.peak), nil
}

func telemetryWorker(id int, ctx context.Context, jobs <-chan TelemetryJob, wg *sync.WaitGroup) {
defer wg.Done()
log.Printf("[Worker %d] Attached to CGO Rust engine. Listening...", id)

for {
select {
case <-ctx.Done():
log.Printf("[Worker %d] Shutdown received. Finishing queue...", id)
return
case job, ok := <-jobs:
if !ok {
log.Printf("[Worker %d] Queue closed, draining completed.", id)
return
}

start := time.Now()
rawFrame := encodeBinaryFrame(job.Timestamp, job.Samples)
rms, peak, err := callRustEngine(rawFrame)
elapsed := time.Since(start)

if err != nil {
log.Printf("[Worker %d] Stream: %s | ERROR in Rust FFI: %v", id, job.StreamID, err)
continue
}

log.Printf("[Worker %d] Stream: %s | Rust Processed: %d samples | RMS: %.4f | Peak: %.4f | Latency: %s\n",
id, job.StreamID, len(job.Samples), rms, peak, elapsed)
}
}
}

func main() {
redisHost := os.Getenv("REDIS_HOST")
if redisHost == "" {
redisHost = "localhost"
}
redisPort := os.Getenv("REDIS_PORT")
if redisPort == "" {
redisPort = "6380"
}

rdb := redis.NewClient(&redis.Options{
Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
})

ctx, cancel := context.WithCancel(context.Background())
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

const workerCount = 4
jobsChan := make(chan TelemetryJob, 500)
var wg sync.WaitGroup

for i := 1; i <= workerCount; i++ {
wg.Add(1)
go telemetryWorker(i, ctx, jobsChan, &wg)
}

// Dispatcher loop with backpressure control
go func() {
for {
select {
case <-ctx.Done():
return
default:
result, err := rdb.BRPop(ctx, 1*time.Second, "nexus:stream:jobs").Result()
if err != nil {
if err == redis.Nil || ctx.Err() != nil {
continue
}
time.Sleep(300 * time.Millisecond)
continue
}

var job TelemetryJob
if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
log.Printf("Invalid JSON frame dropped: %v", err)
continue
}

select {
case jobsChan <- job:
case <-time.After(150 * time.Millisecond):
log.Printf("BACKPRESSURE ALERT: Dropping stream %s to protect memory boundaries", job.StreamID)
case <-ctx.Done():
return
}
}
}
}()

log.Printf("Nexus Core Running. Dispatcher + %d workers linked with Rust SIMD engine. Awaiting signals...", workerCount)
sig := <-sigChan
log.Printf("Signal %v caught. Initiating graceful shutdown...", sig)

cancel()
close(jobsChan)
wg.Wait()
log.Println("All workers safely drained. Zero leak exit.")
}
