"""Pydantic schemas for Step 2 AI endpoints."""

from datetime import datetime

from pydantic import BaseModel, Field


class AnomalyFlag(BaseModel):
    log_id: str
    pass_id: str
    gate_id: str
    scan_time: datetime
    status: str
    anomaly_score: float
    is_suspicious: bool
    reasons: list[str] = Field(default_factory=list)


class AnomalyFlagsResponse(BaseModel):
    model: str
    count: int
    flags: list[AnomalyFlag]


class TrainResponse(BaseModel):
    model: str
    n_samples: int
    n_features: int = 7
    contamination: float
    artifact: str | None = None


class NLQueryRequest(BaseModel):
    question: str
    limit: int = 20


class NLQueryResponse(BaseModel):
    sql: str
    params: dict = Field(default_factory=dict)
    columns: list[str]
    rows: list[list] = Field(default_factory=list)
    answer: str
    warnings: list[str] = Field(default_factory=list)
