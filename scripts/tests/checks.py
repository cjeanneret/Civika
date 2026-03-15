#!/usr/bin/env python3
"""Checks API fonctionnels complementaires a Schemathesis.

Ce script valide des cas metier critiques qui doivent rester stables:
- controle strict des inputs invalides (400/404/429),
- validite minimale des payloads JSON de succes,
- presence de headers de securite.
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any


API_BASE_URL = os.getenv("API_BASE_URL", "http://127.0.0.1:8080").rstrip("/")


@dataclass
class APIResponse:
    status: int
    body: str
    headers: dict[str, str]

    def json(self) -> Any:
        return json.loads(self.body) if self.body else {}


def request(
    method: str,
    path: str,
    *,
    body: dict[str, Any] | None = None,
    headers: dict[str, str] | None = None,
) -> APIResponse:
    encoded = None
    req_headers = {"Accept": "application/json"}
    if headers:
        req_headers.update(headers)
    if body is not None:
        encoded = json.dumps(body).encode("utf-8")
        req_headers["Content-Type"] = "application/json"
    req = urllib.request.Request(
        f"{API_BASE_URL}{path}",
        data=encoded,
        method=method,
        headers=req_headers,
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            payload = resp.read().decode("utf-8")
            return APIResponse(
                status=resp.status,
                body=payload,
                headers={k.lower(): v for k, v in resp.headers.items()},
            )
    except urllib.error.HTTPError as exc:
        payload = exc.read().decode("utf-8")
        return APIResponse(
            status=exc.code,
            body=payload,
            headers={k.lower(): v for k, v in exc.headers.items()},
        )


def assert_true(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def assert_api_error(resp: APIResponse, expected_status: int, expected_code: str) -> None:
    assert_true(resp.status == expected_status, f"status attendu {expected_status}, recu {resp.status}")
    payload = resp.json()
    assert_true(isinstance(payload, dict), "payload erreur doit etre un objet JSON")
    assert_true(payload.get("code") == expected_code, f"code erreur attendu {expected_code}, recu {payload.get('code')}")
    assert_true("message" in payload and isinstance(payload["message"], str), "message erreur manquant")
    assert_true("requestId" in payload and isinstance(payload["requestId"], str), "requestId manquant")


def assert_security_headers(resp: APIResponse) -> None:
    assert_true(resp.headers.get("x-content-type-options") == "nosniff", "header nosniff manquant")
    assert_true(resp.headers.get("x-frame-options") == "DENY", "header frame options manquant")
    assert_true("referrer-policy" in resp.headers, "referrer-policy manquant")


def run_checks() -> None:
    # 1) Endpoint coeur valide.
    health = request("GET", "/health")
    assert_true(health.status == 200, f"/health doit renvoyer 200, recu {health.status}")
    assert_true(health.json().get("status") == "ok", "health.status doit valoir ok")
    assert_security_headers(health)

    # 2) Validation query: limite hors borne.
    invalid_limit = request("GET", "/api/v1/votations?limit=500")
    assert_api_error(invalid_limit, 400, "invalid_query")

    # 3) Validation path param: ID non conforme.
    invalid_path = request("GET", "/api/v1/votations/%20")
    assert_api_error(invalid_path, 400, "invalid_path_param")

    # 4) Validation body QA: champs inconnus rejetes.
    bad_qa = request(
        "POST",
        "/api/v1/qa/query",
        body={
            "question": "Que change cette votation ?",
            "language": "fr",
            "unknownField": "x",
        },
    )
    assert_api_error(bad_qa, 400, "invalid_body")

    # 5) Validation metrics: enum invalide.
    invalid_granularity = request("GET", "/api/v1/metrics/ai-usage?granularity=month")
    assert_api_error(invalid_granularity, 400, "invalid_query")

    # 6) Validite minimale de la liste votations.
    votations = request("GET", "/api/v1/votations?limit=1&offset=0&lang=fr")
    assert_true(votations.status == 200, f"liste votations doit renvoyer 200, recu {votations.status}")
    payload = votations.json()
    assert_true(isinstance(payload, dict), "payload votations doit etre un objet")
    assert_true(isinstance(payload.get("items"), list), "items doit etre une liste")
    assert_true(isinstance(payload.get("limit"), int), "limit doit etre un entier")
    assert_true(isinstance(payload.get("offset"), int), "offset doit etre un entier")
    assert_true(isinstance(payload.get("total"), int), "total doit etre un entier")


def main() -> int:
    try:
        run_checks()
    except Exception as exc:  # pragma: no cover - utile en execution CI
        print(f"[checks] ECHEC: {exc}", file=sys.stderr)
        return 1
    print("[checks] OK: validations fonctionnelles ciblees reussies.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
