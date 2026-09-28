# nexus-stream

High-throughput, asynchronous media and telemetry processing pipeline engineered for deterministic performance, backpressure-controlled concurrency, and multi-runtime isolation.

```
       [ Client / Webhook Ingestion ]
                      │
              FastAPI Gateway (Python)
         (Schema validation, Bearer auth,
            async lifespan & Redis queue)
                      │
              [ Redis Job Queue ]
                      │
                      ▼
            Core Ingestion Worker (Go)
      (Goroutines pool, channel multiplexing,
       backpressure drop tracking, clean signal drain)
                      │
                      ▼ (CGO Static Link)
         High-Performance Kernel (Rust)
       (0xAA55 binary frame decoding, 4-way unrolled
        vector math, zero allocations in parser)
                      │
                      ▼
         [ Normalized Stream Output ]
```

## Architecture Overview

\`nexus-stream\` combines the rapid asynchronous ingress of Python with the concurrency and low-level processing capabilities of Go and Rust:

- **Ingestion Layer (Python / FastAPI):** Strict payload validation via Pydantic (\`1 <= samples <= 65535\`), mandatory Bearer token authentication, and non-blocking asynchronous queueing with \`redis.asyncio\` using modern lifespan management.
- **Worker Engine (Go):** Multi-threaded worker pool consuming from Redis with bounded channels, backpressure drop metrics via \`sync/atomic\`, and race-free graceful termination handling (\`SIGINT\`/\`SIGTERM\`).
- **Computational Kernel (Rust):** Statically linked into Go via CGO (\`libcore_parser.a\`). Decodes binary telemetry frames (\`0xAA55\` magic bytes) and calculates RMS and peak signal metrics using 4-way loop unrolling designed for LLVM auto-vectorization with zero heap allocations during parsing.

## Performance Benchmark

Measured locally via \`make bench\` executing 500 iterations over 16,384-sample frames (8,192,000 samples evaluated):

| Pipeline Stage | Implementation | Throughput | Frame Latency |
| :--- | :--- | :--- | :--- |
| **Pure Go Baseline** | Unrolled Go loop | ~1,500+ MSamples/s | ~10 µs / frame |
| **Go -> CGO -> Rust** | Static C ABI + Rust unrolled | ~800 - 1,200 MSamples/s | ~15 µs / frame |
| **Binary Framing** | Little-endian buffer serialization | N/A | ~20 - 30 µs / frame |

> *Note: CGO introduces a well-documented call boundary overhead (~50-100ns per invocation). In production high-throughput systems, frames are batched into multi-kilobyte buffers to amortize the boundary transition.*

## Project Structure

```
nexus-stream/
├── api/                  # Python FastAPI Ingestion Service
│   ├── app/
│   │   ├── main.py       # Async lifespan gateway & mandatory Bearer auth
│   │   └── schemas.py    # Pydantic schema with protocol limits (max 65535)
│   ├── tests/            # Asynchronous test suite (pytest-asyncio)
│   ├── Dockerfile
│   └── requirements.txt
├── worker/               # Go High-Concurrency Engine
│   ├── cmd/
│   │   ├── main.go       # Worker pool, atomic backpressure & clean drain
│   │   └── main_test.go  # CGO Rust integration tests
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── core-rs/              # Rust Native Kernel / Computational Core
│   ├── include/          # Exported C ABI header (core_parser.h)
│   ├── src/lib.rs        # Binary frame decoder & vector math
│   └── Cargo.toml        # Staticlib compilation manifest
├── bench/                # Reproducible micro-benchmark suite
│   └── run_benchmarks.go # Go baseline vs CGO Rust comparator
├── .github/workflows/    # CI/CD automation pipeline (GitHub Actions)
├── docker-compose.yml    # Isolated container orchestrator
├── Makefile              # Automation targets (run, test, bench, down)
├── .env.example          # Runtime environment template
└── README.md
```

## Quickstart

### Prerequisites
- Docker Engine & Docker Compose
- Go 1.22+ and Rust (Cargo)
- Python 3.11+ with virtualenv

### Automated Verification

```bash
make test         # Runs test suite across Rust, Go (CGO), and Python
make bench        # Runs comparative Go vs CGO Rust benchmark
make run          # Starts isolated containers (ports 8085 and 6380)
make test-ingest  # Sends sample telemetry payload with Bearer authentication
make down         # Graceful shutdown
```

## License

MIT License. See [LICENSE](LICENSE) for details.
