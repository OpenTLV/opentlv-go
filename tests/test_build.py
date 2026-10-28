# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("go_build", ROOT / "scripts/go.py")
build = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build)


class BuildTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()

    def test_lock_rejects_moving_pin_and_foreign_repository(self):
        lock = build.load_lock()
        for field, value in (("commit", "main"), ("commit", "a" * 39),
                             ("repository", "https://other.example/core.git"), ("schema", 2),
                             ("version", None), ("version", "v0.8.0"), ("version", "0.8")):
            with self.subTest(field=field, value=value):
                changed = dict(lock, **{field: value})
                (self.root / "core-source.json").write_text(json.dumps(changed))
                with self.assertRaises(ValueError):
                    build.load_lock(self.root)

    def test_default_selection_never_discovers_sibling_core(self):
        lock = build.load_lock()
        source = self.root / "core"
        source.mkdir()
        selected = {"source_dir": str(source), "repository": build.REPOSITORY,
                    "commit": lock["commit"], "dirty": False}
        with patch.object(build, "identity", return_value=selected):
            actual, record = build.select_core(self.root, None, lock)
        self.assertEqual(actual, source)
        self.assertEqual(record["mode"], "pinned-source")
        for field, value in (("commit", "b" * 40), ("dirty", True)):
            with patch.object(build, "identity", return_value=dict(selected, **{field: value})):
                with self.assertRaises(ValueError):
                    build.select_core(self.root, None, lock)

    def test_override_is_explicit_and_records_dirty_development_identity(self):
        selected = {"source_dir": str(self.root), "repository": build.REPOSITORY,
                    "commit": "b" * 40, "dirty": True}
        with patch.object(build, "identity", return_value=selected), patch.object(build, "run") as run:
            source, record = build.select_core(self.root / "build", self.root, build.load_lock())
        run.assert_not_called()
        self.assertEqual(source, self.root)
        self.assertEqual(record["mode"], "local-source")
        self.assertTrue(record["dirty"])

    def test_go_environment_ignores_workspace_and_external_cgo_flags(self):
        env = build.go_environment(self.root, self.root / "Core source", self.root / "native", False,
                                   "a" * 64, {"GOWORK": "/foreign/go.work", "GOFLAGS": "-mod=mod",
                                              "CGO_CPPFLAGS": "-I/foreign", "CGO_LDFLAGS": "-L/foreign",
                                              "OPENTLV_CORE_SOURCE": "/implicit", "PATH": "tools"})
        self.assertEqual(env["GOWORK"], "off")
        self.assertEqual(env["GOENV"], "off")
        self.assertIn('Core source/tlv/include"', env["CGO_CPPFLAGS"])
        self.assertIn("-DTLV_STATIC_DEFINE", env["CGO_CPPFLAGS"])
        self.assertNotIn("/foreign", env["CGO_LDFLAGS"])
        self.assertNotIn("OPENTLV_CORE_SOURCE", env)
        self.assertTrue(Path(env["GOCACHE"]).is_dir())

    def test_shared_configuration_does_not_define_static_import(self):
        env = build.go_environment(self.root, self.root / "core", self.root / "native", True,
                                   "a" * 64, {})
        self.assertNotIn("TLV_STATIC_DEFINE", env["CGO_CPPFLAGS"])

    def test_native_cache_key_changes_with_library_and_generated_headers(self):
        (self.root / "tlv").mkdir()
        headers = self.root / "generated/include/tlv"
        headers.mkdir(parents=True)
        library = self.root / "tlv/libtlv.a"
        library.write_bytes(b"first library")
        header = headers / "config.h"
        header.write_text("first configuration")
        first = build.native_digest(self.root)
        library.write_bytes(b"second library")
        second = build.native_digest(self.root)
        self.assertNotEqual(first, second)
        header.write_text("second configuration")
        self.assertNotEqual(second, build.native_digest(self.root))

    def test_failed_selection_removes_stale_success_receipt(self):
        receipt = self.root / "core-selection.json"
        receipt.write_text('{"status": "passed"}')
        with patch.object(build, "select_core", side_effect=ValueError("invalid source")):
            with self.assertRaisesRegex(ValueError, "invalid source"):
                build.main(["--build-dir", str(self.root), "--", "build", "./..."])
        self.assertFalse(receipt.exists())

    def test_build_boundary_covers_all_parallel_configurations_and_is_repeatable(self):
        work = self.root / "build/ci/first"
        work.mkdir(parents=True)
        build.isolate_build_output(work, self.root)
        build.isolate_build_output(work, self.root)
        self.assertTrue((self.root / "build/go.mod").is_file())
        self.assertFalse((work / "go.mod").exists())

    def test_build_boundary_never_replaces_an_existing_module(self):
        marker = self.root / "go.mod"
        marker.write_text("module user.example/project\n")
        with self.assertRaisesRegex(ValueError, "Unexpected Go module"):
            build.isolate_build_output(self.root, self.root / "unrelated")
        self.assertEqual(marker.read_text(), "module user.example/project\n")

    @unittest.skipUnless(shutil.which("go"), "Go required for real package discovery")
    def test_go_discovery_excludes_other_builds_generated_cgo_and_toolchain_sources(self):
        (self.root / "go.mod").write_text("module example.test/facade\n\ngo 1.22\n")
        (self.root / "facade.go").write_text("package facade\n")
        generated = self.root / "build/other/go-tmp"
        generated.mkdir(parents=True)
        (generated / "broken.go").write_text("package generated\nimport _ \"internal/runtime/sys\"\n")
        current = self.root / "build/current"
        current.mkdir()
        build.isolate_build_output(current, self.root)
        env = dict(os.environ, GOWORK="off", GOENV="off", GOFLAGS="", GOTOOLCHAIN="local")
        result = subprocess.run(["go", "list", "./..."], cwd=self.root, env=env,
                                capture_output=True, text=True, check=True)
        self.assertEqual(result.stdout.strip(), "example.test/facade")


if __name__ == "__main__":
    unittest.main()
