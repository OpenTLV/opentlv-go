# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Release failures must precede writes; only the tested commit and its evidence may ship."""

import copy
import json
import os
from pathlib import Path, PurePosixPath, PureWindowsPath
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
import release  # noqa: E402
sys.path.pop(0)
ci = release.ci

IDENTITY = dict(execution="github", commit="a" * 40, sha="a" * 40, dirty=False,
                repository=release.REPOSITORY, run_id="123", run_attempt="2", workflow="Go Release",
                workflow_ref=release.SIGNER + "@refs/heads/main")
DEFINITIONS = {"scripts/release.py": "b" * 64}


def release_tree(root, closed=True):
    for name in ("go.mod", "opentlv.go", "CHANGELOG.md", "docs-examples.json", "ci/release-notes.md",
                 *release.package_check.LICENSE_FILES):
        (root / name).parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, root / name)
    if closed:
        changelog = root / "CHANGELOG.md"
        changelog.write_text(changelog.read_text(encoding="utf-8").replace(
            "## [Unreleased]", "## [0.1.0] - 2026-10-28", 1), encoding="utf-8")


def write_records(folder):
    lock = ci.read(ROOT / "core-source.json")
    for config in ci.matrix("full"):
        evidence = folder / ("go-build-full-" + config["id"] + "-2")
        evidence.mkdir(parents=True)
        work = (PureWindowsPath("D:/a/go/build/work") if config["goos"] == "windows"
                else PurePosixPath("/tmp/go/build/work"))
        record = dict(schema=1, tier="full", status="passed", configuration=config, identity=IDENTITY,
                      definitions=DEFINITIONS, core_lock=lock, work=str(work), checks=[],
                      tools=dict(cc="gcc", cmake="cmake", ninja="ninja", python="3.12",
                                 go=dict(GOVERSION="go1.22.12", GOHOSTOS=config["goos"],
                                         GOHOSTARCH=config["goarch"])))
        for name, requested in ci.stages("full", config):
            log = evidence / (name + ".log")
            log.write_text("test output\n", encoding="utf-8")
            check = dict(name=name, status="passed", exit_code=0, log=log.name, log_sha256=ci.digest(log),
                         goexperiment="cgocheck2" if name == "cgocheck2" else "")
            if requested is not None:
                command = ci.go_command(requested, work, config)
                check.update(go_command=command, core=dict(
                    schema=1, status="passed", mode="pinned-source", dirty=False,
                    repository=lock["repository"], commit=lock["commit"], locked_commit=lock["commit"],
                    command=command, minimal=config["minimal"], shared=config["shared"],
                    input_sha256="c" * 64, native_sha256="d" * 64))
            record["checks"].append(check)
        ci.write(evidence / "record.json", record)


