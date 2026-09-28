# nexus-stream

High-throughput, asynchronous media and telemetry processing pipeline engineered for deterministic performance, bounded memory footprint, and horizontal scalability.

```
       [ Client / Webhook Ingestion ]
                      │
              FastAPI Gateway (Python)
         (Schema validation, auth tokens,
            async lifespan & dispatching)
                      │
              [ Redis Job Queue ]
                      │
                      ▼
            Core Ingestion Worker (Go)
      (Goroutines pool, channel multiplexing,
       backpressure & CGO orchestration)
                      │
                      ▼ (Zero-Copy Static Link)
         High-Performance Kernel (Rust)
       (Binary framing 0xAA55, unrolled SIMD,
       deterministic telemetry parsing)
                      │
                      ▼
         [ Normalized Stream Output ]
```

## Architecture Overview

\`nexus-stream\` bridges the rapid asynchronous ingress of Python with the memory safety, concurrency models, and raw throughput of compiled systems languages:

- **Ingestion Layer (Python / FastAPI):** Handles authenticated ingress, token validation (\`Bearer\`), and non-blocking task dispatching using modern async lifespan context into Redis.
- **Processing Engine (Go):** Manages worker pools, channel multiplexing, bounded buffer backpressure, and clean OS signal draining (\`SIGINT\`/\`SIGTERM\`) via \`sync.WaitGroup\`.
- **Compute Kernel (Rust):** High-speed computational module statically linked via CGO (\`libcore_parser.a\`). Parses binary frames with zero heap allocations during runtime loops and computes energy metrics with 4-way unrolled vector operations (SIMD-friendly).

## Performance & Benchmark Metrics

Evaluated locally using reproducible synthetic telemetry datasets (\`make bench\`):

| Metric | Go Baseline / Dynamic Memory | CGO + Rust Static Link (nexus-stream) | Observed Result |
| :--- | :--- | :--- | :--- |
| **Computational Throughput** | ~1,200 MSamples/sec | **1,564.45 MSamples/sec** | High-throughput saturation |
| **Binary Framing Overhead** | N/A | **50.75 µs** | per 32,768-sample frame |
| **Test Suite Latency** | Sequential script runs | **< 0.3s Total** | Unit tests in Rust, Go & Python |
| **Allocation Strategy** | Dynamic Heap Allocations | **Deterministic Stack / Zero-Copy** | Zero runtime memory leaks |

## Project Structure

```
nexus-stream/
├── api/                  # Python FastAPI Ingestion Service
│   ├── app/
│   │   ├── main.py       # Async lifespan gateway & auth routing
│   │   └── schemas.py    # Pydantic contract definitions
│   ├── tests/            # Pytest asynchronous integration suite
│   ├── Dockerfile
│   └── requirements.txt
├── worker/               # Go High-Concurrency Engine
│   ├── cmd/
│   │   ├── main.go       # CGO bindings, worker pool & graceful drain
│   │   └── main_test.go  # Unit & FFI integration tests
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── core-rs/              # Rust Native Kernel / Computational Core
│   ├── include/          # Exported C ABI header (core_parser.h)
│   ├── src/lib.rs        # Unrolled SIMD math & binary frame decoder
│   └── Cargo.toml        # Staticlib compilation manifest
├── bench/                # Reproducible benchmark suite
│   └── run_benchmarks.go # Signal processing throughput evaluator
├── .github/workflows/    # CI/CD automation pipeline (GitHub Actions)
├── docker-compose.yml    # Isolated container orchestrator
├── Makefile              # Automation targets (run, test, bench, down)
├── .env.example          # Runtime environment template
└── README.md
```

## Quickstart

### Prerequisites
- Docker Engine & Docker Compose
- Go 1.22+ and Rust (Cargo) for local verification
- Make

### Running via Docker Compose

```bash
# Clone the repository
git clone https://github.com/Martob13/nexus-stream.git
cd nexus-stream

# Build and launch all isolated services
make run
```

### Health Check & Ingestion Test

```bash
# Verify API gateway status (isolated port 8085)
curl -s http://localhost:8085/health

# Dispatch a test telemetry ingestion payload
curl -X POST http://localhost:8085/v1/telemetry   -H "Content-Type: application/json"   -d '{"stream_id": "sensor-alpha", "samples": [0.15, -0.42, 0.88, 1.25, -0.05], "rate_hz": 1000}'
```

### Verification & Automated Tests

```bash
make test         # Runs full test suite: cargo test, CGO go test, and pytest
make bench        # Executes throughput benchmark (16M samples evaluation)
docker logs nexus-worker  # Inspect Go worker pool processing latency
make down         # Gracefully terminate containers
```

## License

MIT License. See [LICENSE](LICENSE) for details.
