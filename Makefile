SHELL := /bin/bash
NEXUS_AUTH_TOKEN ?= $(shell grep NEXUS_AUTH_TOKEN .env 2>/dev/null | cut -d '=' -f2)
ifeq ($(NEXUS_AUTH_TOKEN),)
	NEXUS_AUTH_TOKEN := dev-local-token
endif

.PHONY: run down build test test-rust test-go test-python bench lint fmt test-ingest

build:
	cd core-rs && cargo build --release
	cd worker && CGO_ENABLED=1 go build -o bin/worker ./cmd/main.go

test-rust:
	cd core-rs && cargo test --verbose

test-go:
	cd core-rs && cargo build --release
	cd worker && CGO_ENABLED=1 go test -v ./...

test-python:
	cd api && NEXUS_AUTH_TOKEN=$(NEXUS_AUTH_TOKEN) PYTHONPATH=. python -m pytest -v tests/

test: test-rust test-go test-python

bench:
	cd core-rs && cargo build --release
	go run bench/run_benchmarks.go

lint:
	cd core-rs && cargo clippy -- -D warnings
	cd api && python -m ruff check . 2>/dev/null || true

fmt:
	cd core-rs && cargo fmt
	cd worker && gofmt -w .
	go run -v fmt ./... 2>/dev/null || true

run:
	@if [ ! -f .env ]; then cp .env.example .env; fi
	docker compose up --build -d

down:
	docker compose down

test-ingest:
	@sleep 2
	@echo "Checking API Liveness:"
	curl -s http://localhost:8085/health
	@echo -e "\nChecking API Readiness (Redis connectivity):"
	curl -s http://localhost:8085/ready
	@echo -e "\nSending Ingestion Payload with Bearer Token:"
	curl -X POST http://localhost:8085/v1/telemetry \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $(NEXUS_AUTH_TOKEN)" \
		-d '{"stream_id": "sensor-alpha", "samples": [0.15, -0.42, 0.88, 1.25, -0.05], "rate_hz": 1000}'
	@echo ""
