"""Thin orchestration for AI routes (Step 2). Heavy lifting lives in anomaly.py/text2sql.py."""

from fastapi import HTTPException
from sqlalchemy.orm import Session

from src.ai import anomaly, text2sql
from src.passes.models import AccessLogModel


def train(db: Session) -> dict:
    return anomaly.train_model(db)


def flags(db: Session, limit: int = 20, only_suspicious: bool = False) -> dict:
    return anomaly.score_recent(db, limit=limit, only_suspicious=only_suspicious)


def score_one(log_id: str, db: Session) -> dict:
    log = db.get(AccessLogModel, log_id)
    if log is None:
        raise HTTPException(status_code=404, detail="Access log not found")
    bundle = anomaly.load_bundle()
    return anomaly.score_log_row(log, db, bundle)


def ask(question: str, db: Session, limit: int = 20) -> dict:
    return text2sql.run_query(db, question=question, limit=limit)
