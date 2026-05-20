#!/usr/bin/env python3
"""Validate Sprint 3 closure alignment across docs, contracts, and SDK metadata."""

from __future__ import annotations

from pathlib import Path
from typing import Any
import json

import yaml


ROOT = Path(__file__).resolve().parents[1]
DOC_ROOT = ROOT.parent

SPRINT3_BATCHES = {
    "M1-NETWORK-A": [
        "m1-network-a-core-api-dev-profile.md",
        "m1-network-a-kubeovn-renderer.md",
        "m1-network-a-provider-dry-run-apply-gate.md",
        "m1-network-a-provider-status-reader.md",
        "m1-network-a-status-reconcile.md",
    ],
    "M1-STORAGE-A": [
        "m1-storage-a-core-api-dev-profile.md",
        "m1-storage-a-persistence-boundary.md",
        "m1-storage-a-provider-renderer.md",
        "m1-storage-a-provider-dry-run-apply-gate.md",
        "m1-storage-a-status-reconcile.md",
    ],
    "M1-VSTORE-A": ["m1-vstore-a-core-api-dev-profile.md"],
    "M1-WKID-A": ["m1-wkid-a-workload-identity-p0.md"],
    "SDK-ALPHA-A": ["sdk-alpha-a-generation-smoke.md"],
    "CORE-DEV-PROFILE-A": ["core-dev-profile-a-boundary-contract.md"],
}

SPRINT3_TARGETS = (
    "validate-network-alpha",
    "validate-storage-alpha",
    "validate-vector-alpha",
    "validate-workload-identity",
    "validate-sdk-alpha",
    "validate-core-dev-profile",
)

CORE_PATH_PREFIXES = (
    "/instances",
    "/instance-operations",
    "/networks",
    "/volumes",
    "/filesystems",
    "/objects",
    "/vector-stores",
)

CORE_SCHEMAS = (
    "CoreDevProfileInfo",
    "InstanceRecord",
    "NetworkVPC",
    "NetworkSubnet",
    "NetworkSecurityGroup",
    "NetworkLoadBalancer",
    "StorageVolume",
    "StorageFilesystem",
    "StorageObject",
    "VectorStore",
    "WorkloadIdentityBinding",
)


def fail(message: str) -> None:
    raise SystemExit(f"sprint3 closure invalid: {message}")


def read_repo(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def read_doc(path: str) -> str:
    return (DOC_ROOT / path).read_text(encoding="utf-8")


def load_yaml(path: str) -> dict[str, Any]:
    return yaml.safe_load(read_repo(path))


def validate_docs() -> None:
    claude = read_doc("CLAUDE.md")
    docs_index = read_doc("ANI-DOCS-INDEX.md")
    plan = read_doc("ANI-06-开发计划.md")

    for path, text in {
        "CLAUDE.md": claude,
        "ANI-DOCS-INDEX.md": docs_index,
    }.items():
        if "Sprint 4" not in text:
            fail(f"{path} must point to Sprint 4 after Sprint 3 closure")
    if "| Sprint 3 | ✅ 已完成" not in plan:
        fail("ANI-06-开发计划.md must mark Sprint 3 completed")
    if "Sprint 4 ⭐（当前）" not in plan:
        fail("ANI-06-开发计划.md must mark Sprint 4 current")

    stale_phrases = (
        "当前下一阶段是 CORE-DEV-PROFILE-A",
        "CORE-DEV-PROFILE-A 与 Sprint 3 闭环审查",
    )
    for phrase in stale_phrases:
        if phrase in plan:
            fail(f"ANI-06-开发计划.md still contains stale next-step phrase {phrase!r}")

    for batch in SPRINT3_BATCHES:
        if batch not in plan:
            fail(f"ANI-06-开发计划.md missing {batch}")


def validate_development_records() -> None:
    index = read_repo("development-records/README.md")
    records_dir = ROOT / "development-records"
    for batch, files in SPRINT3_BATCHES.items():
        if batch not in index:
            fail(f"development-records/README.md missing {batch}")
        for filename in files:
            path = records_dir / filename
            if not path.exists():
                fail(f"missing development record {filename}")
            if filename not in index:
                fail(f"development-records/README.md does not link {filename}")


def validate_makefile() -> None:
    makefile = read_repo("Makefile")
    if "validate-sprint3-closure:" not in makefile:
        fail("Makefile missing validate-sprint3-closure target")
    for target in SPRINT3_TARGETS:
        if f"{target}:" not in makefile:
            fail(f"Makefile missing {target} target")
        if f"$(MAKE) {target}" not in makefile:
            fail(f"validate-sprint3-closure must call {target}")


def validate_openapi_and_sdk() -> None:
    core = load_yaml("api/openapi/v1.yaml")
    services = load_yaml("api/openapi/services/v1.yaml")
    core_paths = core.get("paths", {})
    service_paths = services.get("paths", {})
    core_schemas = core.get("components", {}).get("schemas", {})

    required_core_paths = (
        "/instances",
        "/networks/vpcs",
        "/networks/subnets",
        "/networks/security-groups",
        "/networks/load-balancers",
        "/volumes",
        "/filesystems",
        "/objects",
        "/vector-stores",
    )
    for path in required_core_paths:
        if path not in core_paths:
            fail(f"Core API missing Sprint 3 path {path}")
    leaked = [path for path in service_paths if path.startswith(CORE_PATH_PREFIXES)]
    if leaked:
        fail(f"Services API contains Core paths: {leaked}")
    for schema in CORE_SCHEMAS:
        if schema not in core_schemas:
            fail(f"Core API missing schema {schema}")

    core_metadata = json.loads((ROOT / "sdks/core/sdk-metadata.json").read_text(encoding="utf-8"))
    services_metadata = json.loads((ROOT / "sdks/services/sdk-metadata.json").read_text(encoding="utf-8"))
    core_sdk_schemas = set(core_metadata.get("schemas", []))
    if not set(CORE_SCHEMAS).issubset(core_sdk_schemas):
        missing = sorted(set(CORE_SCHEMAS) - core_sdk_schemas)
        fail(f"Core SDK metadata missing schemas: {missing}")
    services_sdk_paths = {item["path"] for item in services_metadata.get("operations", [])}
    if any(path.startswith(CORE_PATH_PREFIXES) for path in services_sdk_paths):
        fail("Services SDK metadata contains Core infrastructure paths")


def main() -> None:
    validate_docs()
    validate_development_records()
    validate_makefile()
    validate_openapi_and_sdk()
    print("sprint3 closure contract valid")


if __name__ == "__main__":
    main()
