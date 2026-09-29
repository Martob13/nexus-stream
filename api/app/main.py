import logging
import os
import secrets
from contextlib import asynccontextmanager
from typing import Annotated
from fastapi import FastAPI, Header, HTTPException, Request, Response, status
from starlette.middleware.base import BaseHTTPMiddleware
import redis.asyncio as aioredis
from app.schemas import TelemetryPayload, TelemetryResponse

logger = logging.getLogger("nexus.api")
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")

REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", "6379"))
REDIS_URL = f"redis://{REDIS_HOST}:{REDIS_PORT}"

AUTH_TOKEN = os.environ.get("NEXUS_AUTH_TOKEN")
if not AUTH_TOKEN:
    raise RuntimeError("CRITICAL CONFIGURATION ERROR: NEXUS_AUTH_TOKEN must be set in environment")

redis_client: aioredis.Redis | None = None
MAX_BODY_SIZE = 2 * 1024 * 1024  # 2 MB limit to prevent OOM DoS attacks

class LimitUploadSizeMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next):
        content_length = request.headers.get("content-length")
        if content_length and int(content_length) > MAX_BODY_SIZE:
            # Use HTTP_413_CONTENT_TOO_LARGE to follow modern Starlette standards
            return Response(content="Payload Too Large", status_code=status.HTTP_413_CONTENT_TOO_LARGE)
        return await call_next(request)

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
        await redis_client.aclose()

app = FastAPI(title="Nexus Ingestion Gateway", version="1.0.0", lifespan=lifespan)
app.add_middleware(LimitUploadSizeMiddleware)

@app.get("/health")
async def health_check():
    return {"status": "alive", "service": "api-gateway"}

@app.get("/ready")
async def readiness_check():
    if not redis_client:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Redis client not initialized")
    try:
        await redis_client.ping()
        return {"status": "ready", "service": "api-gateway", "redis": "connected"}
    except Exception as exc:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail=f"Redis unreachable: {str(exc)}")

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

    # Timing-safe byte comparison resilient to multi-byte unicode strings
    token_bytes = parts[1].encode("utf-8")
    expected_bytes = AUTH_TOKEN.encode("utf-8")
    if not secrets.compare_digest(token_bytes, expected_bytes):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Unauthorized")

    if not redis_client:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Queue unavailable")

    try:
        job_data = payload.model_dump_json()
        await redis_client.lpush("nexus:stream:jobs", job_data)
    except Exception as exc:
        logger.error("Failed to enqueue telemetry job into Redis: %s", exc)
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Failed to enqueue telemetry job")

    return TelemetryResponse(
        status="queued",
        stream_id=payload.stream_id,
        queued_samples=len(payload.samples)
    )
