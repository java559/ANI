#!/usr/bin/env python3
"""Validate Sprint 14 Core resilience live gate contract and live fixture."""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml


ROOT = Path(__file__).resolve().parents[1]
DOC_ROOT = ROOT.parent
DEFAULT_GATE = ROOT / "deploy/real-k8s-lab/sprint14-resilience-live-gate.yaml"
DEFAULT_FIXTURE = ROOT / "deploy/real-k8s-lab/sprint14-resilience-live-fixture.yaml"
PROFILE = "SPRINT14-CORE-RESILIENCE-LIVE-GATE"
GATE_ID = "sprint14-resilience-live-gate"
DEFAULT_NAMESPACE = "ani-sprint14-resilience"
DEFAULT_READYZ_PATH = (
    "/api/v1/namespaces/{namespace}/services/"
    "sprint14-reconcile-worker-metrics:9205/proxy/readyz"
)
REQUIRED_CHECKS = {
    "p0-readyz-baseline",
    "p0-real-backend-kill-strong-dependency",
    "p1-real-backend-kill-weak-dependency-degraded",
    "p2-primary-kill-controller-failover",
    "p2-recovery-readyz-ok",
}
REQUIRED_DOC_TOKENS = [
    PROFILE,
    "validate-sprint14-resilience-live-gate",
    "Sprint14 resilience live gate",
    "ani-sprint14-resilience",
]
URL_PATTERN = re.compile(r"https?://[A-Za-z0-9._:-]+")
SERVICE_DNS_PATTERN = re.compile(r"[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.svc(?:\.cluster\.local)?(?::\d+)?")
IPV4_ENDPOINT_PATTERN = re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b")


def fail(message: str) -> None:
    raise SystemExit(f"Sprint14 resilience live gate invalid: {message}")


def gate_path_label(path: Path) -> str:
    try:
        return str(path.relative_to(ROOT))
    except ValueError:
        return str(path)


def load_gate(path: Path) -> dict[str, Any]:
    label = gate_path_label(path)
    if not path.exists():
        fail(f"missing {label}")
    try:
        with path.open(encoding="utf-8") as handle:
            data = yaml.safe_load(handle)
    except OSError:
        fail(f"unreadable {label}")
    except yaml.YAMLError:
        fail(f"malformed {label}")
    if not isinstance(data, dict):
        fail(f"{label} must be a YAML object")
    return data


def validate_contract(document: dict[str, Any]) -> None:
    if document.get("profile") != PROFILE:
        fail(f"profile must be {PROFILE}")
    if document.get("status") not in {"contract", "live"}:
        fail("status must be contract or live")
    tools = document.get("required_tools")
    if not isinstance(tools, list) or "kubectl" not in tools:
        fail("required_tools must include kubectl")
    endpoints = document.get("required_endpoints")
    required_endpoints = {"kubernetes_api", "reconcile_worker_metrics", "postgres_control_plane_leases"}
    if not isinstance(endpoints, list) or required_endpoints - set(endpoints):
        fail("required_endpoints must include Kubernetes API, reconcile metrics and metadata leases")
    checks = document.get("live_checks")
    if not isinstance(checks, list):
        fail("live_checks must be a list")
    check_ids = set()
    for check in checks:
        if not isinstance(check, dict):
            fail("live check must be an object")
        for field in ("id", "command", "pass_condition"):
            value = check.get(field)
            if not isinstance(value, str) or not value.strip():
                fail(f"live check {field} must be a non-empty string")
        check_ids.add(check["id"])
    missing = REQUIRED_CHECKS - check_ids
    if missing:
        fail(f"missing live checks: {', '.join(sorted(missing))}")


def validate_docs() -> None:
    docs = {
        "ANI-DOCS-INDEX.md": DOC_ROOT / "ANI-DOCS-INDEX.md",
        "ANI-06-开发计划.md": DOC_ROOT / "ANI-06-开发计划.md",
        "CURRENT-SPRINT.md": ROOT / "CURRENT-SPRINT.md",
        "development-records/README.md": ROOT / "development-records/README.md",
    }
    for label, path in docs.items():
        try:
            content = path.read_text(encoding="utf-8")
        except FileNotFoundError:
            fail(f"missing doc {label}")
        except OSError:
            fail(f"unreadable doc {label}")
        except UnicodeError:
            fail(f"malformed doc {label}")
        for token in REQUIRED_DOC_TOKENS:
            if token not in content:
                fail(f"{label} must reference {token}")


def validate_path(value: str, label: str) -> None:
    if not value.strip():
        fail(f"{label} must not be empty")
    if value != value.strip():
        fail(f"{label} must not contain surrounding whitespace")


