from fastapi import FastAPI

from src.ai.router import router as ai_router
from src.passes.models import AccessLogModel, PassModel
from src.passes.router import router as pass_router
from src.user.models import UserModel
from src.user.router import router as user_router
from src.utils.db import Base, engine

__all__ = ["AccessLogModel", "PassModel", "UserModel"]

Base.metadata.create_all(bind=engine)

app = FastAPI(title="Smart Visitor Access Control API")

app.include_router(pass_router)
app.include_router(user_router)
app.include_router(ai_router)
