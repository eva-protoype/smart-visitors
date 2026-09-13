import datetime

from fastapi import HTTPException
from sqlalchemy.orm import Session

from src.passes.dtos import PassCreateSchema
from src.passes.models import AccessLogModel, PassModel, PassStatus
from src.user.models import UserModel


def create_visitor_pass(payload: PassCreateSchema, db: Session):
    host = db.get(UserModel, payload.host_user_id)
    if host is None:
        raise HTTPException(status_code=404, detail="Host user not found")

    new_pass = PassModel(
        visitor_name=payload.visitor_name,
        visitor_email=payload.visitor_email,
        host_user_id=payload.host_user_id,
        valid_until=payload.valid_until,
    )
    db.add(new_pass)
    db.commit()
    db.refresh(new_pass)
    return new_pass


def scan_visitor_pass(qr_token: str, db: Session, gate_id: str = "Main Gate"):
    visitor_pass = db.query(PassModel).filter(PassModel.qr_token == qr_token).first()
    if visitor_pass is None:
        raise HTTPException(status_code=404, detail="Pass not found")

    scan_time = datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None)
    access = "denied"

    if visitor_pass.status == PassStatus.revoked:
        reason = "Pass has been revoked"
    elif visitor_pass.status == PassStatus.used:
        reason = "Pass has already been used"
    elif visitor_pass.status == PassStatus.expired or (
        visitor_pass.valid_until and visitor_pass.valid_until < scan_time
    ):
        visitor_pass.status = PassStatus.expired
        reason = "Pass has expired"
    else:
        visitor_pass.status = PassStatus.used
        access = "approved"
        reason = "Access granted"

    log = AccessLogModel(
        pass_id=visitor_pass.id,
        gate_id=gate_id,
        scan_time=scan_time,
        status=access,
    )
    db.add(visitor_pass)
    db.add(log)
    db.commit()
    db.refresh(visitor_pass)
    db.refresh(log)
    return {"access": access, "reason": reason, "log_id": log.id}