def validate_evidence_output(path: str) -> None:
    validate_path(path, "evidence_output")
    output = Path(path)
    if output.is_dir():
        fail("evidence_output must be a file path")
    if output.parent.exists() and not output.parent.is_dir():
        fail("evidence_output parent must be a directory")
    try:
        output.parent.mkdir(parents=True, exist_ok=True)
    except OSError:
        fail("evidence_output parent must be a directory")


@dataclass(frozen=True)
class LiveConfig:
    namespace: str = DEFAULT_NAMESPACE
    manifest: str = str(DEFAULT_FIXTURE)
    evidence_output: str = ""
    cleanup: bool = False
    kubectl_binary: str = "kubectl"
    readyz_attempts: int = 18
    readyz_sleep_seconds: float = 3.0
    failover_attempts: int = 12
    failover_sleep_seconds: float = 5.0
    lease_name: str = "sprint14-workload-reconcile-controller"


class LiveRunner:
    def __init__(self, kubectl_binary: str = "kubectl") -> None:
        self.kubectl_binary = kubectl_binary

    def run(self, command: list[str]) -> str:
        result = subprocess.run(command, check=False, text=True, capture_output=True)
        if result.returncode != 0:
            detail = result.stderr.strip() or result.stdout.strip()
            raise RuntimeError(f"{' '.join(command)} failed: {detail}")
        return result.stdout

    def get_raw(self, path: str) -> tuple[int, str]:
        result = subprocess.run(
            [self.kubectl_binary, "get", "--raw", path],
            check=False,
            text=True,
            capture_output=True,
        )
        return result.returncode, result.stdout.strip() or result.stderr.strip()

    def query_lease_holder(self, config: LiveConfig) -> str:
        pod = self.run(
            [
                config.kubectl_binary,
                "get",
                "pods",
                "-n",
                config.namespace,
                "-l",
                "app=sprint14-postgres",
                "-o",
                "jsonpath={.items[0].metadata.name}",
            ]
        ).strip()
        if not pod:
            raise RuntimeError("sprint14 postgres pod not found")
        lease_name = config.lease_name.replace("'", "''")
        query = (
            "SELECT holder_id FROM control_plane_leases "
            f"WHERE lease_name = '{lease_name}' AND lease_until > now() "
            "ORDER BY updated_at DESC LIMIT 1"
        )
        return self.run(
            [
                config.kubectl_binary,
                "exec",
                "-n",
                config.namespace,
                pod,
                "--",
                "psql",
                "postgres://ani:ani_dev_password@127.0.0.1:5432/ani?sslmode=disable",
                "-t",
                "-A",
                "-c",
                query,
            ]
        ).strip()


def validate_live_config(config: LiveConfig) -> None:
    validate_path(config.namespace, "namespace")
    if not config.namespace.startswith("ani-sprint14-"):
        fail("live mode namespace must be isolated and start with ani-sprint14-")
    validate_path(config.manifest, "manifest")
    if not Path(config.manifest).exists():
        fail(f"missing manifest {config.manifest}")
    if config.readyz_attempts <= 0 or config.failover_attempts <= 0:
        fail("live attempts must be greater than zero")
    if config.readyz_sleep_seconds < 0 or config.failover_sleep_seconds < 0:
        fail("live sleep seconds cannot be negative")
    if shutil.which(config.kubectl_binary) is None:
        fail(f"{config.kubectl_binary} is required for --live")
    if config.evidence_output:
        validate_evidence_output(config.evidence_output)


def parse_readyz(status_code: int, body: str) -> dict[str, Any]:
    parsed: dict[str, Any] = {"http_status": 200 if status_code == 0 else 503, "raw": redact_internal_endpoints(body[:160])}
    start = body.find("{")
    if start >= 0:
        try:
            payload = json.loads(body[start:])
        except json.JSONDecodeError:
            payload = {}
        if isinstance(payload, dict):
            parsed.update(payload)
            if "http_status" not in payload:
                parsed["http_status"] = 200 if status_code == 0 else 503
    sanitized = redact_evidence(parsed)
    if not isinstance(sanitized, dict):
        fail("readyz evidence must be a JSON object")
    return sanitized


def redact_internal_endpoints(value: str) -> str:
    value = URL_PATTERN.sub("<redacted-endpoint>", value)
    value = SERVICE_DNS_PATTERN.sub("<redacted-endpoint>", value)
    return IPV4_ENDPOINT_PATTERN.sub("<redacted-endpoint>", value)


def redact_evidence(value: Any) -> Any:
    if isinstance(value, str):
        return redact_internal_endpoints(value)
    if isinstance(value, list):
        return [redact_evidence(item) for item in value]
    if isinstance(value, dict):
        return {key: redact_evidence(item) for key, item in value.items()}
    return value


def readyz_path(namespace: str) -> str:
    return DEFAULT_READYZ_PATH.format(namespace=namespace)


