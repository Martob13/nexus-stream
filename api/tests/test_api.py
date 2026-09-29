import os
import pytest
from httpx import AsyncClient, ASGITransport
from pydantic import ValidationError

os.environ["TESTING"] = "1"
os.environ["NEXUS_AUTH_TOKEN"] = "nexus-secret-key"
from app.main import app
from app.schemas import TelemetryPayload

VALID_HEADERS = {
    "Authorization": "Bearer nexus-secret-key",
    "Content-Type": "application/json"
}

@pytest.mark.asyncio
async def test_health_liveness():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        res = await ac.get("/health")
    assert res.status_code == 200
    assert res.json()["status"] == "alive"

@pytest.mark.asyncio
async def test_readiness_failure_when_no_redis():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        res = await ac.get("/ready")
    assert res.status_code == 503

@pytest.mark.asyncio
async def test_auth_cases():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        # Missing auth
        r1 = await ac.post("/v1/telemetry", json={"stream_id": "s1", "samples": [1.0]})
        assert r1.status_code == 401

        # Invalid token
        r2 = await ac.post("/v1/telemetry", headers={"Authorization": "Bearer wrong"}, json={"stream_id": "s1", "samples": [1.0]})
        assert r2.status_code == 401

        # Invalid scheme
        r3 = await ac.post("/v1/telemetry", headers={"Authorization": "Basic 1234"}, json={"stream_id": "s1", "samples": [1.0]})
        assert r3.status_code == 401

        # Raw bytes header to verify ASGI server handles multibyte inputs gracefully without 500 error
        raw_headers = [
            (b"authorization", "Bearer túñç€密".encode("utf-8")),
            (b"content-type", b"application/json")
        ]
        r4 = await ac.post("/v1/telemetry", headers=raw_headers, json={"stream_id": "s1", "samples": [1.0]})
        assert r4.status_code == 401

@pytest.mark.asyncio
async def test_payload_too_large():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        headers = {**VALID_HEADERS, "content-length": str(3 * 1024 * 1024)}
        res = await ac.post("/v1/telemetry", headers=headers, json={"stream_id": "s1", "samples": [1.0]})
    assert res.status_code == 413

@pytest.mark.asyncio
async def test_sample_boundaries_and_rejections():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        # 0 samples -> 422
        r0 = await ac.post("/v1/telemetry", headers=VALID_HEADERS, json={"stream_id": "s", "samples": []})
        assert r0.status_code == 422

        # Negative timestamp -> 422
        r_time = await ac.post("/v1/telemetry", headers=VALID_HEADERS, json={"stream_id": "s", "timestamp": -5, "samples": [1.0]})
        assert r_time.status_code == 422

def test_pydantic_nan_and_inf_validation():
    with pytest.raises(ValidationError):
        TelemetryPayload(stream_id="test", samples=[1.0, float("nan")])

    with pytest.raises(ValidationError):
        TelemetryPayload(stream_id="test", samples=[1.0, float("inf")])

    with pytest.raises(ValidationError):
        TelemetryPayload(stream_id="test", samples=[1.0, float("-inf")])
