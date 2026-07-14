#!/usr/bin/env python3
"""Validate the repository's fail-closed CI workflow contract."""

from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml

REQUIRED_JOBS = {
    "go-ci",
    "python-ci",
    "frontend-ci",
    "services-pr-gate",
    "dependency-scan",
    "api-spec-lint",
}
WORKFLOW_PATH = Path(__file__).resolve().parents[2] / ".github/workflows/ci.yml"
MAKEFILE_PATH = Path(__file__).resolve().parents[1] / "Makefile"


def load_workflow(path: Path = WORKFLOW_PATH) -> dict[str, Any]:
    with path.open(encoding="utf-8") as handle:
        workflow = yaml.safe_load(handle) or {}
    if not isinstance(workflow, dict):
        raise ValueError(f"{path} must contain a YAML object")
    return workflow


def validate(workflow: dict[str, Any], makefile_text: str) -> list[str]:
    errors: list[str] = []
    jobs = workflow.get("jobs")
    if not isinstance(jobs, dict):
        return ["workflow.jobs must be a mapping"]

    missing = REQUIRED_JOBS - set(jobs)
    if missing:
        errors.append(f"missing required jobs: {', '.join(sorted(missing))}")

    aggregate = jobs.get("required-gates")
    if not isinstance(aggregate, dict):
        errors.append("required-gates job is missing")
    else:
        if "always()" not in str(aggregate.get("if", "")):
            errors.append("required-gates must use always()")
        needs = set(aggregate.get("needs") or [])
        missing_needs = REQUIRED_JOBS - needs
        if missing_needs:
            errors.append(
                "required-gates must need every required job: "
                + ", ".join(sorted(missing_needs))
            )
        run_text = "\n".join(
            str(step.get("run", ""))
            for step in aggregate.get("steps", [])
            if isinstance(step, dict)
        )
        if "success" not in run_text or "exit 1" not in run_text:
            errors.append("required-gates must fail closed on non-success results")

    services = jobs.get("services-pr-gate")
    services_text = str(services)
    if "ai/rag-engine/requirements.txt" in services_text:
        errors.append("Services gate must not install RAG runtime requirements")

    workflow_text = str(workflow)
    if "swaggerhub-actions/validate-openapi" in workflow_text:
        errors.append("workflow must not use the unresolved SwaggerHub action")

    if "/private/tmp" in makefile_text:
        errors.append("Makefile must not use the non-portable /private/tmp path")

    return errors


def main() -> int:
    errors = validate(load_workflow(), MAKEFILE_PATH.read_text(encoding="utf-8"))
    for error in errors:
        print(f"ERROR: {error}")
    if errors:
        return 1
    print("CI workflow contract valid")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
