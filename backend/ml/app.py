import hashlib
import hmac
import json
import math
import os
import pickle
import tempfile
import time
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any
from urllib.error import URLError
from urllib.parse import quote
from urllib.request import Request, urlopen

import numpy as np
from fastapi import Depends, FastAPI, HTTPException, Request as FastAPIRequest
from pydantic import BaseModel, Field
from sklearn.ensemble import IsolationForest

app = FastAPI(title="Smart Visitors anomaly advisor")
SERVICE_TOKEN = os.environ.get("ML_SERVICE_TOKEN", "")
API_URL = os.environ.get("API_URL", "http://api:8000").rstrip("/")
ARTIFACTS_DIR = Path(os.environ.get("ARTIFACTS_DIR", "/data/artifacts"))
MODEL_PATH = ARTIFACTS_DIR / "isolation_forest.pkl"
OFF_HOUR_START = int(os.environ.get("OFF_HOUR_START", "22"))
OFF_HOUR_END = int(os.environ.get("OFF_HOUR_END", "6"))


class TrainRequest(BaseModel):
    features: list[list[float]]
    contamination: float = Field(default=0.1, gt=0, le=0.5)
    gate_frequency: dict[str, float] = Field(default_factory=dict)


class ScanContext(BaseModel):
    log_id: str
    pass_id: str = ""
    gate_id: str = ""
    scan_time: datetime
    direction: str
    outcome: str


def canonical_request(timestamp: str, method: str, target: str, body: bytes) -> bytes:
    digest = hashlib.sha256(body).hexdigest()
    return f"{timestamp}\n{method}\n{target}\n{digest}".encode()


def signed_headers(method: str, target: str, body: bytes) -> dict[str, str]:
    if not SERVICE_TOKEN:
        raise HTTPException(status_code=503, detail="Service authentication is not configured")
    timestamp = str(int(time.time()))
    signature = hmac.new(
        SERVICE_TOKEN.encode(),
        canonical_request(timestamp, method, target, body),
        hashlib.sha256,
    ).hexdigest()
    return {"X-ML-Timestamp": timestamp, "X-ML-Signature": signature}


async def require_service_signature(request: FastAPIRequest) -> None:
    if not SERVICE_TOKEN:
        raise HTTPException(status_code=503, detail="Service authentication is not configured")
    timestamp = request.headers.get("X-ML-Timestamp", "")
    signature = request.headers.get("X-ML-Signature", "")
    try:
        age = abs(time.time() - int(timestamp))
    except ValueError as exc:
        raise HTTPException(status_code=401, detail="Invalid service authentication") from exc
    if age > 300:
        raise HTTPException(status_code=401, detail="Expired service authentication")
    body = await request.body()
    target = request.url.path
    if request.url.query:
        target += "?" + request.url.query
    expected = hmac.new(
        SERVICE_TOKEN.encode(),
        canonical_request(timestamp, request.method, target, body),
        hashlib.sha256,
    ).hexdigest()
    if not hmac.compare_digest(signature, expected):
        raise HTTPException(status_code=401, detail="Invalid service authentication")


def rule_score(features: list[float]) -> float:
    velocity = features[4] * 10
    hopping = max(0, features[5] * 10 - 1)
    score = -(
        0.45 * features[1]
        + 0.25 * min(velocity, 3) / 3
        + 0.20 * min(hopping, 3) / 3
        + 0.10 * features[6]
    )
    if features[2] == 1:
        score -= 0.05
    return max(-1, score)


def features_for_scan(
    scan: ScanContext,
    history: list[dict[str, Any]],
    gate_frequency: dict[str, float] | None = None,
) -> tuple[list[float], list[str]]:
    scanned_at = scan.scan_time.astimezone(timezone.utc)
    gates = [item.get("gate_id", "") for item in history if item.get("gate_id")]
    gate = scan.gate_id or ""
    if gate_frequency is not None:
        gate_rarity = 1 - gate_frequency.get(gate, 0)
    else:
        gate_counts = Counter(gates)
        gate_rarity = 1 - gate_counts.get(gate, 0) / max(1, len(gates))
    siblings = []
    for item in history:
        if item.get("log_id") == scan.log_id:
            continue
        raw_time = item.get("scan_time")
        try:
            item_time = datetime.fromisoformat(raw_time.replace("Z", "+00:00"))
        except (AttributeError, ValueError):
            continue
        if item_time.tzinfo is None:
            item_time = item_time.replace(tzinfo=timezone.utc)
        siblings.append((item, item_time.astimezone(timezone.utc)))
    siblings.append(({"gate_id": gate, "outcome": scan.outcome}, scanned_at))

    velocity = sum(abs((scanned_at - at).total_seconds()) <= 3600 for _, at in siblings)
    recent = [(item, at) for item, at in siblings if abs((scanned_at - at).total_seconds()) <= 86400]
    hopping = len({item.get("gate_id", "") for item, _ in recent if item.get("gate_id")} | ({gate} if gate else set()))
    denied = sum(item.get("outcome") == "denied" for item, _ in recent)
    off_hours = (
        OFF_HOUR_START <= scanned_at.hour < OFF_HOUR_END
        if OFF_HOUR_START <= OFF_HOUR_END
        else scanned_at.hour >= OFF_HOUR_START or scanned_at.hour < OFF_HOUR_END
    )
    weekend = scanned_at.weekday() >= 5
    is_denied = scan.outcome == "denied"
    vector = [
        scanned_at.hour / 23,
        float(off_hours),
        float(weekend),
        gate_rarity,
        min(velocity, 10) / 10,
        min(hopping, 10) / 10,
        float(is_denied),
    ]
    reasons = []
    if off_hours:
        reasons.append(f"off-hours {scan.direction} ({scanned_at.hour:02}:00)")
    if weekend:
        reasons.append("weekend scan")
    if velocity >= 3:
        reasons.append(f"rapid repeat scans: {velocity} in 1h")
    if hopping >= 3:
        reasons.append(f"gate hopping: {hopping} gates in 24h")
    elif hopping == 2:
        reasons.append("multi-gate use in 24h")
    if is_denied:
        reasons.append(f"{denied} denied in 24h" if denied > 1 else "denied scan")
    if gate_rarity >= 0.9:
        reasons.append(f"rare gate: {gate or 'unknown'}")
    return vector, reasons