def read_readyz(config: LiveConfig, runner: LiveRunner) -> dict[str, Any]:
    status_code, body = runner.get_raw(readyz_path(config.namespace))
    return parse_readyz(status_code, body)


def wait_readyz(config: LiveConfig, runner: LiveRunner, expected_status: str) -> dict[str, Any]:
    last: dict[str, Any] = {}
    for _ in range(config.readyz_attempts):
        last = read_readyz(config, runner)
        if last.get("status") == expected_status:
            return last
        if config.readyz_sleep_seconds:
            time.sleep(config.readyz_sleep_seconds)
    fail(f"readyz did not reach {expected_status}; last={last}")


def wait_readyz_fail(config: LiveConfig, runner: LiveRunner) -> dict[str, Any]:
    last: dict[str, Any] = {}
    for _ in range(config.readyz_attempts):
        last = read_readyz(config, runner)
        if last.get("status") == "fail" or last.get("http_status") == 503:
            return last
        if config.readyz_sleep_seconds:
            time.sleep(config.readyz_sleep_seconds)
    fail(f"readyz did not fail while strong dependency was down; last={last}")


def rollout(config: LiveConfig, runner: LiveRunner, deployment: str) -> None:
    runner.run(
        [
            config.kubectl_binary,
            "rollout",
            "status",
            f"deployment/{deployment}",
            "-n",
            config.namespace,
            "--timeout=180s",
        ]
    )


def list_first_pod(config: LiveConfig, runner: LiveRunner, selector: str) -> str:
    output = runner.run(
        [
            config.kubectl_binary,
            "get",
            "pods",
            "-n",
            config.namespace,
            "-l",
            selector,
            "-o",
            "json",
        ]
    )
    try:
        document = json.loads(output)
    except json.JSONDecodeError as err:
        fail(f"pod list JSON malformed for {selector}: {err}")
    items = document.get("items")
    if not isinstance(items, list) or not items:
        fail(f"selector {selector} matched no pods")
    metadata = items[0].get("metadata", {}) if isinstance(items[0], dict) else {}
    name = metadata.get("name") if isinstance(metadata, dict) else ""
    if not isinstance(name, str) or not name.strip():
        fail(f"selector {selector} returned a pod without name")
    return name


def list_worker_pods(config: LiveConfig, runner: LiveRunner) -> dict[str, str]:
    output = runner.run(
        [
            config.kubectl_binary,
            "get",
            "pods",
            "-n",
            config.namespace,
            "-l",
            "app=ani-reconcile-worker",
            "-o",
            "json",
        ]
    )
    try:
        document = json.loads(output)
    except json.JSONDecodeError as err:
        fail(f"worker pod list JSON malformed: {err}")
    pods: dict[str, str] = {}
    for item in document.get("items", []):
        if not isinstance(item, dict):
            continue
        metadata = item.get("metadata", {})
        if not isinstance(metadata, dict):
            continue
        labels = metadata.get("labels", {})
        name = metadata.get("name")
        if not isinstance(labels, dict) or not isinstance(name, str):
            continue
        identity = labels.get("ani.kubercloud.io/reconcile-identity")
        if isinstance(identity, str) and identity.strip():
            pods[identity] = name
    if len(pods) < 2:
        fail("live mode requires two reconcile workers with unique identities")
    return pods


def scale(config: LiveConfig, runner: LiveRunner, deployment: str, replicas: int) -> None:
    runner.run(
        [
            config.kubectl_binary,
            "scale",
            f"deployment/{deployment}",
            f"--replicas={replicas}",
            "-n",
            config.namespace,
        ]
    )


def restore_scaled_dependencies(config: LiveConfig, runner: LiveRunner, scaled: list[tuple[str, int]]) -> list[str]:
    errors: list[str] = []
    for deployment, replicas in reversed(scaled):
        try:
            scale(config, runner, deployment, replicas)
            rollout(config, runner, deployment)
        except RuntimeError as err:
            errors.append(f"{deployment}: {err}")
    scaled.clear()
    return errors


def delete_pod(config: LiveConfig, runner: LiveRunner, pod: str) -> None:
    runner.run([config.kubectl_binary, "delete", "pod", pod, "-n", config.namespace])


def wait_for_new_leader(config: LiveConfig, runner: LiveRunner, initial: str) -> str:
    for _ in range(config.failover_attempts):
        holder = runner.query_lease_holder(config)
        if holder and holder != initial:
            return holder
        if config.failover_sleep_seconds:
            time.sleep(config.failover_sleep_seconds)
    fail("follower did not acquire leader lease after deleting primary pod")


