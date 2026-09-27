"""IsolationForest helper for Go backend. Python kept only here.

Usage:
  echo '{"features": [[...]], "contamination": 0.1}' | python3 ml/isolation.py train --artifact artifacts/isolation_forest.pkl --contamination 0.1
  echo '{"features": [[...]]}' | python3 ml/isolation.py score --artifact artifacts/isolation_forest.pkl

train: fits IsolationForest, pickles bundle, prints {"threshold": t, "n_samples": n}.
score: loads pickle, prints {"scores": [...]}.
No FastAPI, no DB access. Go owns features, validation, HTTP.
"""
import argparse
import json
import pickle
import sys


def cmd_train(args):
    payload = json.load(sys.stdin)
    features = payload.get("features", [])
    contamination = float(payload.get("contamination", args.contamination))
    import numpy as np

    X = np.array(features, dtype=float)
    n = len(X)
    if n == 0:
        print(json.dumps({"threshold": 0.0, "n_samples": 0}))
        return
    try:
        from sklearn.ensemble import IsolationForest
    except Exception as e:
        print(f"sklearn missing: {e}", file=sys.stderr)
        sys.exit(2)
    model = IsolationForest(contamination=contamination, random_state=42)
    model.fit(X)
    scores = model.score_samples(X)
    threshold = float(np.percentile(scores, 100.0 * contamination))
    import datetime, os

    os.makedirs(os.path.dirname(os.path.abspath(args.artifact)) or ".", exist_ok=True)
    bundle = {
        "model": model,
        "threshold": threshold,
        "n_samples": n,
        "contamination": contamination,
        "trained_at": datetime.datetime.utcnow().isoformat() + "Z",
    }
    with open(args.artifact, "wb") as fh:
        pickle.dump(bundle, fh)
    print(json.dumps({"threshold": threshold, "n_samples": n}))


def cmd_score(args):
    payload = json.load(sys.stdin)
    features = payload.get("features", [])
    import numpy as np

    with open(args.artifact, "rb") as fh:
        bundle = pickle.load(fh)
    model = bundle["model"]
    X = np.array(features, dtype=float)
    if len(X) == 0:
        print(json.dumps({"scores": []}))
        return
    scores = model.score_samples(X)
    print(json.dumps({"scores": [float(s) for s in scores]}))


def main():
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    t = sub.add_parser("train")
    t.add_argument("--artifact", required=True)
    t.add_argument("--contamination", default="0.1")
    s = sub.add_parser("score")
    s.add_argument("--artifact", required=True)
    args = ap.parse_args()
    if args.cmd == "train":
        cmd_train(args)
    else:
        cmd_score(args)


if __name__ == "__main__":
    main()
