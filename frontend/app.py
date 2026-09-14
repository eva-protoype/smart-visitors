"""Smart Visitors — Streamlit frontend for the FastAPI backend.

Pages: Dashboard, Users, Passes, Gate Scan, Anomaly, Ask AI.
Backend URL is configurable via BACKEND_URL env var or the sidebar input.
"""

from __future__ import annotations

import os
from datetime import date, datetime, time, timedelta

import pandas as pd
import requests
import streamlit as st

BACKEND_URL = os.getenv("BACKEND_URL", "http://localhost:8000").rstrip("/")
KNOWN_GATES = ["Main Gate", "Back Gate", "East Wing", "Parking"]
ROLES = ["visitor", "host", "guard", "admin"]
EXAMPLE_QUESTIONS = [
    "how many denied entries today?",
    "how many approved scans last 7 days at Main Gate?",
    "show last 10 scans",
    "show last 5 denied scans",
    "entries by gate",
    "how many active passes?",
    "which passes are expiring?",
    "how many visitors?",
]

st.set_page_config(
    page_title="Smart Visitors",
    page_icon="🎫",
    layout="wide",
)


# ---------------------------------------------------------------- helpers
def api_url(path: str) -> str:
    base = st.session_state.get("backend_url", BACKEND_URL).rstrip("/")
    return f"{base}{path}"


def api_get(path: str, params: dict | None = None, timeout: int = 15):
    try:
        r = requests.get(api_url(path), params=params, timeout=timeout)
    except requests.RequestException as e:
        return None, f"Connection failed: {e}"
    if r.status_code >= 400:
        return None, f"HTTP {r.status_code}: {r.text[:500]}"
    try:
        return r.json(), None
    except ValueError:
        return None, f"Non-JSON response: {r.text[:500]}"


def api_post(path: str, payload: dict, timeout: int = 30):
    try:
        r = requests.post(api_url(path), json=payload, timeout=timeout)
    except requests.RequestException as e:
        return None, f"Connection failed: {e}"
    if r.status_code >= 400:
        return None, f"HTTP {r.status_code}: {r.text[:500]}"
    try:
        return r.json(), None
    except ValueError:
        return None, f"Non-JSON response: {r.text[:500]}"


def show_error(err: str):
    st.error(f"Backend error — {err}")


def check_backend() -> tuple[bool, str]:
    """Lightweight health check: /openapi.json always exists on FastAPI."""
    try:
        r = requests.get(api_url("/openapi.json"), timeout=5)
        if r.status_code == 200:
            title = r.json().get("info", {}).get("title", "API")
            return True, f"Connected ({title})"
        return False, f"HTTP {r.status_code}"
    except requests.RequestException as e:
        return False, str(e)


# ---------------------------------------------------------------- sidebar
if "backend_url" not in st.session_state:
    st.session_state["backend_url"] = BACKEND_URL

with st.sidebar:
    st.title("🎫 Smart Visitors")
    st.caption("Visitor passes + gate audit + AI flags")
    st.text_input("Backend URL", key="backend_url", help="e.g. http://localhost:8000 or http://backend:8000 in Docker")
    ok, msg = check_backend()
    if ok:
        st.success(f"Backend: {msg}")
    else:
        st.error(f"Backend unreachable: {msg}")
        st.caption("Start it with `docker compose up --build` (backend on :8000).")
    st.divider()
    page = st.radio(
        "Go to",
        ["Dashboard", "Users", "Passes", "Gate Scan", "Anomaly", "Ask AI"],
        index=0,
    )
    st.divider()
    st.caption("Roles: host creates passes · guard scans · admin audits.")


# ---------------------------------------------------------------- Dashboard
if page == "Dashboard":
    st.header("Dashboard")
    st.write(
        "Digital visitor book: hosts issue time-limited QR passes, guards scan them at the gate, "
        "and managers audit entries, suspicious flags, and plain-English questions."
    )
    c1, c2, c3 = st.columns(3)
    with c1:
        st.subheader("Today")
        data, err = api_post("/ai/query", {"question": "how many denied entries today?", "limit": 5})
        if err:
            st.warning(f"Counts unavailable: {err[:120]}")
        else:
            st.metric("Denied today", data["answer"])
        data2, _ = api_post("/ai/query", {"question": "how many approved scans last 7 days?", "limit": 5})
        if data2:
            st.metric("Approved (7d)", data2["answer"])
    with c2:
        st.subheader("Pass stock")
        for q in ["how many active passes?", "how many used passes?", "how many expired passes?"]:
            d, e = api_post("/ai/query", {"question": q, "limit": 5})
            if d:
                st.metric(q.replace("how many ", "").replace("?", ""), d["answer"])
            elif e:
                st.caption(f"{q}: unavailable")
                break
    with c3:
        st.subheader("Anomaly model")
        flags, err = api_get("/ai/anomaly/flags", {"limit": 5, "only_suspicious": False})
        if err:
            st.warning(f"Anomaly API unavailable: {err[:120]}")
        else:
            st.metric("Backend", flags.get("model", "?"))
            susp = sum(1 for f in flags.get("flags", []) if f.get("is_suspicious"))
            st.metric("Suspicious in last 5", susp)
            if st.button("Retrain model"):
                res, terr = api_post("/ai/anomaly/train", {})
                if terr:
                    show_error(terr)
                else:
                    st.success(f"Trained: {res}")
    st.divider()
    st.subheader("Typical flow")
    st.markdown(
        "1. **Users** → create a `host` user (copy its `id`).\n"
        "2. **Passes** → create a pass for that host (copy the `qr_token`).\n"
        "3. **Gate Scan** → scan the `qr_token` at a gate → **Allowed / Denied**.\n"
        "4. **Anomaly** → train + review suspicious entries.\n"
        "5. **Ask AI** → e.g. *“how many denied entries today?”*"
    )

