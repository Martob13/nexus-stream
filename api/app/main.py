from fastapi import FastAPI, HTTPException
import redis
import json
import os
from .schemas import TelemetryPayload, JobResponse

app = FastAPI(title="Nexus-Stream Ingestion Gateway", version="1.0.0")

REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6380))
r = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, db=0)

@app.get("/health")
def health_check():
    return {"status": "ok", "service": "api-gateway"}

@app.post("/v1/telemetry", response_model=JobResponse)
def ingest_telemetry(payload: TelemetryPayload):
    try:
        job_data = {
            "stream_id": payload.stream_id,
            "samples": payload.samples,
            "rate_hz": payload.rate_hz
        }
        r.lpush("nexus:stream:jobs", json.dumps(job_data))
        return JobResponse(
            status="queued",
            stream_id=payload.stream_id,
            queued_samples=len(payload.samples)
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
