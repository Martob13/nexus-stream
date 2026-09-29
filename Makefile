SHELL := /bin/bash
NEXUS_AUTH_TOKEN ?= $(shell grep NEXUS_AUTH_TOKEN .env 2>/dev/null | cut -d '=' -f2)
ifeq ($(NEXUS_AUTH_TOKEN),)
	NEXUS_AUTH_TOKEN := dev-local-token
endif

.PHONY: run down build test test-rust test-go test-python bench lint fmt clean logs test-ingest

build:
	cd core-rs && cargo build --release
	cd worker && CGO_ENABLED=1 go build -o bin/worker ./cmd/main.go

test-rust:
	cd core-rs && cargo test --verbose
	cd core-rs && cargo clippy --all-targets -- -D warnings

test-go:
	cd core-rs && cargo build --release
	cd worker && CGO_ENABLED=1 go test -race -v ./...

test-python:
	cd api && NEXUS_AUTH_TOKEN=$(NEXUS_AUTH_TOKEN) PYTHONPATH=. python -m pytest -v tests/

test: test-rust test-go test-python

bench:
	cd core-rs && cargo build --release
	go run bench/run_benchmarks.go

lint:
	cd core-rs && cargo clippy --all-targets -- -D warnings
	test -z "$$(gofmt -l worker/ bench/)"

fmt:
	cd core-rs && cargo fmt
	gofmt -w worker/ bench/

clean:
	rm -rf core-rs/target worker/bin api/__pycache__ api/.pytest_cache api/app/__pycache__ api/tests/__pycache__
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true

logs:
	docker compose logs -f

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
