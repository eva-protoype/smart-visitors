from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from src.user import controller
from src.user.dtos import UserCreateSchema
from src.utils.db import get_db

router = APIRouter(prefix="/users", tags=["Users"])


@router.post("/create")
def create_user(payload: UserCreateSchema, db: Session = Depends(get_db)):  # noqa: B008
    return controller.create_user(payload=payload, db=db)
