import enum
import uuid

from sqlalchemy import Column, String
from sqlalchemy import Enum as SQLEnum

from src.utils.db import Base


class UserRole(str, enum.Enum):
    VISITOR = "visitor"
    HOST = "host"
    GUARD = "guard"
    ADMIN = "admin"

class UserModel(Base):
    __tablename__ = "users"

    id = Column(String, primary_key=True, default=lambda:str(uuid.uuid4()), index=True)

    username = Column(String, unique=True, nullable=False, index=True)

    aadhar_number = Column(String, unique=True, nullable=False, index=True)

    email = Column(String, unique=True, nullable=False, index=True)

    role = Column(SQLEnum(UserRole), default=UserRole.HOST, nullable=False)
