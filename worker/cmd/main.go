package main

import (
"context"
"encoding/json"
"fmt"
"log"
"math"
"os"
"sync"
"time"

"github.com/redis/go-redis/v9"
)

type TelemetryJob struct {
StreamID string    `json:"stream_id"`
Samples  []float64 `json:"samples"`
RateHz   int       `json:"rate_hz"`
}

func calculateRMS(samples []float64) float64 {
if len(samples) == 0 {
return 0.0
}
var sum float64
for _, v := range samples {
sum += v * v
}
return math.Sqrt(sum / float64(len(samples)))
}

func worker(id int, jobs <-chan TelemetryJob, wg *sync.WaitGroup) {
defer wg.Done()
for job := range jobs {
start := time.Now()
rms := calculateRMS(job.Samples)
log.Printf("[Worker %d] Stream: %s | Samples: %d | RMS: %.4f | Latency: %s\n",
id, job.StreamID, len(job.Samples), rms, time.Since(start))
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

ctx := context.Background()
jobsChan := make(chan TelemetryJob, 500)
var wg sync.WaitGroup

numWorkers := 4
for w := 1; w <= numWorkers; w++ {
wg.Add(1)
go worker(w, jobsChan, &wg)
}

log.Printf("Worker engine online. Listening for queue events with %d workers...\n", numWorkers)

for {
result, err := rdb.BRPop(ctx, 0*time.Second, "nexus:stream:jobs").Result()
if err != nil {
log.Printf("Queue read error: %v. Retrying...\n", err)
time.Sleep(1 * time.Second)
continue
}

var job TelemetryJob
if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
log.Printf("Failed to unmarshal payload: %v\n", err)
continue
}
jobsChan <- job
}
}