def request_api(method: str, path: str, body: bytes = b"") -> bytes:
    headers = {"Content-Type": "application/json"}
    headers.update(signed_headers(method, path, body))
    request = Request(API_URL + path, data=body if method != "GET" else None, headers=headers, method=method)
    try:
        with urlopen(request, timeout=3) as response:
            return response.read(2 * 1024 * 1024)
    except (URLError, TimeoutError) as exc:
        raise HTTPException(status_code=502, detail="API callback failed") from exc


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/train", dependencies=[Depends(require_service_signature)])
def train(payload: TrainRequest) -> dict[str, Any]:
    if any(len(row) != 7 or any(not math.isfinite(value) for value in row) for row in payload.features):
        raise HTTPException(status_code=422, detail="Every feature vector must contain seven finite values")
    count = len(payload.features)
    if count < 10:
        MODEL_PATH.unlink(missing_ok=True)
        return {
            "model": "rules", "n_samples": count, "n_features": 7,
            "threshold": None, "trained_at": None,
        }

    model = IsolationForest(contamination=payload.contamination, random_state=42)
    model.fit(payload.features)
    scores = model.score_samples(payload.features)
    threshold = float(np.percentile(scores, 100 * payload.contamination))
    ARTIFACTS_DIR.mkdir(parents=True, exist_ok=True)
    bundle = {
        "model": model,
        "threshold": threshold,
        "n_samples": count,
        "contamination": payload.contamination,
        "gate_frequency": payload.gate_frequency,
        "trained_at": datetime.now(timezone.utc).isoformat(),
    }
    with tempfile.NamedTemporaryFile(dir=ARTIFACTS_DIR, delete=False) as artifact:
        temporary_path = Path(artifact.name)
        pickle.dump(bundle, artifact)
    temporary_path.replace(MODEL_PATH)
    return {
        "model": "isolation_forest", "n_samples": count, "n_features": 7,
        "threshold": threshold, "trained_at": bundle["trained_at"],
    }


@app.post("/score", dependencies=[Depends(require_service_signature)])
def score(scan: ScanContext) -> dict[str, Any]:
    history: list[dict[str, Any]] = []
    if scan.pass_id:
        path = f"/api/internal/scans/{quote(scan.pass_id, safe='')}/history?limit=5000"
        try:
            history = json.loads(request_api("GET", path)).get("items", [])
        except (json.JSONDecodeError, AttributeError) as exc:
            raise HTTPException(status_code=502, detail="API history response was invalid") from exc
    model_name = "rules"
    bundle = None
    if MODEL_PATH.exists():
        try:
            with MODEL_PATH.open("rb") as artifact:
                bundle = pickle.load(artifact)
        except (OSError, pickle.PickleError, EOFError):
            bundle = None
    gate_frequency = bundle.get("gate_frequency") if bundle else None
    vector, reasons = features_for_scan(scan, history, gate_frequency)
    if bundle:
        score_value = float(bundle["model"].score_samples([vector])[0])
        suspicious = score_value < float(bundle["threshold"])
        model_name = "isolation_forest"
    else:
        score_value = rule_score(vector)
        suspicious = score_value <= -0.3
    if suspicious and not reasons:
        reasons = ["statistical outlier for this facility"]

    result = {
        "score": round(score_value, 4), "is_suspicious": suspicious,
        "reasons": reasons, "model": model_name,
    }
    if scan.log_id:
        body = json.dumps({
            "score": result["score"], "is_suspicious": suspicious, "reasons": reasons,
        }, separators=(",", ":")).encode()
        request_api("POST", f"/api/internal/logs/{quote(scan.log_id, safe='')}/anomaly", body)
    return result
