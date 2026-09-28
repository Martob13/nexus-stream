import pytest
from httpx import AsyncClient, ASGITransport
from unittest.mock import patch, AsyncMock
from app.main import app

AUTH_HEADER = {"Authorization": "Bearer nexus-secret-key"}

@pytest.mark.asyncio
async def test_health_endpoint():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        response = await ac.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok", "service": "api-gateway"}

@pytest.mark.asyncio
async def test_missing_auth():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        response = await ac.post(
            "/v1/telemetry",
            json={"stream_id": "test", "samples": [1.0, 2.0]}
        )
    assert response.status_code == 401

@pytest.mark.asyncio
async def test_invalid_auth():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        response = await ac.post(
            "/v1/telemetry",
            headers={"Authorization": "Bearer invalid-token"},
            json={"stream_id": "test", "samples": [1.0, 2.0]}
        )
    assert response.status_code == 401

@pytest.mark.asyncio
async def test_sample_limit_validation():
    # Attempting to send 0 samples
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        response = await ac.post(
            "/v1/telemetry",
            headers=AUTH_HEADER,
            json={"stream_id": "test", "samples": []}
        )
    assert response.status_code == 422
