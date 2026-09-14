"""Text-to-SQL assistant (Step 2.2).

Rule-based NL -> parametrised SELECT (no LLM key required).
Phase 3 hook: if settings.LLM_PROVIDER != "none", an LLM may propose SQL,
but it MUST pass through validate_sql() before execution.

Safety contract (see task.md §3.4):
- single SELECT only, allowlisted tables/columns, aadhar_number never selected,
  always LIMIT-clamped, parametrised (no f-string interpolation of user text).
"""

import re

from fastapi import HTTPException
from sqlalchemy import text
from sqlalchemy.orm import Session

from src.utils.constant import (
    ALLOWED_COLUMNS,
    ALLOWED_TABLES,
    BLOCKED_SQL_KEYWORDS,
    EXAMPLE_QUESTIONS,
    KNOWN_GATES,
)
from src.utils.settings import settings

_ROLE_MAP = {
    "visitor": "visitor",
    "visitors": "visitor",
    "host": "host",
    "hosts": "host",
    "guard": "guard",
    "guards": "guard",
    "admin": "admin",
    "admins": "admin",
}

_PASS_STATUSES = ("active", "used", "expired", "revoked")

# Word-boundary match for expiry questions. \bexpires?\b matches "expire" /
# "expires" but NOT "expired" (handled by its own branch below).
_EXPIRY_RE = re.compile(r"\bexpiring\b|\bexpires?\b|valid until")


def _find_gate(question: str) -> str | None:
    q = question.lower()
    for gate in KNOWN_GATES:
        if gate.lower() in q:
            return gate
    m = re.search(r"(?:at|for|in|gate)\s+([a-z][a-z\s]+)", q)
    if m:
        candidate = m.group(1).strip().title()
        # Only accept if it resembles a known gate or is short (avoid swallowing dates).
        if candidate in KNOWN_GATES or len(candidate) <= 20:
            # Guard against capturing date words like "today".
            if candidate.lower() not in ("today", "yesterday", "days"):
                # Prefer known gates; otherwise return None to avoid SQL on unknown gate.
                return candidate if candidate in KNOWN_GATES else None
    return None


def _date_filter(question: str) -> tuple[str, dict, str]:
    """Return (sql_fragment, params, label) for supported date phrases."""
    q = question.lower()
    m = re.search(r"last\s+(\d+)\s+days?", q)
    if m:
        days = max(1, min(int(m.group(1)), 365))
        return (
            " AND date(scan_time) >= date('now', :days_ago)",
            {"days_ago": f"-{days} days"},
            f"last {days} days",
        )
    if "yesterday" in q:
        return (
            " AND date(scan_time) = date('now', '-1 day')",
            {},
            "yesterday",
        )
    if "today" in q:
        return (
            " AND date(scan_time) = date('now')",
            {},
            "today",
        )
    return "", {}, ""


def validate_sql(sql: str) -> None:
    s = sql.strip().lower()
    if not s.startswith("select"):
        raise HTTPException(status_code=400, detail="Only SELECT queries are allowed")
    for kw in BLOCKED_SQL_KEYWORDS:
        # Block DDL/DML keywords and statement chaining.
        if re.search(rf"(?<![a-z]){re.escape(kw)}(?![a-z])", s) if kw.isalpha() else kw in s:
            raise HTTPException(status_code=400, detail=f"Blocked keyword in query: {kw}")
    # Tables referenced must be allowlisted (cheap FROM/JOIN scan).
    tables = re.findall(r"(?:from|join)\s+([a-z_]+)", s)
    for t in tables:
        if t not in ALLOWED_TABLES:
            raise HTTPException(status_code=400, detail=f"Table not allowed: {t}")
    # Projected columns must be allowlisted too: SELECT * would silently
    # return users.aadhar_number. COUNT(*) stays allowed (an aggregate).
    select_part = s.split("from", 1)[0]
    select_sparse = re.sub(r"\bcount\s*\(\s*\*\s*\)", "", select_part)
    if "*" in select_sparse:
        raise HTTPException(status_code=400, detail="Wildcard SELECT is not allowed")
    allowed_cols = {c for cols in ALLOWED_COLUMNS.values() for c in cols}
    projected = re.sub(r"\bas\s+[a-z_][a-z0-9_]*", "", select_sparse)
    skip = {"select", "distinct", "all", "count", "as", "asc", "desc"}
    for ident in re.findall(r"[a-z_][a-z0-9_]*", projected):
        if ident in skip:
            continue
        if ident not in allowed_cols:
            raise HTTPException(status_code=400, detail=f"Column not allowed: {ident}")
    if "aadhar" in s:
        raise HTTPException(status_code=400, detail="aadhar_number is never queryable")


def _clamp_limit(limit: int | None) -> int:
    try:
        n = int(limit) if limit is not None else 20
    except (TypeError, ValueError):
        n = 20
    return max(1, min(n, int(settings.AI_MAX_QUERY_ROWS)))


