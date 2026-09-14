from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from src.ai import controller
from src.ai.dtos import (
    AnomalyFlagsResponse,
    NLQueryRequest,
    NLQueryResponse,
    TrainResponse,
)
from src.utils.db import get_db

router = APIRouter(prefix="/ai", tags=["AI"])


@router.post("/anomaly/train", response_model=TrainResponse)
def train_anomaly(db: Session = Depends(get_db)):  # noqa: B008
    return controller.train(db)


@router.get("/anomaly/flags", response_model=AnomalyFlagsResponse)
def list_anomaly_flags(
    limit: int = 20,
    only_suspicious: bool = False,
    db: Session = Depends(get_db),  # noqa: B008
):
    return controller.flags(db, limit=limit, only_suspicious=only_suspicious)


@router.get("/anomaly/score/{log_id}")
def score_anomaly_log(log_id: str, db: Session = Depends(get_db)):  # noqa: B008
    return controller.score_one(log_id, db)


@router.post("/query", response_model=NLQueryResponse)
def nl_query(payload: NLQueryRequest, db: Session = Depends(get_db)):  # noqa: B008
    return controller.ask(payload.question, db, limit=payload.limit)
