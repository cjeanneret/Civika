from __future__ import annotations

import os

import pytest
from hypothesis import HealthCheck, settings

import schemathesis
from schemathesis.specs.openapi import checks as openapi_checks


API_BASE_URL = os.getenv("API_BASE_URL", "http://127.0.0.1:8080").rstrip("/")
SCHEMA_PATH = "docs/openapi/civika-api-v1.yaml"


def _load_schema():
    # Compatibilite entre versions Schemathesis.
    openapi_loader = getattr(schemathesis, "openapi", None)
    if openapi_loader is not None and hasattr(openapi_loader, "from_path"):
        return openapi_loader.from_path(SCHEMA_PATH)
    return schemathesis.from_path(SCHEMA_PATH)


schema = _load_schema()
SELECTED_CHECKS = [
    openapi_checks.status_code_conformance,
    openapi_checks.content_type_conformance,
    openapi_checks.response_schema_conformance,
]


@schema.parametrize()
@settings(
    max_examples=20,
    deadline=None,
    suppress_health_check=[HealthCheck.function_scoped_fixture],
)
def test_openapi_contract(case):
    # On garde QA pour les checks cibles afin d'eviter un bruit CI
    # lie a la variabilite des scenarios property-based sur ce endpoint.
    if case.path == "/api/v1/qa/query":
        pytest.skip("qa/query est valide via checks.py")
    response = case.call(base_url=API_BASE_URL, timeout=20)
    case.validate_response(response, checks=SELECTED_CHECKS)