def run_live(config: LiveConfig, runner: LiveRunner | None = None) -> dict[str, Any]:
    validate_live_config(config)
    runner = runner or LiveRunner(config.kubectl_binary)
    scaled_dependencies: list[tuple[str, int]] = []
    try:
        runner.run([config.kubectl_binary, "apply", "-f", config.manifest])
        for deployment in ("sprint14-postgres", "sprint14-nats", "sprint14-redis", "sprint14-minio", "sprint14-worker-a", "sprint14-worker-b"):
            rollout(config, runner, deployment)

        baseline = wait_readyz(config, runner, "ok")

        minio_pod = list_first_pod(config, runner, "app=sprint14-minio")
        delete_pod(config, runner, minio_pod)
        scale(config, runner, "sprint14-minio", 0)
        scaled_dependencies.append(("sprint14-minio", 1))
        weak_degraded = wait_readyz(config, runner, "degraded")
        restore_scaled_dependencies(config, runner, scaled_dependencies)
        after_weak_recovery = wait_readyz(config, runner, "ok")

        redis_pod = list_first_pod(config, runner, "app=sprint14-redis")
        delete_pod(config, runner, redis_pod)
        scale(config, runner, "sprint14-redis", 0)
        scaled_dependencies.append(("sprint14-redis", 1))
        strong_fail = wait_readyz_fail(config, runner)
        restore_scaled_dependencies(config, runner, scaled_dependencies)
        after_strong_recovery = wait_readyz(config, runner, "ok")

        worker_pods = list_worker_pods(config, runner)
        initial_leader = runner.query_lease_holder(config)
        if not initial_leader:
            fail("no active Sprint14 controller leader lease holder")
        leader_pod = worker_pods.get(initial_leader)
        if not leader_pod:
            fail(f"active leader {initial_leader} does not map to observed worker pods")
        delete_pod(config, runner, leader_pod)
        new_leader = wait_for_new_leader(config, runner, initial_leader)
        final_readyz = wait_readyz(config, runner, "ok")
    except BaseException:
        restore_scaled_dependencies(config, runner, scaled_dependencies)
        raise

    evidence: dict[str, Any] = {
        "id": GATE_ID,
        "profile": PROFILE,
        "status": "passed",
        "namespace": config.namespace,
        "p0": {
            "baseline_readyz": baseline,
            "strong_dependency_kill": {
                "dependency": "redis",
                "deleted_pod": redis_pod,
                "observed_readyz": strong_fail,
                "recovery_readyz": after_strong_recovery,
            },
        },
        "p1": {
            "weak_dependency_kill": {
                "dependency": "object-store",
                "deleted_pod": minio_pod,
                "observed_readyz": weak_degraded,
                "recovery_readyz": after_weak_recovery,
            },
        },
        "p2": {
            "initial_primary": initial_leader,
            "new_primary": new_leader,
            "deleted_primary_pod": leader_pod,
            "final_readyz": final_readyz,
        },
        "production_ready_scope": {
            "status": "passed",
            "scope": "isolated Sprint14 Core resilience fixture",
            "proof_items": [
                "isolated_sprint14_fixture",
                "real_backend_kill_observed",
                "weak_dependency_degraded_observed",
                "controller_primary_kill_failover_observed",
                "readyz_recovered_after_faults",
            ],
            "limitations": [
                "does_not_claim_existing_sprint13_single_replica_backends_are_intrinsically_ha",
                "does_not_claim Redis/Postgres/MinIO/Milvus production operator topology is complete",
            ],
        },
    }
    if config.cleanup:
        runner.run([config.kubectl_binary, "delete", "namespace", config.namespace])
    return evidence


def write_evidence(path: str, evidence: dict[str, Any]) -> None:
    validate_evidence_output(path)
    output = Path(path)
    output.write_text(json.dumps(evidence, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gate", default=str(DEFAULT_GATE), help="Sprint14 resilience live gate YAML")
    parser.add_argument("--manifest", default=str(DEFAULT_FIXTURE), help="isolated Sprint14 live fixture YAML")
    parser.add_argument("--live", action="store_true", help="execute approved live backend kill/failover checks")
    parser.add_argument("--namespace", default=DEFAULT_NAMESPACE)
    parser.add_argument("--evidence-output", default="")
    parser.add_argument("--cleanup", action="store_true")
    parser.add_argument("--kubectl-binary", default="kubectl")
    args = parser.parse_args()

    validate_path(args.gate, "gate")
    document = load_gate(Path(args.gate))
    validate_contract(document)
    validate_docs()
    if args.live:
        evidence = run_live(
            LiveConfig(
                namespace=args.namespace,
                manifest=args.manifest,
                evidence_output=args.evidence_output,
                cleanup=args.cleanup,
                kubectl_binary=args.kubectl_binary,
            )
        )
        if args.evidence_output:
            write_evidence(args.evidence_output, evidence)
            print(f"{PROFILE} live checks passed; evidence written to {args.evidence_output}")
        else:
            print(f"{PROFILE} live checks passed: {json.dumps(evidence, sort_keys=True)}")
        return 0
    print(f"{PROFILE} contract valid; use --live only after human approval")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
