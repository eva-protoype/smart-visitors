"""Anomaly Detection Engine (Step 2.1).

- Feature builder over AccessLogModel rows (7 dims, see task.md §4).
- IsolationForest when history exists (>= MIN_TRAIN_SAMPLES), else transparent rules.
- Pickle persistence to AI_ARTIFACT_DIR/isolation_forest.pkl.
- Every score carries human-readable reasons (no black-box booleans alone).
"""

import datetime
import os
import pickle

from sqlalchemy.orm import Session

from src.passes.models import AccessLogModel
from src.utils.constant import MIN_TRAIN_SAMPLES, RULE_SUSPICIOUS_THRESHOLD
from src.utils.helpers import hours_between, is_off_hours
from src.utils.settings import settings

N_FEATURES = 7
ARTIFACT_NAME = "isolation_forest.pkl"


def artifact_path() -> str:
    return os.path.join(settings.AI_ARTIFACT_DIR, ARTIFACT_NAME)


def _ensure_artifact_dir() -> None:
    os.makedirs(settings.AI_ARTIFACT_DIR, exist_ok=True)


def get_gate_freq(db: Session) -> dict[str, float]:
    """Relative frequency of each gate_id across all logs (for gate_rarity)."""
    logs = db.query(AccessLogModel.gate_id).all()
    total = len(logs)
    if total == 0:
        return {}
    counts: dict[str, int] = {}
    for (gate_id,) in logs:
        key = gate_id or "Main Gate"
        counts[key] = counts.get(key, 0) + 1
    return {gate: count / total for gate, count in counts.items()}


def extract_features(
    log: AccessLogModel, db: Session, gate_freq: dict[str, float]
) -> tuple[list[float], list[str]]:
    """Build the 7-dim feature vector + human reasons for one log row."""
    scan_time = log.scan_time or datetime.datetime.utcnow()
    hour = float(scan_time.hour)
    off_hours = is_off_hours(scan_time)
    is_weekend = scan_time.weekday() >= 5

    gate_id = log.gate_id or "Main Gate"
    gate_rarity = 1.0 - gate_freq.get(gate_id, 0.0)

    # Same-pass velocity: scans of this pass within +-1h (capped at 10).
    velocity = 0
    hopping_set: set[str] = {gate_id}
    recent_denied = 0
    try:
        siblings = (
            db.query(AccessLogModel)
            .filter(AccessLogModel.pass_id == log.pass_id)
            .all()
        )
    except Exception:
        siblings = [log]
    for sib in siblings:
        sib_time = sib.scan_time or scan_time
        if hours_between(sib_time, scan_time) <= 1.0:
            velocity += 1
        if hours_between(sib_time, scan_time) <= 24.0:
            hopping_set.add(sib.gate_id or "Main Gate")
        if sib.status == "denied" and hours_between(sib_time, scan_time) <= 24.0:
            recent_denied += 1
    velocity = min(velocity, 10)
    hopping = min(len(hopping_set), 10)
    is_denied = 1.0 if log.status == "denied" else 0.0

    features = [
        hour / 23.0,  # normalised hour
        1.0 if off_hours else 0.0,
        1.0 if is_weekend else 0.0,
        float(gate_rarity),
        min(velocity, 10) / 10.0,
        min(hopping, 10) / 10.0,
        is_denied,
    ]

    reasons: list[str] = []
    if off_hours:
        reasons.append(f"off-hours entry ({scan_time.hour:02d}:00)")
    if is_weekend:
        reasons.append("weekend entry")
    if velocity >= 3:
        reasons.append(f"rapid repeat scans: {velocity} in 1h")
    if hopping >= 3:
        reasons.append(f"gate hopping: {hopping} gates in 24h")
    elif hopping == 2:
        reasons.append("multi-gate use in 24h")
    if is_denied:
        reasons.append("denied entry" if recent_denied <= 1 else f"{recent_denied} denied in 24h")
    if gate_rarity >= 0.9:
        reasons.append(f"rare gate: {gate_id}")

    return features, reasons


def rule_score(features: list[float], reasons: list[str]) -> float:
    """Transparent fallback score in [-1, 0]; suspicious if <= RULE_SUSPICIOUS_THRESHOLD."""
    # features: [hour_norm, off_hours, weekend, rarity, velocity_norm, hopping_norm, denied]
    score = -(
        0.45 * features[1]
        + 0.25 * min(features[4] * 10.0, 3.0) / 3.0
        + 0.20 * min(max(features[5] * 10.0 - 1.0, 0.0), 3.0) / 3.0
        + 0.10 * features[6]
    )
    if features[2]:  # small weekend bump so weekend-only anomalies still surface
        score -= 0.05
    return max(-1.0, score)


