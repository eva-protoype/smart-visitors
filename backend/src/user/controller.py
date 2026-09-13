from fastapi import HTTPException
from sqlalchemy.orm import Session

from src.user.dtos import UserCreateSchema
from src.user.models import UserModel


def create_user(payload: UserCreateSchema, db: Session):
    existing = db.query(UserModel).filter(
        (UserModel.username == payload.username)
        | (UserModel.aadhar_number == payload.aadhar_number)
        | (UserModel.email == payload.email)
    ).first()
    if existing is not None:
        raise HTTPException(status_code=409, detail="User already exists")

    user = UserModel(
        username=payload.username,
        aadhar_number=payload.aadhar_number,
        email=payload.email,
        role=payload.role,
    )
    db.add(user)
    db.commit()
    db.refresh(user)
    return user