def build_query(question: str, limit: int | None = 20) -> tuple[str, dict, list[str], str]:
    """Parse NL question -> (sql, params, columns, answer_prefix).

    Raises HTTPException 400 if the question is not understood.
    """
    if not question or not question.strip():
        raise HTTPException(status_code=400, detail="Question must not be empty")
    q = question.strip().lower()
    lim = _clamp_limit(limit)
    gate = _find_gate(question)
    date_sql, date_params, date_label = _date_filter(question)
    gate_sql, gate_params = ("", {}) if not gate else (" AND gate_id = :gate_0", {"gate_0": gate})
    where_suffix = f"{date_sql}{gate_sql}"
    params = {**date_params, **gate_params}

    # 1. Users by role: "how many visitors/hosts/guards/admins"
    for plural, role in _ROLE_MAP.items():
        if re.search(rf"how many\s+{plural}\b", q):
            return (
                "SELECT COUNT(*) AS count FROM users WHERE role = :role_0 LIMIT 1",
                {"role_0": role},
                ["count"],
                f"{plural} count",
            )

    # 2. Pass status counts: "how many active/expired/... passes"
    for status in _PASS_STATUSES:
        if re.search(rf"how many\s+{status}\s+passes?", q):
            return (
                "SELECT COUNT(*) AS count FROM passes WHERE status = :status_0 LIMIT 1",
                {"status_0": status},
                ["count"],
                f"{status} passes",
            )
    if re.search(r"how many\s+passes?", q) and not _EXPIRY_RE.search(q):
        return (
            "SELECT COUNT(*) AS count FROM passes LIMIT 1",
            {},
            ["count"],
            "total passes",
        )

    # 3. Entries by gate / top gates breakdown.
    if re.search(r"(entries by gate|by gate|top gates|breakdown by gate|per gate)", q):
        return (
            "SELECT gate_id, COUNT(*) AS entries FROM access_logs"
            f"{date_sql} GROUP BY gate_id ORDER BY entries DESC LIMIT :limit_0",
            {**date_params, "limit_0": lim},
            ["gate_id", "entries"],
            "entries by gate",
        )

    # 4. Counts by scan status: "how many denied/approved/total entries ... [today] [at GATE]"
    m = re.search(r"how many\s+(denied|approved|total)?\s*(entries|entry|scans?|logs?|attempts?)?", q)
    if m and ("denied" in q or "approved" in q or "entr" in q or "scan" in q or "log" in q or "attempt" in q):
        status = m.group(1)
        if status in ("denied", "approved"):
            scope = f"{status} entries{(' ' + date_label) if date_label else ''}{(' at ' + gate) if gate else ''}"
            return (
                f"SELECT COUNT(*) AS count FROM access_logs WHERE status = :status_0{where_suffix} LIMIT 1",
                {"status_0": status, **params},
                ["count"],
                scope,
            )
        scope = f"entries{(' ' + date_label) if date_label else ''}{(' at ' + gate) if gate else ''}"
        where = where_suffix[5:] if where_suffix.startswith(" AND ") else where_suffix
        sql = "SELECT COUNT(*) AS count FROM access_logs"
        if where:
            sql += f" WHERE {where}"
        return (sql + " LIMIT 1", params, ["count"], scope)

    # 5. Recent logs: "show last N denied/approved scans"
    if any(w in q for w in ("show", "list", "recent", "last")) and any(
        w in q for w in ("scan", "log", "entr", "denied", "approved", "attempt", "recent", "last")
    ):
        n_match = re.search(r"(last|recent|show|list)\s+(\d+)", q)
        n = _clamp_limit(int(n_match.group(2)) if n_match else lim)
        status_word = "denied" if "denied" in q else ("approved" if "approved" in q else None)
        cols = ["id", "pass_id", "gate_id", "scan_time", "status"]
        if status_word in ("denied", "approved"):
            scope = f"last {n} {status_word} scans"
            return (
                f"SELECT {', '.join(cols)} FROM access_logs WHERE status = :status_0{where_suffix}"
                " ORDER BY scan_time DESC LIMIT :limit_0",
                {"status_0": status_word, **params, "limit_0": n},
                cols,
                scope,
            )
        scope = f"last {n} scans"
        where = where_suffix[5:] if where_suffix.startswith(" AND ") else where_suffix
        sql = f"SELECT {', '.join(cols)} FROM access_logs"
        if where:
            sql += f" WHERE {where}"
        return (
            sql + " ORDER BY scan_time DESC LIMIT :limit_0",
            {**params, "limit_0": n},
            cols,
            scope,
        )

    # 6. Expired vs expiring passes. Word boundaries matter: "expired" must
    # not match the expiring logic (and vice versa).
    if re.search(r"\bexpired\b", q):
        return (
            "SELECT id, visitor_name, visitor_email, status, valid_until FROM passes"
            " WHERE status = 'expired' ORDER BY valid_until DESC LIMIT :limit_0",
            {"limit_0": lim},
            ["id", "visitor_name", "visitor_email", "status", "valid_until"],
            "expired passes",
        )
    if _EXPIRY_RE.search(q):
        return (
            "SELECT id, visitor_name, visitor_email, status, valid_until FROM passes"
            " WHERE status = 'active' ORDER BY valid_until ASC LIMIT :limit_0",
            {"limit_0": lim},
            ["id", "visitor_name", "visitor_email", "status", "valid_until"],
            "active passes by expiry",
        )

    raise HTTPException(
        status_code=400,
        detail=f"Could not understand question. Try e.g.: {EXAMPLE_QUESTIONS[0]}",
    )


def run_query(db: Session, question: str, limit: int | None = 20) -> dict:
    sql, params, columns, scope = build_query(question, limit)
    validate_sql(sql)
    result = db.execute(text(sql), params)
    raw_rows = result.fetchmany(_clamp_limit(limit))
    rows = [list(r) for r in raw_rows]

    # Human one-liner.
    if len(columns) == 1 and columns[0] == "count" and rows:
        answer = f"{rows[0][0]} {scope}."
    elif not rows:
        answer = f"No results for: {scope}."
    else:
        answer = f"{len(rows)} row(s) for: {scope}."

    warnings: list[str] = []
    if _find_gate(question) is None and re.search(r"\bat\b", question.lower()):
        warnings.append("Gate name not recognised; searched across all gates.")
    return {
        "sql": sql,
        "params": params,
        "columns": columns,
        "rows": rows,
        "answer": answer,
        "warnings": warnings,
    }
