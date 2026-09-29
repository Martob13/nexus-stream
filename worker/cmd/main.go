package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Martob13/nexus-stream/worker/pkg/ffi"
	"github.com/Martob13/nexus-stream/worker/pkg/frame"
	"github.com/redis/go-redis/v9"
)

type TelemetryJob struct {
	StreamID  string    `json:"stream_id"`
	Timestamp uint64    `json:"timestamp"`
	Samples   []float64 `json:"samples"`
	RateHz    int       `json:"rate_hz"`
}

func telemetryWorker(id int, jobs <-chan TelemetryJob, wg *sync.WaitGroup, processedCount *atomic.Uint64) {
	defer wg.Done()

	for job := range jobs {
		rawFrame, err := frame.EncodeBinaryFrame(job.Timestamp, job.Samples)
		if err != nil {
			slog.Warn("Rejected malformed job payload", "worker", id, "stream_id", job.StreamID, "err", err)
			continue
		}

		metrics, err := ffi.CallRustEngine(rawFrame)
		if err != nil {
			slog.Error("FFI execution failed", "worker", id, "stream_id", job.StreamID, "err", err)
			continue
		}

		count := processedCount.Add(1)
		if count%1000 == 0 || len(job.Samples) < 10 {
			slog.Info("Processed telemetry frame",
				"worker", id,
				"stream_id", job.StreamID,
				"samples", len(job.Samples),
				"rms", metrics.RMS,
				"peak", metrics.Peak,
				"total_processed", count,
			)
		}
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	workerCount := 4
	if envWorkers := os.Getenv("WORKER_COUNT"); envWorkers != "" {
		if parsed, err := strconv.Atoi(envWorkers); err == nil && parsed > 0 {
			workerCount = parsed
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

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
				// Bloqueo continuo (0) interrumpible de inmediato por context cancelation.
				// Elimina el warning de resolución de sub-segundos de go-redis.
				result, err := rdb.BRPop(ctx, 0, "nexus:stream:jobs").Result()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					time.Sleep(100 * time.Millisecond)
					continue
				}

				var job TelemetryJob
				if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
					slog.Warn("Corrupt JSON payload dropped", "err", err)
					continue
				}

				select {
				case jobsChan <- job:
				case <-time.After(100 * time.Millisecond):
					dropped := droppedCount.Add(1)
					slog.Warn("Backpressure drop event triggered", "stream_id", job.StreamID, "total_dropped", dropped)
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	slog.Info("Nexus Worker online", "worker_count", workerCount, "redis_addr", fmt.Sprintf("%s:%s", redisHost, redisPort))
	sig := <-sigChan
	slog.Info("Shutdown signal caught. Draining pipeline...", "signal", sig.String())

	cancel()
	dispatcherWg.Wait()
	close(jobsChan)
	workersWg.Wait()

	if err := rdb.Close(); err != nil {
		slog.Error("Error closing Redis connection", "err", err)
	}

	slog.Info("All workers safely stopped", "processed", processedCount.Load(), "dropped", droppedCount.Load())
}
