from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from src.passes.controller import create_visitor_pass, scan_visitor_pass
from src.passes.dtos import PassCreateSchema, PassScanSchema
from src.utils.db import get_db

router = APIRouter(prefix="/passes", tags=["Passes"])


@router.post("/create")
def create_pass(payload: PassCreateSchema, db: Session = Depends(get_db)):  # noqa: B008
    return create_visitor_pass(payload=payload, db=db)


@router.post("/scan")
def scan_pass(payload: PassScanSchema, db: Session = Depends(get_db)):  # noqa: B008
    return scan_visitor_pass(
        qr_token=payload.qr_token, gate_id=payload.gate_id, db=db
    )