import os
import secrets
from contextlib import asynccontextmanager
from typing import Annotated
from fastapi import FastAPI, Header, HTTPException, status
import redis.asyncio as aioredis
from app.schemas import TelemetryPayload, TelemetryResponse

REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", "6379"))
REDIS_URL = f"redis://{REDIS_HOST}:{REDIS_PORT}"

# Strict Auth: fail immediately if not provided
AUTH_TOKEN = os.getenv("NEXUS_AUTH_TOKEN")
if not AUTH_TOKEN:
    # Allow fallback ONLY in local testing mode
    if os.getenv("TESTING") == "1":
        AUTH_TOKEN = "nexus-secret-key"
    else:
        raise RuntimeError("CRITICAL: NEXUS_AUTH_TOKEN environment variable must be set")

redis_client: aioredis.Redis | None = None

@asynccontextmanager
async def lifespan(app: FastAPI):
    global redis_client
    redis_client = aioredis.from_url(
        REDIS_URL,
        encoding="utf-8",
        decode_responses=True,
        max_connections=50
    )
    yield
    if redis_client:
        await redis_client.close()

app = FastAPI(title="Nexus Ingestion Gateway", version="1.0.0", lifespan=lifespan)

@app.get("/health")
async def health_check():
    """Liveness probe: verifies the HTTP server process is running."""
    return {"status": "alive", "service": "api-gateway"}

@app.get("/ready")
async def readiness_check():
    """Readiness probe: validates backend infrastructure (Redis) is operational."""
    if not redis_client:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Redis client not initialized")
    try:
        await redis_client.ping()
        return {"status": "ready", "service": "api-gateway", "redis": "connected"}
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail=f"Redis unreachable: {str(e)}")

@app.post(
    "/v1/telemetry",
    response_model=TelemetryResponse,
    status_code=status.HTTP_202_ACCEPTED
)
async def ingest_telemetry(
    payload: TelemetryPayload,
    authorization: Annotated[str | None, Header()] = None
):
    if not authorization:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Authorization header missing")

    parts = authorization.split(" ")
    if len(parts) != 2 or parts[0].lower() != "bearer":
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid token scheme")

    # Timing-safe constant-time comparison against timing attacks
    if not secrets.compare_digest(parts[1], AUTH_TOKEN):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Unauthorized")

    if not redis_client:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Queue unavailable")

    try:
        job_data = payload.model_dump_json()
        await redis_client.lpush("nexus:stream:jobs", job_data)
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Failed to enqueue telemetry job")

    return TelemetryResponse(
        status="queued",
        stream_id=payload.stream_id,
        queued_samples=len(payload.samples)
    )
