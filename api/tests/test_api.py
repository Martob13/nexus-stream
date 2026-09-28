import pytest
from httpx import AsyncClient, ASGITransport
from app.main import app

@pytest.mark.asyncio
async def test_health_endpoint():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        response = await ac.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok", "service": "api-gateway"}

@pytest.mark.asyncio
async def test_invalid_auth():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        response = await ac.post(
            "/v1/telemetry",
            headers={"Authorization": "Bearer bad-token"},
            json={"stream_id": "test", "samples": [1.0, 2.0]}
        )
    assert response.status_code == 401

