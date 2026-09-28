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
       (0xAA55 binary frame decoding, 4-way loop
        unrolling, zero heap allocations in parser)
                      │
                      ▼
         [ Normalized Stream Output ]
```

## Architecture Overview

\`nexus-stream\` bridges the rapid asynchronous ingress of Python with the concurrency and low-level processing models of Go and Rust:

- **Ingestion Layer (Python / FastAPI):** Strict payload validation via Pydantic (\`1 <= samples <= 65535\`, NaN/Inf rejection, positive timestamps), mandatory Bearer token authentication with timing-safe comparison (\`secrets.compare_digest\`), separated \`/health\` (liveness) and \`/ready\` (readiness with Redis ping), and non-blocking asynchronous queueing via \`redis.asyncio\`.
- **Worker Engine (Go):** Multi-threaded worker pool consuming from Redis with bounded channels, backpressure drop metrics via \`sync/atomic\`, internal defensive protocol validation, and race-free graceful termination handling (\`SIGINT\`/\`SIGTERM\`).
- **Computational Kernel (Rust):** Statically linked into Go via CGO (\`libcore_parser.a\`). Decodes binary telemetry frames (\`0xAA55\` magic bytes) and calculates RMS and peak signal metrics using 4-way loop unrolling designed for LLVM auto-vectorization with zero heap allocations inside the parsing loop.

## Architectural Trade-offs: CGO Boundary vs. Native Execution

Micro-benchmarks (\`make bench\`) demonstrate that executing computations directly in pure Go eliminates the CGO context-switch overhead (~50-100ns per invocation). The hybrid Go + Rust architecture is intentionally designed for:

1. **Shared Computational Kernel:** The Rust kernel compiles to a self-contained static C-ABI library (\`libcore_parser.a\`), enabling identical, deterministic mathematical processing across disparate services (C++, Python FFI, Go, or embedded devices) without duplicating business logic.
2. **Deterministic Memory Guarantees:** The Rust parser guarantees zero heap allocations during signal ingestion, eliminating garbage collector pauses in the low-level parsing routine.

### Local Benchmark Results (Reference Machine)

Evaluated with 500 iterations over 16,384-sample frames (8,192,000 total samples):

| Stage | Implementation | Throughput | Latency / Frame |
| :--- | :--- | :--- | :--- |
| **Go Baseline Loop** | Direct iteration | ~1,518 MSamples/sec | ~10.78 µs |
| **Go -> CGO -> Rust** | Static C ABI + Unrolled Rust | ~1,035 MSamples/sec | ~15.82 µs |
| **Binary Framing** | Little-endian serialization | N/A | ~19.83 µs |

> *Note: Exact metrics depend on CPU architecture and memory cache. Run \`make bench\` to inspect your hardware performance.*

## Project Structure

```
nexus-stream/
├── api/                  # Python FastAPI Ingestion Service
│   ├── app/
│   │   ├── main.py       # Async lifespan gateway & timing-safe Bearer auth
│   │   └── schemas.py    # Pydantic schema with protocol limits (max 65535, NaN check)
│   ├── tests/            # Asynchronous test suite (pytest-asyncio)
│   ├── Dockerfile
│   └── requirements.txt
├── worker/               # Go High-Concurrency Engine
│   ├── cmd/
│   │   ├── main.go       # Worker pool, atomic backpressure & clean drain
│   │   └── main_test.go  # CGO Rust integration tests (happy & edge cases)
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile        # Multi-stage CGO + Rust build
├── core-rs/              # Rust Native Kernel / Computational Core
│   ├── include/          # Exported C ABI header (core_parser.h)
│   ├── src/lib.rs        # Binary frame decoder & vector math
│   └── Cargo.toml        # Staticlib compilation manifest
├── bench/                # Reproducible micro-benchmark suite
│   └── run_benchmarks.go # Go baseline vs CGO Rust comparator
├── .github/workflows/    # CI/CD automation pipeline with caching
├── docker-compose.yml    # Isolated container orchestrator
├── Makefile              # Automation targets (run, test, bench, down)
├── .env.example          # Runtime environment template
└── README.md
```

## Quickstart & Verification

```bash
make test         # Runs full test suite: cargo test, CGO go test, and pytest
make bench        # Runs comparative Go vs CGO Rust benchmark
make run          # Starts containerized stack with multi-stage worker build
make test-ingest  # Sends sample telemetry payload with Bearer authentication
make down         # Graceful shutdown
```

## License

MIT License. See [LICENSE](LICENSE) for details.
