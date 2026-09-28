import time
from pydantic import BaseModel, Field

class TelemetryPayload(BaseModel):
    stream_id: str = Field(..., min_length=1, max_length=64)
    timestamp: int = Field(default_factory=lambda: int(time.time()))
    samples: list[float] = Field(..., min_length=1, max_length=65535)
    rate_hz: int = Field(default=1000, gt=0)

class TelemetryResponse(BaseModel):
    status: str
    stream_id: str
    queued_samples: int
