import math
import time
from pydantic import BaseModel, Field, field_validator

class TelemetryPayload(BaseModel):
    stream_id: str = Field(..., min_length=1, max_length=64)
    timestamp: int = Field(default_factory=lambda: int(time.time()), gt=0)
    samples: list[float] = Field(..., min_length=1, max_length=65535)
    rate_hz: int = Field(default=1000, gt=0, le=500000)

    @field_validator("samples")
    @classmethod
    def validate_finite_samples(cls, v: list[float]) -> list[float]:
        for val in v:
            if math.isnan(val) or math.isinf(val):
                raise ValueError("Samples must not contain NaN or Infinite values")
        return v

class TelemetryResponse(BaseModel):
    status: str
    stream_id: str
    queued_samples: int
