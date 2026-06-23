#!/usr/bin/env python3
"""Tests for the Sprint 14 resilience live validation gate."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import validate_sprint14_resilience_live_gate as gate


class FakeRunner:
    def __init__(self) -> None:
        self.commands: list[list[str]] = []
        self.raw_calls: list[str] = []
        self.lease_holders = ["worker-a", "worker-b"]

    def run(self, command: list[str]) -> str:
        self.commands.append(command)
        joined = " ".join(command)
        if "get pods" in joined and "app=ani-reconcile-worker" in joined:
            return json.dumps(
                {
                    "items": [
                        {
                            "metadata": {
                                "name": "sprint14-worker-a-abc",
                                "labels": {"ani.kubercloud.io/reconcile-identity": "worker-a"},
                            }
                        },
                        {
                            "metadata": {
                                "name": "sprint14-worker-b-def",
                                "labels": {"ani.kubercloud.io/reconcile-identity": "worker-b"},
                            }
                        },
                    ]
                }
            )
        if "get pods" in joined and "app=sprint14-redis" in joined:
            return json.dumps({"items": [{"metadata": {"name": "sprint14-redis-abc"}}]})
        if "get pods" in joined and "app=sprint14-minio" in joined:
            return json.dumps({"items": [{"metadata": {"name": "sprint14-minio-abc"}}]})
        if "get pods" in joined and "app=sprint14-postgres" in joined:
            return "sprint14-postgres-abc\n"
        if "delete pod" in joined:
            return "deleted\n"
        if "rollout status" in joined:
            return "deployment ready\n"
        if command[:2] == ["kubectl", "scale"]:
            return "scaled\n"
        if command[:3] == ["kubectl", "apply", "-f"]:
            return "configured\n"
        raise AssertionError(f"unexpected command: {joined}")

    def get_raw(self, path: str) -> tuple[int, str]:
        self.raw_calls.append(path)
        if len(self.raw_calls) == 1:
            return 200, '{"status":"ok","checks":{"redis":{"status":"ok"},"object-store":{"status":"ok"}}}'
        if len(self.raw_calls) == 2:
            return 200, '{"status":"degraded","checks":{"object-store":{"status":"degraded"}}}'
        if len(self.raw_calls) == 3:
            return 503, '{"status":"fail","checks":{"redis":{"status":"fail"}}}'
        return 200, '{"status":"ok","checks":{"redis":{"status":"ok"},"object-store":{"status":"ok"}}}'

    def query_lease_holder(self, config: gate.LiveConfig) -> str:
        return self.lease_holders.pop(0)


class NeverDegradedRunner(FakeRunner):
    def get_raw(self, path: str) -> tuple[int, str]:
        self.raw_calls.append(path)
        return 200, '{"status":"ok","checks":{"redis":{"status":"ok"},"object-store":{"status":"ok"}}}'


class Sprint14ResilienceLiveGateTest(unittest.TestCase):
    def test_contract_defines_three_sprint14_phases(self) -> None:
        document = gate.load_gate(gate.DEFAULT_GATE)

        gate.validate_contract(document)

        check_ids = {check["id"] for check in document["live_checks"]}
        self.assertIn("p0-readyz-baseline", check_ids)
        self.assertIn("p0-real-backend-kill-strong-dependency", check_ids)
        self.assertIn("p1-real-backend-kill-weak-dependency-degraded", check_ids)
        self.assertIn("p2-primary-kill-controller-failover", check_ids)
        self.assertIn("p2-recovery-readyz-ok", check_ids)

    def test_live_gate_runs_backend_kill_and_primary_failover(self) -> None:
        runner = FakeRunner()

        evidence = gate.run_live(
            gate.LiveConfig(
                namespace="ani-sprint14-resilience",
                manifest=str(gate.DEFAULT_FIXTURE),
                evidence_output="",
                cleanup=False,
            ),
            runner=runner,
        )

        self.assertEqual(evidence["status"], "passed")
        self.assertEqual(evidence["profile"], gate.PROFILE)
        self.assertEqual(evidence["namespace"], "ani-sprint14-resilience")
        self.assertEqual(evidence["p0"]["strong_dependency_kill"]["deleted_pod"], "sprint14-redis-abc")
        self.assertEqual(evidence["p1"]["weak_dependency_kill"]["deleted_pod"], "sprint14-minio-abc")
        self.assertEqual(evidence["p2"]["initial_primary"], "worker-a")
        self.assertEqual(evidence["p2"]["new_primary"], "worker-b")
        self.assertEqual(evidence["production_ready_scope"]["status"], "passed")
        self.assertIn("isolated_sprint14_fixture", evidence["production_ready_scope"]["proof_items"])
        delete_commands = [command for command in runner.commands if command[:3] == ["kubectl", "delete", "pod"]]
        self.assertEqual(len(delete_commands), 3)

    def test_live_gate_restores_scaled_dependencies_when_assertion_fails(self) -> None:
        runner = NeverDegradedRunner()

        with self.assertRaises(SystemExit):
            gate.run_live(
                gate.LiveConfig(
                    namespace="ani-sprint14-resilience",
                    manifest=str(gate.DEFAULT_FIXTURE),
                    evidence_output="",
                    cleanup=False,
                    readyz_attempts=1,
                    readyz_sleep_seconds=0,
                ),
                runner=runner,
            )

        scale_commands = [command for command in runner.commands if command[:2] == ["kubectl", "scale"]]
        self.assertIn(
            ["kubectl", "scale", "deployment/sprint14-minio", "--replicas=1", "-n", "ani-sprint14-resilience"],
            scale_commands,
        )

    def test_readyz_evidence_redacts_internal_endpoints(self) -> None:
        readyz = gate.parse_readyz(
            1,
            'Error from http://sprint14-minio.ani-sprint14-resilience.svc.cluster.local:9000: '
            '{"status":"degraded","checks":{"object-store":{"status":"degraded","error":"dial tcp '
            'sprint14-minio.ani-sprint14-resilience.svc.cluster.local:9000 failed via 10.99.175.245:9000"}}}',
        )

        encoded = json.dumps(readyz, sort_keys=True)
        self.assertNotIn("svc.cluster.local", encoded)
        self.assertNotIn("http://sprint14-minio", encoded)
        self.assertNotIn("10.99.175.245", encoded)
        self.assertIn("<redacted-endpoint>", encoded)

    def test_evidence_output_is_rejected_when_parent_is_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            parent_file = Path(tmpdir) / "not-a-dir"
            parent_file.write_text("x", encoding="utf-8")

            with self.assertRaises(SystemExit) as raised:
                gate.validate_evidence_output(str(parent_file / "evidence.json"))

        self.assertIn("evidence_output parent must be a directory", str(raised.exception))

    def test_main_writes_evidence_when_live_passes(self) -> None:
        document = {
            "profile": gate.PROFILE,
            "status": "contract",
            "required_tools": ["kubectl"],
            "required_endpoints": ["kubernetes_api", "reconcile_worker_metrics", "postgres_control_plane_leases"],
            "live_checks": [
                {"id": check_id, "command": "x", "pass_condition": "x"}
                for check_id in gate.REQUIRED_CHECKS
            ],
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            evidence = Path(tmpdir) / "evidence.json"
            with (
                patch("sys.argv", ["validate_sprint14_resilience_live_gate.py", "--live", "--evidence-output", str(evidence)]),
                patch.object(gate, "load_gate", return_value=document),
                patch.object(gate, "validate_docs"),
                patch.object(gate, "run_live", return_value={"status": "passed", "profile": gate.PROFILE}),
            ):
                self.assertEqual(gate.main(), 0)

            self.assertEqual(json.loads(evidence.read_text(encoding="utf-8"))["status"], "passed")


if __name__ == "__main__":
    unittest.main()
