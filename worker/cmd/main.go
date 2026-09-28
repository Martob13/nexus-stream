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
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/redis/go-redis/v9"
)

const (
	MaxSamplesPerFrame = 65535
	FrameHeaderSize    = 12
)

type TelemetryJob struct {
	StreamID  string    `json:"stream_id"`
	Timestamp uint64    `json:"timestamp"`
	Samples   []float64 `json:"samples"`
	RateHz    int       `json:"rate_hz"`
}

func encodeBinaryFrame(timestamp uint64, samples []float64) ([]byte, error) {
	n := len(samples)
	if n == 0 {
		return nil, errors.New("cannot encode empty samples")
	}
	if n > MaxSamplesPerFrame {
		return nil, fmt.Errorf("sample count %d exceeds protocol maximum %d", n, MaxSamplesPerFrame)
	}

	buf := make([]byte, FrameHeaderSize+(n*8))
	buf[0] = 0x55
	buf[1] = 0xAA
	binary.LittleEndian.PutUint64(buf[2:10], timestamp)
	binary.LittleEndian.PutUint16(buf[10:12], uint16(n))

	for i, s := range samples {
		if math.IsNaN(s) || math.IsInf(s, 0) {
			return nil, fmt.Errorf("sample index %d is not a finite number", i)
		}
		bits := math.Float64bits(s)
		offset := FrameHeaderSize + (i * 8)
		binary.LittleEndian.PutUint64(buf[offset:offset+8], bits)
	}

	return buf, nil
}

func callRustEngine(frame []byte) (float64, float64, error) {
	if len(frame) < FrameHeaderSize {
		return 0, 0, errors.New("frame buffer smaller than header")
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

func telemetryWorker(id int, jobs <-chan TelemetryJob, wg *sync.WaitGroup, processedCount *atomic.Uint64) {
	defer wg.Done()

	for job := range jobs {
		rawFrame, err := encodeBinaryFrame(job.Timestamp, job.Samples)
		if err != nil {
			log.Printf("[Worker %d] Invalid job rejected (%s): %v", id, job.StreamID, err)
			continue
		}

		rms, peak, err := callRustEngine(rawFrame)
		if err != nil {
			log.Printf("[Worker %d] Stream: %s | FFI ERROR: %v", id, job.StreamID, err)
			continue
		}

		count := processedCount.Add(1)
		// Periodic sampled logging to prevent I/O bottleneck in hot path
		if count%500 == 0 || len(job.Samples) < 10 {
			log.Printf("[Worker %d] Stream: %s | Samples: %d | RMS: %.4f | Peak: %.4f (Total: %d)",
				id, job.StreamID, len(job.Samples), rms, peak, count)
		}
	}
	log.Printf("[Worker %d] Stopped cleanly.", id)
}

func main() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	const workerCount = 4
	jobsChan := make(chan TelemetryJob, 1000)
	var workersWg sync.WaitGroup
	var dispatcherWg sync.WaitGroup
	var droppedCount atomic.Uint64
	var processedCount atomic.Uint64

	for i := 1; i <= workerCount; i++ {
		workersWg.Add(1)
		go telemetryWorker(i, jobsChan, &workersWg, &processedCount)
	}

	dispatcherWg.Add(1)
	go func() {
		defer dispatcherWg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				result, err := rdb.BRPop(ctx, 500*time.Millisecond, "nexus:stream:jobs").Result()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					if err == redis.Nil {
						continue
					}
					time.Sleep(200 * time.Millisecond)
					continue
				}

				var job TelemetryJob
				if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
					log.Printf("Malformed JSON dropped: %v", err)
					continue
				}

				select {
				case jobsChan <- job:
				case <-time.After(100 * time.Millisecond):
					droppedCount.Add(1)
					log.Printf("BACKPRESSURE ALERT: Dropped stream %s | Total: %d", job.StreamID, droppedCount.Load())
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	log.Printf("Nexus Engine online (%d workers). Standard Redis port: %s", workerCount, redisPort)
	sig := <-sigChan
	log.Printf("Signal %v received. Draining pipeline...", sig)

	cancel()
	dispatcherWg.Wait()
	close(jobsChan)
	workersWg.Wait()

	log.Printf("Shutdown complete. Processed: %d, Dropped: %d.", processedCount.Load(), droppedCount.Load())
}