def _try_import_forest():
    try:
        from sklearn.ensemble import IsolationForest  # type: ignore

        return IsolationForest
    except Exception:
        return None


def train_model(db: Session) -> dict:
    """Fit IsolationForest on all access_logs; fall back to rules when data is thin."""
    logs = db.query(AccessLogModel).order_by(AccessLogModel.scan_time.asc()).all()
    n = len(logs)
    gate_freq = get_gate_freq(db)
    contamination = float(settings.ANOMALY_CONTAMINATION)

    if n < MIN_TRAIN_SAMPLES:
        return {
            "model": "rules",
            "n_samples": n,
            "n_features": N_FEATURES,
            "contamination": contamination,
            "artifact": None,
        }

    import numpy as np

    X = [extract_features(log, db, gate_freq)[0] for log in logs]
    arr = np.array(X, dtype=float)

    forest_cls = _try_import_forest()
    if forest_cls is None:  # sklearn missing -> rules mode, still a valid 200
        return {
            "model": "rules",
            "n_samples": n,
            "n_features": N_FEATURES,
            "contamination": contamination,
            "artifact": None,
        }

    model = forest_cls(contamination=contamination, random_state=42)
    model.fit(arr)
    train_scores = model.score_samples(arr)
    # Threshold at the contamination percentile: scores below it are suspicious.
    import numpy as _np

    threshold = float(_np.percentile(train_scores, 100.0 * contamination))

    _ensure_artifact_dir()
    bundle = {
        "model": model,
        "gate_freq": gate_freq,
        "threshold": threshold,
        "n_samples": n,
        "contamination": contamination,
        "trained_at": datetime.datetime.utcnow().isoformat() + "Z",
    }
    path = artifact_path()
    with open(path, "wb") as fh:
        pickle.dump(bundle, fh)
    return {
        "model": "isolation_forest",
        "n_samples": n,
        "n_features": N_FEATURES,
        "contamination": contamination,
        "artifact": path,
    }


def load_bundle() -> dict | None:
    path = artifact_path()
    if not os.path.exists(path):
        return None
    try:
        with open(path, "rb") as fh:
            return pickle.load(fh)
    except Exception:
        return None


def _score_with_bundle(
    features: list[float], bundle: dict | None
) -> tuple[float, bool, str]:
    """Return (score, is_suspicious, model_name)."""
    if bundle is not None and "model" in bundle and bundle["model"] is not None:
        try:
            import numpy as np

            score = float(bundle["model"].score_samples(np.array([features]))[0])
            return score, score < float(bundle.get("threshold", 0.0)), "isolation_forest"
        except Exception:
            pass
    s = rule_score(features, [])
    return s, s <= RULE_SUSPICIOUS_THRESHOLD, "rules"


def score_log_row(log: AccessLogModel, db: Session, bundle: dict | None) -> dict:
    gate_freq = (bundle or {}).get("gate_freq") or get_gate_freq(db)
    features, reasons = extract_features(log, db, gate_freq)
    score, suspicious, model_name = _score_with_bundle(features, bundle)
    # If the ML model says normal but strong rules fire, surface reasons anyway
    # (keeps day-0 interpretability while ML calibrates).
    if not reasons and suspicious:
        reasons = ["statistical outlier for this facility"]
    return {
        "log_id": log.id,
        "pass_id": log.pass_id,
        "gate_id": log.gate_id or "Main Gate",
        "scan_time": log.scan_time,
        "status": log.status,
        "anomaly_score": round(float(score), 4),
        "is_suspicious": bool(suspicious),
        "reasons": reasons,
        "model": model_name,
    }


def score_recent(db: Session, limit: int = 20, only_suspicious: bool = False) -> dict:
    from src.utils.helpers import safe_limit

    limit = safe_limit(limit, default=20, maximum=200)
    bundle = load_bundle()
    logs = (
        db.query(AccessLogModel)
        .order_by(AccessLogModel.scan_time.desc())
        .limit(limit)
        .all()
    )
    flags = [score_log_row(log, db, bundle) for log in logs]
    model_name = bundle is not None and "isolation_forest" or "rules"
    # If any flag used the forest, report forest; else rules.
    used_forest = any(f.pop("model") == "isolation_forest" for f in flags)
    model_name = "isolation_forest" if used_forest else "rules"
    if only_suspicious:
        flags = [f for f in flags if f["is_suspicious"]]
    # Most suspicious first.
    flags.sort(key=lambda f: f["anomaly_score"])
    return {"model": model_name, "count": len(flags), "flags": flags}
