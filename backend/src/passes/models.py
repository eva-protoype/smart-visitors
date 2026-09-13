import datetime
import enum
import uuid

from sqlalchemy import Column, DateTime, ForeignKey, String
from sqlalchemy import Enum as SQLEnum

from src.utils.db import Base

##----------------------------------------------------------------------------

class PassStatus(str, enum.Enum):
    active = "active"
    used = "used"
    expired = "expired"
    revoked = "revoked"

class PassModel(Base):
    __tablename__ = "passes"

    id = Column(String, primary_key=True,default=lambda:str(uuid.uuid4()),index=True)

    visitor_name = Column(String, nullable=False)

    visitor_email = Column(String, nullable=False)

    qr_token = Column(String, unique=True, index=True, default=lambda: str(uuid.uuid4()))

    status = Column(SQLEnum(PassStatus), default=PassStatus.active)

    host_user_id = Column(String, ForeignKey("users.id"), nullable=True)

    valid_until = Column(DateTime)

class AccessLogModel(Base):
    __tablename__ = "access_logs"

    id = Column(String, primary_key=True, default=lambda: str(uuid.uuid4()), index=True)

    pass_id = Column(String, ForeignKey("passes.id"), nullable=False)

    gate_id = Column(String, default="Main Gate") # Where they checked in[cite: 1]

    scan_time = Column(DateTime, default=datetime.datetime.utcnow) # When they checked in[cite: 1]

    status = Column(String, nullable=False) # "approved" or "denied"[cite: 1]