class Preflight(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)

    def test_version_catalog_changelog_and_notes_match_the_first_release(self):
        release_tree(self.root)
        self.assertEqual(release.version_for("v0.1.0", self.root), "0.1.0")
        for tag in ("v0.8.0", "v0.1.0-dev", "v0.1.0-rc.1", "0.1.0", "v00.1.0", "../main"):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                release.version_for(tag, self.root)

    def test_unclosed_changelog_stale_catalog_or_notes_are_rejected(self):
        for name, before, after in (("CHANGELOG.md", "## [0.1.0] - 2026-10-28", "## [Unreleased]"),
                                    ("CHANGELOG.md", "## [0.1.0] - 2026-10-28", "## [0.1.0]"),
                                    ("docs-examples.json", '"0.1.0"', '"0.1.0-dev"'),
                                    ("ci/release-notes.md", "# OpenTLV Go v0.1.0", "# OpenTLV Go v0.2.0")):
            release_tree(self.root)
            path = self.root / name
            path.write_text(path.read_text(encoding="utf-8").replace(before, after, 1), encoding="utf-8")
            with self.subTest(name=name, after=after), self.assertRaises(ValueError):
                release.version_for("v0.1.0", self.root)

    def test_core_lock_must_be_the_published_core_release(self):
        lock = release.go.load_lock()
        self.assertEqual((lock["version"], lock["commit"]),
                         ("0.8.0", "345c77acb29266036df5f27d04ce315bb61853cc"))
        published = {"tag_name": "v0.8.0", "draft": False, "prerelease": False, "html_url": "core-url"}
        with mock.patch.object(release, "api", side_effect=[{"sha": lock["commit"]}, published]) as api:
            self.assertEqual(release.core_release(), {
                "repository": lock["repository"], "version": "0.8.0", "tag": "v0.8.0",
                "commit": lock["commit"], "release": "core-url"})
            self.assertEqual(api.call_args_list[0].args, ("repos/OpenTLV/opentlv/commits/v0.8.0",))
        for replies in ([{"sha": "b" * 40}, published], [{"sha": lock["commit"]}, {**published, "draft": True}],
                        [{"sha": lock["commit"]}, {**published, "prerelease": True}]):
            with mock.patch.object(release, "api", side_effect=replies), self.assertRaises(ValueError):
                release.core_release()

    def test_dispatch_requires_official_main_workflow_and_commit(self):
        env = {"GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REF": "refs/heads/main",
               "GITHUB_WORKFLOW_REF": release.SIGNER + "@refs/heads/main",
               "GITHUB_WORKFLOW_SHA": IDENTITY["commit"]}
        with mock.patch.dict(os.environ, env, clear=True), \
                mock.patch.object(ci, "identity", return_value=IDENTITY):
            self.assertEqual(release.hosted_identity(), IDENTITY)
            for name in env:
                with mock.patch.dict(os.environ, {name: "other"}), self.assertRaises(ValueError):
                    release.hosted_identity()
        for key, value in (("dirty", True), ("execution", "local"), ("repository", "fork/repo")):
            with mock.patch.dict(os.environ, env, clear=True), \
                    mock.patch.object(ci, "identity", return_value={**IDENTITY, key: value}), \
                    self.assertRaises(ValueError):
                release.hosted_identity()

    def test_main_identity_and_latest_failed_attempt(self):
        good = {"head_sha": IDENTITY["commit"], "event": "push", "head_branch": "main",
                "path": ".github/workflows/go-main.yml", "status": "completed",
                "repository": {"full_name": release.REPOSITORY},
                "head_repository": {"full_name": release.REPOSITORY},
                "conclusion": "success", "id": 100, "run_attempt": 2, "html_url": "main-url"}
        release.check_main(good, IDENTITY["commit"])
        for key in ("head_sha", "event", "head_branch", "path", "status", "conclusion"):
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.check_main({**good, key: "wrong"}, IDENTITY["commit"])
        with mock.patch.object(release, "pages", return_value=[good]):
            self.assertEqual(release.main_evidence(IDENTITY["commit"]),
                             {"run_id": 100, "run_attempt": 2, "url": "main-url"})
        for runs in ([], [good, {**good, "id": 101, "conclusion": "failure"}]):
            with mock.patch.object(release, "pages", return_value=runs), self.assertRaises(ValueError):
                release.main_evidence(IDENTITY["commit"])

    def test_unused_tag_rejects_existing_tag_draft_or_changed_main(self):
        main = {"object": {"sha": IDENTITY["commit"]}}
        with mock.patch.object(release, "api", side_effect=[main, []]), \
                mock.patch.object(release, "pages", return_value=[]):
            release.unused_tag("v0.1.0", IDENTITY["commit"])
        for refs, releases in (([{"ref": "refs/tags/v0.1.0"}], []),
                               ([], [{"tag_name": "v0.1.0", "draft": True}])):
            with mock.patch.object(release, "api", side_effect=[main, refs]), \
                    mock.patch.object(release, "pages", return_value=releases), \
                    self.assertRaises(ValueError):
                release.unused_tag("v0.1.0", IDENTITY["commit"])
        with mock.patch.object(release, "api", return_value={"object": {"sha": "b" * 40}}), \
                self.assertRaisesRegex(ValueError, "Main moved"):
            release.unused_tag("v0.1.0", IDENTITY["commit"])

    def test_gate_is_attempt_scoped_and_unique(self):
        good = {"name": "full / checks / Go quality gate", "status": "completed", "conclusion": "success"}
        with mock.patch.object(release, "pages", return_value=[good]) as pages:
            release.check_gate("123", "2")
            pages.assert_called_once_with(f"repos/{release.REPOSITORY}/actions/runs/123/attempts/2/jobs", "jobs")
        for jobs in ([], [good, good], [{**good, "conclusion": "skipped"}], [{**good, "status": "in_progress"}]):
            with mock.patch.object(release, "pages", return_value=jobs), self.assertRaises(ValueError):
                release.check_gate("123", "2")


