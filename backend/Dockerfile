FROM golang:1.24-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o /sv-go .

FROM python:3.11-slim
WORKDIR /app
# Python kept only for IsolationForest (ml/isolation.py).
RUN pip install --no-cache-dir scikit-learn>=1.3 numpy>=1.26
COPY --from=builder /sv-go /app/sv-go
COPY ml/ ./ml/
# Legacy FastAPI src/ kept for reference; Go binary is the runtime.
COPY src/ ./legacy-src/
COPY main.py ./legacy-main.py
RUN mkdir -p ./artifacts ./data
EXPOSE 8000
ENV DB_CONNECTION=sqlite:///./smart_visitors.db
ENV AI_ARTIFACT_DIR=./artifacts
ENV PYTHON_BIN=python3
ENV PORT=8000
CMD ["/app/sv-go"]
