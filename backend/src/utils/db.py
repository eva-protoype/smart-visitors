import os
import re

from sqlalchemy import create_engine
from sqlalchemy.orm import declarative_base, sessionmaker

from src.utils.settings import settings  # Based on the video's setup

# 1. Add connect_args ONLY for SQLite
connect_args = {}
if settings.DB_CONNECTION.startswith("sqlite"):
    connect_args = {"check_same_thread": False}
    # Ensure the parent directory of a file-backed SQLite DB exists
    # (e.g. ./data/ when DB_CONNECTION points into a Docker volume).
    m = re.match(r"sqlite:///(.+)", settings.DB_CONNECTION)
    if m and m.group(1) not in (":memory:", ""):
        os.makedirs(os.path.dirname(os.path.abspath(m.group(1))), exist_ok=True)

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
