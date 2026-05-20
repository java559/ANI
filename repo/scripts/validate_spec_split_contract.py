#!/usr/bin/env python3
"""Validate Sprint 4 SPEC-SPLIT-A Core/Services API split."""

from __future__ import annotations

from pathlib import Path
from typing import Any
import json

import yaml


ROOT = Path(__file__).resolve().parents[1]
DOC_ROOT = ROOT.parent

SERVICE_PATH_PREFIXES = ("/models", "/inference-services", "/knowledge-bases")
EXPECTED_SERVICE_PATHS = {
    "/models",
    "/models/import",
    "/models/{model_id}",
    "/models/{model_id}/versions",
    "/inference-services",
    "/inference-services/{service_id}",
    "/knowledge-bases",
    "/knowledge-bases/{kb_id}",
    "/knowledge-bases/{kb_id}/documents",
    "/knowledge-bases/{kb_id}/documents/{doc_id}",
    "/knowledge-bases/{kb_id}/query",
    "/knowledge-bases/{kb_id}/query/stream",
}
EXPECTED_SERVICE_SCHEMAS = {
    "Model",
    "ModelVersion",
    "InferenceService",
    "KnowledgeBase",
    "KBDocument",
    "KBQueryResponse",
}


def fail(message: str) -> None:
    raise SystemExit(f"spec split contract invalid: {message}")


def read_repo(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def read_doc(path: str) -> str:
    return (DOC_ROOT / path).read_text(encoding="utf-8")


def load_yaml(path: str) -> dict[str, Any]:
    return yaml.safe_load(read_repo(path))


def validate_openapi() -> None:
    core = load_yaml("api/openapi/v1.yaml")
    services = load_yaml("api/openapi/services/v1.yaml")
    core_paths = set(core.get("paths", {}))
    service_paths = set(services.get("paths", {}))

    leaked = sorted(path for path in core_paths if path.startswith(SERVICE_PATH_PREFIXES))
    if leaked:
        fail(f"Core API must not contain Services business paths: {leaked}")
    missing = sorted(EXPECTED_SERVICE_PATHS - service_paths)
    if missing:
        fail(f"Services API missing migrated paths: {missing}")
    if services.get("servers", [{}])[0].get("url") != "https://{host}/api/v1/svc":
        fail("Services API server URL must be https://{host}/api/v1/svc")

    service_schemas = set(services.get("components", {}).get("schemas", {}))
    missing_schemas = sorted(EXPECTED_SERVICE_SCHEMAS - service_schemas)
    if missing_schemas:
        fail(f"Services API missing schemas: {missing_schemas}")

    core_tags = {item.get("name") for item in core.get("tags", []) if isinstance(item, dict)}
    forbidden_tags = {"Models", "InferenceServices", "KnowledgeBases"}
    if core_tags & forbidden_tags:
        fail(f"Core API tags must not include Services tags: {sorted(core_tags & forbidden_tags)}")


def validate_gateway() -> None:
    router = read_repo("services/ani-gateway/internal/router/router.go")
    if 'h.Group("/api/v1/svc")' not in router:
        fail("Gateway must register Services routes under /api/v1/svc")
    for forbidden in ("registerModels(v1)", "registerInferenceServices(v1)", "registerKnowledgeBases(v1)"):
        if forbidden in router:
            fail(f"Gateway must not register Services routes under Core group: {forbidden}")
    for required in ("registerModels(svc)", "registerInferenceServices(svc)", "registerKnowledgeBases(svc)"):
        if required not in router:
            fail(f"Gateway missing Services route registration {required}")

    rbac = read_repo("services/ani-gateway/internal/middleware/rbac.go")
    if 'resource == "svc"' not in rbac:
        fail("RBAC inference must skip /api/v1/svc prefix and use the owned Services resource")


def validate_sdk_metadata() -> None:
    core_metadata = json.loads((ROOT / "sdks/core/sdk-metadata.json").read_text(encoding="utf-8"))
    services_metadata = json.loads((ROOT / "sdks/services/sdk-metadata.json").read_text(encoding="utf-8"))
    core_paths = {item["path"] for item in core_metadata.get("operations", [])}
    services_paths = {item["path"] for item in services_metadata.get("operations", [])}

    leaked = sorted(path for path in core_paths if path.startswith(SERVICE_PATH_PREFIXES))
    if leaked:
        fail(f"Core SDK must not contain Services business paths: {leaked}")
    missing = sorted(EXPECTED_SERVICE_PATHS - services_paths)
    if missing:
        fail(f"Services SDK metadata missing migrated paths: {missing}")


def validate_docs() -> None:
    plan = read_doc("ANI-06-开发计划.md")
    sprint = read_repo("CURRENT-SPRINT.md")
    records = read_repo("development-records/README.md")
    for path, text in {
        "ANI-06-开发计划.md": plan,
        "CURRENT-SPRINT.md": sprint,
        "development-records/README.md": records,
    }.items():
        if "SPEC-SPLIT-A" not in text:
            fail(f"{path} must reference SPEC-SPLIT-A")


def main() -> None:
    validate_openapi()
    validate_gateway()
    validate_sdk_metadata()
    validate_docs()
    print("spec split contract valid")


if __name__ == "__main__":
    main()
