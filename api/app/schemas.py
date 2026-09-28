from pydantic import BaseModel, Field
from typing import List

class TelemetryPayload(BaseModel):
    stream_id: str = Field(..., description="Unique identifier for the telemetry stream")
    samples: List[float] = Field(..., description="Sensor readings / audio signal chunks")
    rate_hz: int = Field(default=1000, description="Sampling rate in Hertz")

class JobResponse(BaseModel):
    status: str
    stream_id: str
    queued_samples: int
