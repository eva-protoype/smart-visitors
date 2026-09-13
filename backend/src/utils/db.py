from sqlalchemy import create_engine
from sqlalchemy.orm import declarative_base, sessionmaker

from src.utils.settings import settings  # Based on the video's setup

# 1. Add connect_args ONLY for SQLite
connect_args = {}
if settings.DB_CONNECTION.startswith("sqlite"):
    connect_args = {"check_same_thread": False}

# 2. Pass connect_args to create_engine
engine = create_engine(
    settings.DB_CONNECTION,
    connect_args=connect_args
)

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
