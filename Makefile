.PHONY: run down test-ingest test test-go test-rust test-python bench

run:
	docker compose up --build -d

down:
	docker compose down

test-rust:
	cd core-rs && cargo test

test-go:
	cd core-rs && cargo build --release
	cd worker && CGO_ENABLED=1 go test -v ./...

test-python:
	cd api && PYTHONPATH=. python -m pytest -v tests/

test: test-rust test-go test-python

bench:
	go run bench/run_benchmarks.go

test-ingest:
	@sleep 2
	curl -s http://localhost:8085/health
	@echo ""
	curl -X POST http://localhost:8085/v1/telemetry -H "Content-Type: application/json" -d '{"stream_id": "sensor-alpha", "samples": [0.15, -0.42, 0.88, 1.25, -0.05], "rate_hz": 1000}'
	@echo ""
