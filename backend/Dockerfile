FROM python:3.11-slim

WORKDIR /app

COPY requirement.txt .
RUN pip install --no-cache-dir -r requirement.txt

COPY main.py .
COPY src/ ./src/

EXPOSE 8000

ENV DB_CONNECTION=sqlite:///./smart_visitors.db
ENV AI_ARTIFACT_DIR=./artifacts

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