# ---------------------------------------------------------------- Users
elif page == "Users":
    st.header("Users — create host / guard / visitor / admin")
    col_f, col_h = st.columns([1, 1])
    with col_f:
        with st.form("create_user"):
            username = st.text_input("Username", placeholder="priya_host")
            aadhar = st.text_input("Aadhar number", placeholder="1234-5678-9012")
            email = st.text_input("Email", placeholder="priya@example.com")
            role = st.selectbox("Role", ROLES, index=1)
            submitted = st.form_submit_button("Create user", type="primary")
        if submitted:
            if not username or not aadhar or not email:
                st.warning("Username, Aadhar and Email are required.")
            else:
                with st.spinner("Creating user…"):
                    data, err = api_post(
                        "/users/create",
                        {"username": username, "aadhar_number": aadhar, "email": email, "role": role},
                    )
                if err:
                    show_error(err)
                else:
                    st.success(f"User created: {data.get('username')} ({data.get('role')})")
                    st.code(f"user id (use as host_user_id): {data.get('id')}", language="text")
                    st.json(data)
    with col_h:
        st.subheader("Headcount by role")
        if st.button("Refresh headcount"):
            rows = []
            for r in ROLES:
                d, e = api_post("/ai/query", {"question": f"how many {r}s?", "limit": 5})
                if d and d.get("rows"):
                    rows.append({"role": r, "count": d["rows"][0][0], "answer": d["answer"]})
            if rows:
                st.dataframe(pd.DataFrame(rows), use_container_width=True)
            else:
                st.info("No data yet — create users first.")
        st.caption("User listing has no dedicated endpoint; counts come from the read-only NL-query API.")

# ---------------------------------------------------------------- Passes
elif page == "Passes":
    st.header("Passes — issue time-limited visitor passes")
    col_f, col_h = st.columns([1, 1])
    with col_f:
        with st.form("create_pass"):
            visitor_name = st.text_input("Visitor name", placeholder="Ravi Kumar")
            visitor_email = st.text_input("Visitor email", placeholder="ravi@example.com")
            host_id = st.text_input("Host user id", placeholder="paste id from Users page")
            d = st.date_input("Valid until (date)", value=date.today() + timedelta(days=1))
            t = st.time_input("Valid until (time)", value=time(18, 0))
            submitted = st.form_submit_button("Issue pass", type="primary")
        if submitted:
            if not visitor_name or not visitor_email or not host_id:
                st.warning("Visitor name, email and host id are required.")
            else:
                valid_until = datetime.combine(d, t).isoformat()
                with st.spinner("Issuing pass…"):
                    data, err = api_post(
                        "/passes/create",
                        {
                            "visitor_name": visitor_name,
                            "visitor_email": visitor_email,
                            "host_user_id": host_id,
                            "valid_until": valid_until,
                        },
                    )
                if err:
                    show_error(err)
                else:
                    st.success(f"Pass issued for {data.get('visitor_name')} — status {data.get('status')}")
                    st.code(f"qr_token (scan this at the gate): {data.get('qr_token')}", language="text")
                    st.json(data)
    with col_h:
        st.subheader("Pass stock-take")
        q = st.selectbox(
            "Question",
            ["how many active passes?", "how many used passes?", "how many expired passes?",
             "how many revoked passes?", "how many passes?", "which passes are expiring?"],
        )
        if st.button("Run", key="pass_stock"):
            with st.spinner("Querying…"):
                data, err = api_post("/ai/query", {"question": q, "limit": 20})
            if err:
                show_error(err)
            else:
                st.success(data.get("answer", ""))
                if len(data.get("columns", [])) > 1 and data.get("rows"):
                    st.dataframe(pd.DataFrame(data["rows"], columns=data["columns"]), use_container_width=True)
                else:
                    st.code(f"SQL: {data.get('sql')}", language="sql")

