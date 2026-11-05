FROM golang:1.27-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /smart-visitors-api .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget \
    && addgroup -S app && adduser -S -G app app \
    && mkdir -p /data/photos /data/artifacts \
    && chown -R app:app /data
COPY --from=builder /smart-visitors-api /usr/local/bin/smart-visitors-api
USER app
ENV PORT=8000 \
    PHOTOS_DIR=/data/photos \
    AI_ARTIFACT_DIR=/data/artifacts
EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=20s \
    CMD wget -qO- http://127.0.0.1:8000/readyz >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/smart-visitors-api"]
