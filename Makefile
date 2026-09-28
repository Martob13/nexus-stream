.PHONY: run test-ingest test-rust down

run:
	docker compose up --build -d

down:
	docker compose down

test-ingest:
	@sleep 2
	curl -s http://localhost:8085/health
	@echo ""
	curl -X POST http://localhost:8085/v1/telemetry -H "Content-Type: application/json" -d '{"stream_id": "sensor-alpha", "samples": [0.15, -0.42, 0.88, 1.25, -0.05], "rate_hz": 1000}'
	@echo ""

test-rust:
	cd core-rs && cargo test