class Payload(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.evidence = self.root / "evidence"
        self.output = self.root / "release"
        write_records(self.evidence / "builds")
        self.request = {"schema": 1, "tag": "v0.1.0", "version": "0.1.0", "module": release.MODULE,
                        "identity": copy.deepcopy(IDENTITY), "definitions": DEFINITIONS,
                        "core": {"version": "0.8.0"}, "notes_sha256": ci.digest(release.NOTES)}
        self.quality = {"schema": 1, "tier": "full", "status": "passed", "identity": IDENTITY,
                        "configurations": [row["id"] for row in ci.matrix("full")]}
        ci.write(self.evidence / "quality.json", self.quality)
        ci.write(self.evidence / "request.json", self.request)
        for name in ("validate_request", "check_gate"):
            patcher = mock.patch.object(release, name)
            patcher.start()
            self.addCleanup(patcher.stop)

    def assemble(self):
        release.assemble(self.evidence, self.output)

    def assets(self, draft):
        return {"id": 42, "html_url": "https://github.com/OpenTLV/opentlv-go/releases/tag/v0.1.0",
                "tag_name": "v0.1.0", "draft": draft, "prerelease": False,
                "body": release.NOTES.read_text(encoding="utf-8"),
                "assets": [{"name": path.name, "size": path.stat().st_size, "state": "uploaded",
                            "digest": "sha256:" + ci.digest(path)} for path in self.output.iterdir()]}

    def test_complete_full_evidence_notes_and_checksums(self):
        self.assemble()
        self.assertEqual(release.verify_payload(self.output, self.evidence), self.request)
        self.assertEqual({path.name for path in self.output.iterdir()},
                         {"release-manifest.json", "release-notes.md", "SHA256SUMS"})
        self.assertEqual((self.output / "release-notes.md").read_bytes(), release.NOTES.read_bytes())
        manifest = ci.read(self.output / "release-manifest.json")
        self.assertEqual(len(manifest["records"]), len(ci.matrix("full")))

    def test_incomplete_stale_or_failed_full_is_rejected(self):
        records = sorted((self.evidence / "builds").rglob("record.json"))
        for kind in ("missing", "attempt", "definitions", "quality"):
            originals = {path: path.read_bytes() for path in records}
            quality = copy.deepcopy(self.quality)
            if kind == "missing":
                records[0].unlink()
            elif kind == "attempt":
                record = ci.read(records[0])
                record["identity"]["run_attempt"] = "1"
                ci.write(records[0], record)
            elif kind == "definitions":
                record = ci.read(records[0])
                record["definitions"] = {}
                ci.write(records[0], record)
            else:
                quality["configurations"].pop()
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                release.check_full(quality, self.evidence, self.request)
            for path, content in originals.items():
                path.write_bytes(content)
        self.assertFalse(self.output.exists())

    def test_manifest_must_match_current_full_evidence(self):
        self.assemble()
        record = sorted((self.evidence / "builds").rglob("record.json"))[0]
        changed = ci.read(record)
        changed["tools"]["cc"] = "other gcc"
        ci.write(record, changed)
        with self.assertRaisesRegex(ValueError, "differs from Full evidence"):
            release.verify_payload(self.output, self.evidence)

    def test_changed_notes_extra_assets_and_checksums_are_rejected(self):
        self.assemble()
        for filename in ("release-notes.md", "SHA256SUMS", "extra.txt"):
            path = self.output / filename
            original = path.read_bytes() if path.exists() else None
            path.write_bytes(b"changed")
            with self.subTest(filename=filename), self.assertRaises(ValueError):
                release.verify_payload(self.output, self.evidence)
            if original is None:
                path.unlink()
            else:
                path.write_bytes(original)
        notes = self.output / "release-notes.md"
        notes.write_bytes(b"changed")
        (self.output / "SHA256SUMS").write_text(release.checksums(self.output, release.ASSETS), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "notes changed"):
            release.verify_payload(self.output, self.evidence)

    def test_attestation_failure_precedes_any_publication_write(self):
        self.assemble()
        with mock.patch.object(release, "run", side_effect=subprocess.CalledProcessError(1, "gh")) as run, \
                mock.patch.object(release, "unused_tag") as unused, \
                self.assertRaises(subprocess.CalledProcessError):
            release.publish(self.output, self.evidence)
        self.assertEqual(run.call_args.args[1:3], ("attestation", "verify"))
        unused.assert_not_called()

    def test_remote_asset_metadata_is_checked(self):
        self.assemble()
        published = self.assets(True)
        release.verify_published_assets(published, self.output, self.request, True)
        for key, value in (("digest", "sha256:" + "0" * 64), ("size", 0), ("state", "new")):
            changed = copy.deepcopy(published)
            changed["assets"][0][key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                release.verify_published_assets(changed, self.output, self.request, True)
        for changed in ({**published, "assets": published["assets"][:-1]}, {**published, "body": "other"},
                        {**published, "prerelease": True}, {**published, "draft": False}):
            with self.assertRaises(ValueError):
                release.verify_published_assets(changed, self.output, self.request, True)

    def test_publication_verifies_draft_before_making_it_public(self):
        self.assemble()
        draft = self.assets(True)
        for corrupt in (False, True):
            remote = copy.deepcopy(draft)
            if corrupt:
                remote["assets"][0]["digest"] = "sha256:" + "0" * 64
            replies = [remote, {"sha": IDENTITY["commit"]}, self.assets(False), {"sha": IDENTITY["commit"]}]
            with self.subTest(corrupt=corrupt), mock.patch.object(release, "run") as run, \
                    mock.patch.object(release, "unused_tag"), \
                    mock.patch.object(release, "pages", return_value=[draft]), \
                    mock.patch.object(release, "api", side_effect=replies), \
                    mock.patch.object(release, "ROOT", self.root), mock.patch("builtins.print"):
                if corrupt:
                    with self.assertRaisesRegex(ValueError, "digest"):
                        release.publish(self.output, self.evidence)
                else:
                    release.publish(self.output, self.evidence)
                    published = ci.read(self.root / "build/published.json")
                    self.assertEqual((published["url"], published["module"]), (draft["html_url"], release.MODULE))
                calls = [call.args for call in run.call_args_list]
                self.assertTrue(all(args[1:3] == ("attestation", "verify") for args in calls[:3]))
                self.assertEqual(calls[3][1:4], ("api", "--method", "POST"))
                self.assertIn("--draft", calls[4])
                edits = [args for args in calls if args[1:3] == ("release", "edit")]
                self.assertEqual(len(edits), 0 if corrupt else 1)


class Module(unittest.TestCase):
    def setUp(self):
        self.info = {"Path": release.MODULE, "Version": "v0.1.0", "Sum": "h1:module=",
                     "GoModSum": "h1:gomod=", "Origin": {"VCS": "git", "Hash": "a" * 40}}
        self.files = {"go.mod": b"module x\n", "opentlv.go": b"package opentlv\n"}

    def test_public_module_must_match_tag_commit_and_tree(self):
        release.check_module(self.info, dict(self.files), "v0.1.0", "a" * 40, self.files)
        release.check_module({k: v for k, v in self.info.items() if k != "Origin"},
                             dict(self.files), "v0.1.0", "a" * 40, self.files)
        for changed in ({"Version": "v0.1.1"}, {"Path": "example.test/other"}, {"Sum": ""},
                        {"Origin": {"Hash": "b" * 40}}):
            with self.subTest(changed=changed), self.assertRaises(ValueError):
                release.check_module({**self.info, **changed}, dict(self.files), "v0.1.0", "a" * 40, self.files)
        for files in ({**self.files, "extra.go": b""}, {"go.mod": self.files["go.mod"]},
                      {**self.files, "opentlv.go": b"package changed\n"}):
            with self.assertRaisesRegex(ValueError, "ZIP differs"):
                release.check_module(self.info, files, "v0.1.0", "a" * 40, self.files)

    def test_module_files_are_relative_posix_paths(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / "internal/capi").mkdir(parents=True)
            (directory / "go.mod").write_bytes(b"module x\n")
            (directory / "internal/capi/bridge.c").write_bytes(b"/* bridge */\n")
            self.assertEqual(release.module_files(directory),
                             {"go.mod": b"module x\n", "internal/capi/bridge.c": b"/* bridge */\n"})

    def test_unavailable_proxy_fails_after_bounded_retries(self):
        failed = subprocess.CompletedProcess([], 1, "", "not found")
        published = {"tag": "v0.1.0", "identity": {"commit": "a" * 40}}
        with mock.patch.object(ci, "capture", return_value="a" * 40), \
                mock.patch.object(release.subprocess, "run", return_value=failed) as run, \
                mock.patch.object(release.time, "sleep") as sleep, mock.patch("builtins.print"), \
                self.assertRaisesRegex(ValueError, "did not serve"):
            release.verify_module(published, attempts=3, delay=1)
        self.assertEqual(run.call_count, 3)
        self.assertEqual(sleep.call_count, 2)
        env = run.call_args.kwargs["env"]
        self.assertEqual((env["GOPROXY"], env["GOSUMDB"]), ("https://proxy.golang.org", "sum.golang.org"))


class Workflow(unittest.TestCase):
    def test_release_runs_full_before_attesting_publishing_and_proxy_checks(self):
        text = (ROOT / release.WORKFLOW).read_text(encoding="utf-8")
        header, jobs = text.split("\njobs:\n", 1)
        self.assertIn("on:\n  workflow_dispatch:\n", header)
        self.assertNotIn("push:", text)
        # Top-level jobs are the two-space-indented keys under jobs:.
        parts = re.split(r"^  ([a-z_]+):\n", jobs, flags=re.M)[1:]
        sections = dict(zip(parts[::2], parts[1::2]))
        self.assertEqual(list(sections), ["prepare", "full", "assemble", "publish", "module"])
        self.assertIn("needs: prepare\n    uses: ./.github/workflows/go-full.yml", sections["full"])
        self.assertIn("needs: [prepare, full]", sections["assemble"])
        self.assertIn("needs: [full, assemble]", sections["publish"])
        self.assertIn("needs: publish", sections["module"])
        self.assertIn("actions/attest-build-provenance@", sections["assemble"])
        for name, body in sections.items():
            self.assertEqual("contents: write" in body, name == "publish", name)
        self.assertIn("permissions:\n  contents: read\n", header)


if __name__ == "__main__":
    unittest.main()