# ---------------------------------------------------------------- Gate Scan
elif page == "Gate Scan":
    st.header("Gate Scan — guard verifies a QR pass")
    with st.form("scan"):
        qr = st.text_input("QR token", placeholder="paste qr_token from Passes page")
        gate = st.selectbox("Gate", KNOWN_GATES, index=0)
        submitted = st.form_submit_button("Scan", type="primary")
    if submitted:
        if not qr.strip():
            st.warning("Paste a qr_token first.")
        else:
            with st.spinner(f"Scanning at {gate}…"):
                data, err = api_post("/passes/scan", {"qr_token": qr.strip(), "gate_id": gate})
            if err:
                show_error(err)
            else:
                access = data.get("access", "?")
                if access == "approved":
                    st.success(f"✅ ALLOWED — {data.get('reason')}")
                else:
                    st.error(f"⛔ DENIED — {data.get('reason')}")
                st.code(f"log_id: {data.get('log_id')}", language="text")
                st.json(data)
    st.divider()
    st.subheader("Recent scans")
    n = st.slider("How many?", 5, 50, 10, key="recent_n")
    if st.button("Load recent scans"):
        with st.spinner("Loading…"):
            data, err = api_post("/ai/query", {"question": f"show last {n} scans", "limit": n})
        if err:
            show_error(err)
        elif data.get("rows"):
            st.dataframe(pd.DataFrame(data["rows"], columns=data["columns"]), use_container_width=True)
        else:
            st.info(data.get("answer", "No scans yet."))

# ---------------------------------------------------------------- Anomaly
elif page == "Anomaly":
    st.header("Anomaly — suspicious-entry alarm (advisor only, never opens gates)")
    c1, c2 = st.columns([1, 2])
    with c1:
        st.subheader("Train")
        st.caption("Needs ≥10 logs for IsolationForest, else rule-based fallback. Retrain every few weeks.")
        if st.button("Train / retrain model", type="primary"):
            with st.spinner("Training…"):
                data, err = api_post("/ai/anomaly/train", {})
            if err:
                show_error(err)
            else:
                st.success(f"model={data.get('model')} · n={data.get('n_samples')}")
                st.json(data)
        st.divider()
        st.subheader("Score one log")
        log_id = st.text_input("log_id", placeholder="paste log_id from a scan")
        if st.button("Score"):
            if not log_id.strip():
                st.warning("Enter a log_id.")
            else:
                with st.spinner("Scoring…"):
                    data, err = api_get(f"/ai/anomaly/score/{log_id.strip()}")
                if err:
                    show_error(err)
                else:
                    st.metric("Suspicious?", str(data.get("is_suspicious")))
                    st.metric("Score (lower = stranger)", f"{data.get('anomaly_score', 0):.3f}")
                    st.write("**Reasons:**", ", ".join(data.get("reasons", []) or ["—"]))
                    st.json(data)
    with c2:
        st.subheader("Flags (most suspicious first)")
        limit = st.slider("Limit", 1, 200, 20)
        only = st.checkbox("Only suspicious", value=True)
        if st.button("Load flags", type="primary"):
            with st.spinner("Scoring…"):
                data, err = api_get("/ai/anomaly/flags", {"limit": limit, "only_suspicious": str(only).lower()})
            if err:
                show_error(err)
            else:
                st.caption(f"model={data.get('model')} · count={data.get('count')}")
                flags = data.get("flags", [])
                if not flags:
                    st.info("No flags — scan some passes first.")
                else:
                    df = pd.DataFrame(flags)
                    # Compact table + expandable reasons
                    show_cols = [c for c in ["scan_time", "gate_id", "status", "anomaly_score", "is_suspicious"] if c in df.columns]
                    st.dataframe(df[show_cols] if show_cols else df, use_container_width=True)
                    for f in flags:
                        with st.expander(
                            f"{'🚨' if f.get('is_suspicious') else '✓'} {f.get('gate_id')} · "
                            f"{str(f.get('scan_time'))[:16]} · {f.get('status')} · "
                            f"score {f.get('anomaly_score', 0):.3f}"
                        ):
                            st.write("**Reasons:**", ", ".join(f.get("reasons", []) or ["statistical outlier"]))
                            st.code(f"log_id={f.get('log_id')} pass_id={f.get('pass_id')}", language="text")

# ---------------------------------------------------------------- Ask AI
else:
    st.header("Ask AI — plain-English questions over the audit log")
    st.caption("Read-only. Max 50 rows. Aadhar numbers are never shown. SQL is echoed for auditability.")
    preset = st.selectbox("Try an example", EXAMPLE_QUESTIONS, index=0)
    question = st.text_area("Your question", value=preset, height=70)
    limit = st.slider("Max rows", 1, 50, 20)
    if st.button("Ask", type="primary"):
        if not question.strip():
            st.warning("Type a question first.")
        else:
            with st.spinner("Querying…"):
                data, err = api_post("/ai/query", {"question": question.strip(), "limit": limit})
            if err:
                show_error(err)
                st.caption("Try e.g.: “how many denied entries today?” · “entries by gate” · “show last 10 scans”")
            else:
                st.success(data.get("answer", ""))
                for w in data.get("warnings", []):
                    st.warning(w)
                st.code(data.get("sql", ""), language="sql")
                if data.get("params"):
                    st.json({"params": data["params"], "columns": data["columns"]})
                if data.get("rows"):
                    st.dataframe(
                        pd.DataFrame(data["rows"], columns=data["columns"]),
                        use_container_width=True,
                    )
                else:
                    st.info("No rows returned.")
