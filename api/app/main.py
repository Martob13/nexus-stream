import os
from contextlib import asynccontextmanager
from typing import Annotated
from fastapi import FastAPI, Header, HTTPException, status
import redis.asyncio as aioredis
from app.schemas import TelemetryPayload, TelemetryResponse

REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", "6379"))
REDIS_URL = f"redis://{REDIS_HOST}:{REDIS_PORT}"
AUTH_TOKEN = os.getenv("NEXUS_AUTH_TOKEN", "nexus-secret-key")

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
    return {"status": "ok", "service": "api-gateway"}

@app.post(
    "/v1/telemetry",
    response_model=TelemetryResponse,
    status_code=status.HTTP_202_ACCEPTED
)
async def ingest_telemetry(
    payload: TelemetryPayload,
    authorization: Annotated[str | None, Header()] = None
):
    # Mandatory Authentication: Missing or incorrect token yields 401
    if not authorization or authorization != f"Bearer {AUTH_TOKEN}":
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Unauthorized: Valid Bearer token required"
        )

    if not redis_client:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Queue backend unavailable"
        )

    job_data = payload.model_dump_json()
    await redis_client.lpush("nexus:stream:jobs", job_data)

    return TelemetryResponse(
        status="queued",
        stream_id=payload.stream_id,
        queued_samples=len(payload.samples)
    )
