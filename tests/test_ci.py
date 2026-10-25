# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Regressions for CI ownership, missing evidence and false-green gates."""

import copy
import importlib.util
import json
from pathlib import Path, PurePosixPath, PureWindowsPath
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("go_ci", ROOT / "scripts/ci.py")
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class GateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.identity = dict(execution="github", commit="a" * 40, sha="a" * 40, dirty=False,
                             repository="OpenTLV/opentlv-go", run_id="123", run_attempt="2",
                             workflow="Go Full CI", workflow_ref="OpenTLV/opentlv-go/.github/workflows/go-full.yml@refs/heads/main")
        self.definitions = {"scripts/ci.py": "b" * 64}
        self.lock = ci.read(ROOT / "core-source.json")

    def test_definition_bytes_survive_windows_autocrlf_checkout(self):
        source = self.root / "source"
        source.mkdir()
        files = ci.definition_hashes()
        for name in files:
            target = source / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((ROOT / name).read_bytes().replace(b"\r\n", b"\n"))

        def git(*args):
            return subprocess.check_output(["git", "-C", str(source), *args],
                                           stderr=subprocess.STDOUT)

        git("init")
        git("config", "core.autocrlf", "false")
        git("add", ".")
        git("-c", "user.name=CI fixture", "-c", "user.email=ci@example.invalid",
            "-c", "commit.gpgsign=false", "commit", "-m", "Definition fixture")
        checkout = self.root / "windows"
        git("clone", "--no-local", "-c", "core.autocrlf=true", str(source), str(checkout))
        for name in files:
            with self.subTest(path=name):
                self.assertEqual((checkout / name).read_bytes(), git("show", "HEAD:" + name))

    def records(self, tier):
        paths = []
        for config in ci.matrix(tier):
            folder = self.root / config["id"]
            folder.mkdir(exist_ok=True)
            work = PureWindowsPath("D:/a/go/build/work") if config["goos"] == "windows" else PurePosixPath("/tmp/go/build/work")
            record = dict(schema=1, tier=tier, status="passed", configuration=config,
                          identity=self.identity, definitions=self.definitions, core_lock=self.lock,
                          work=str(work), checks=[], tools=dict(cc="gcc", cmake="cmake", ninja="ninja", python="3.12",
                              go=dict(GOVERSION="go1.22.12", GOHOSTOS=config["goos"], GOHOSTARCH=config["goarch"])))
            for name, requested in ci.stages(tier, config):
                log = folder / (name + ".log")
                log.write_text("test output\n", encoding="utf-8")
                check = dict(name=name, status="passed", exit_code=0, log=log.name, log_sha256=ci.digest(log),
                             goexperiment="cgocheck2" if name == "cgocheck2" else "")
                if requested is not None:
                    command = ci.go_command(requested, work, config)
                    check.update(go_command=command, core=dict(schema=1, status="passed", mode="pinned-source", dirty=False,
                        repository=self.lock["repository"], commit=self.lock["commit"], locked_commit=self.lock["commit"],
                        command=command, minimal=config["minimal"], shared=config["shared"],
                        input_sha256="c" * 64, native_sha256="d" * 64))
                record["checks"].append(check)
            path = folder / "record.json"
            ci.write(path, record)
            paths.append(path)
        return paths

    def validate(self, tier, paths, identity=None):
        ci.validate_records(tier, paths, identity or self.identity, self.definitions, self.lock)

    def test_complete_fast_main_and_full_evidence_including_windows_paths(self):
        for tier in ci.TIERS:
            with self.subTest(tier=tier):
                self.validate(tier, self.records(tier))

    def test_missing_and_duplicate_configuration_fail(self):
        paths = self.records("full")
        for selected in ([], paths[:-1], paths + paths[:1]):
            with self.subTest(count=len(selected)), self.assertRaises(ValueError):
                self.validate("full", selected)

    def test_local_evidence_cannot_pass_hosted_gate(self):
        with self.assertRaisesRegex(ValueError, "Local results"):
            self.validate("pr", self.records("pr"), dict(self.identity, execution="local"))

    def test_wrong_run_attempt_commit_repository_workflow_and_dirty_state_fail(self):
        paths = self.records("pr")
        original = ci.read(paths[0])
        for key, value in (("run_id", "124"), ("run_attempt", "1"), ("commit", "f" * 40),
                           ("repository", "other/go"), ("workflow_ref", "other"), ("dirty", True)):
            changed = copy.deepcopy(original)
            changed["identity"][key] = value
            ci.write(paths[0], changed)
            with self.subTest(key=key), self.assertRaises(ValueError):
                self.validate("pr", paths)

    def test_failed_changed_or_incomplete_records_fail(self):
        paths = self.records("pr")
        original = ci.read(paths[0])
        mutations = [lambda r: r.update(status="failed"), lambda r: r.update(tier="full"),
                     lambda r: r.update(definitions={}), lambda r: r.update(tools={}),
                     lambda r: r["configuration"].update(shared=True),
                     lambda r: r["checks"].pop(), lambda r: r["checks"].reverse(),
                     lambda r: r["checks"].append(r["checks"][0]),
                     lambda r: r["checks"][0].update(exit_code=1),
                     lambda r: r["checks"][0].update(log="../outside.log"),
                     lambda r: r["tools"]["go"].update(GOHOSTARCH="386"),
                     lambda r: r["tools"]["go"].update(GOVERSION="go1.23.0")]
        for index, mutate in enumerate(mutations):
            changed = copy.deepcopy(original)
            mutate(changed)
            ci.write(paths[0], changed)
            with self.subTest(index=index), self.assertRaises(ValueError):
                self.validate("pr", paths)

    def test_missing_or_modified_logs_fail(self):
        paths = self.records("pr")
        log = paths[0].parent / "test.log"
        log.write_text("modified")
        with self.assertRaisesRegex(ValueError, "check log"):
            self.validate("pr", paths)
        log.unlink()
        with self.assertRaisesRegex(ValueError, "check log"):
            self.validate("pr", paths)

    def test_local_dirty_wrong_core_or_command_receipts_fail(self):
        paths = self.records("pr")
        original = ci.read(paths[0])
        for key, value in (("mode", "local-source"), ("dirty", True), ("commit", "f" * 40),
                           ("locked_commit", "f" * 40), ("repository", "other"), ("shared", True),
                           ("minimal", True), ("command", ["env"]), ("native_sha256", ""),
                           ("input_sha256", "e" * 64), ("status", "failed")):
            changed = copy.deepcopy(original)
            next(c for c in changed["checks"] if c["name"] == "test")["core"][key] = value
            ci.write(paths[0], changed)
            with self.subTest(key=key), self.assertRaises(ValueError):
                self.validate("pr", paths)

    def test_disabled_cgocheck_fails(self):
        paths = self.records("full")
        record = ci.read(paths[0])
        next(c for c in record["checks"] if c["name"] == "cgocheck2")["goexperiment"] = ""
        ci.write(paths[0], record)
        with self.assertRaisesRegex(ValueError, "cgo check mode"):
            self.validate("full", paths)

    def test_failed_skipped_cancelled_or_missing_jobs_remove_stale_gate(self):
        output = self.root / "quality.json"
        for result in ("failure", "skipped", "cancelled", None):
            needs = {name: {"result": "success"} for name in ("plan", "build", "quality")}
            if result is None:
                needs.pop("plan")
            else:
                needs["build"]["result"] = result
            ci.write(output, {"status": "passed"})
            with patch.dict(ci.os.environ, CHECK_RESULTS=json.dumps(needs)), self.assertRaises(ValueError):
                ci.main(["gate", "--tier", "pr", "--output", str(output)])
            self.assertFalse(output.exists())

    def test_failed_preflight_overwrites_stale_success(self):
        with patch.object(ci, "ROOT", self.root), patch.object(ci, "identity", side_effect=ValueError("preflight")):
            evidence = self.root / "build/evidence"
            ci.write(evidence / "record.json", {"status": "passed"})
            with self.assertRaisesRegex(ValueError, "preflight"):
                ci.run_build("pr", ci.matrix("pr")[0], self.root / "build/work", evidence)
            self.assertEqual(ci.read(evidence / "record.json")["status"], "failed")

    def test_full_preserves_baseline_quality_on_both_feature_sets(self):
        for config in ci.matrix("full")[:2]:
            planned = dict(ci.stages("full", config))
            self.assertEqual(planned["race"], ["test", "-race", "-count=1", "./..."])
            self.assertIn("cgocheck2", planned)
            self.assertIn("example-codec", planned)
            self.assertEqual("query-conformance" in planned, not config["minimal"])
        self.assertEqual(len(ci.matrix("pr")), 1)
        with self.assertRaises(ValueError):
            ci.matrix("release")

    def test_external_package_is_required_in_every_tier_and_configuration(self):
        for tier in ci.TIERS:
            for config in ci.matrix(tier):
                self.assertEqual(dict(ci.stages(tier, config))["package"], ["package-check"])
                self.assertEqual(dict(ci.stages(tier, config))["example-formats"], ["run", "./examples/formats"])
                self.assertEqual(dict(ci.stages(tier, config))["example-schema"], ["run", "./examples/schema"])
                self.assertEqual(dict(ci.stages(tier, config))["example-generation"], ["run", "./examples/generation"])
                self.assertIn("api-completeness", dict(ci.stages(tier, config)))
        paths = self.records("pr")
        record = ci.read(paths[0])
        record["checks"] = [check for check in record["checks"] if check["name"] != "package"]
        ci.write(paths[0], record)
        with self.assertRaisesRegex(ValueError, "Missing/reordered/extra checks"):
            self.validate("pr", paths)


if __name__ == "__main__":
    unittest.main()
