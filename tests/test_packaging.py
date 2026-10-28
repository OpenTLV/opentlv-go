# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

import importlib.util
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch
import zipfile

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("package_check", ROOT / "scripts/package_check.py")
packaging = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packaging)


class PackagingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        for name in ("go.mod", "opentlv.go", "CHANGELOG.md", *packaging.LICENSE_FILES):
            (self.root / name).parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / name, self.root / name)

    def test_metadata_rejects_each_missing_licensing_file(self):
        for name in packaging.LICENSE_FILES:
            path = self.root / name
            original = path.read_bytes()
            path.unlink()
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, "Missing module licensing file"):
                packaging.metadata(self.root)
            path.write_bytes(original)

    def test_zip_inputs_require_each_licensing_file(self):
        for omitted in packaging.LICENSE_FILES:
            names = [name for name in packaging.LICENSE_FILES if name != omitted]
            with patch.object(packaging.subprocess, "check_output", return_value="\0".join(names).encode()):
                with self.subTest(omitted=omitted), self.assertRaisesRegex(ValueError, "licensing files in ZIP"):
                    packaging.write_proxy(self.root / ("proxy-" + omitted.replace("/", "-")), self.root)

    def test_module_rejects_legacy_path_replace_and_changed_minimum(self):
        path = self.root / "go.mod"
        original = path.read_text()
        for content in (original.replace(packaging.MODULE, "example.test/old"),
                        original + "\nreplace example.test/other => ../other\n",
                        original.replace("1.22", "1.23")):
            path.write_text(content)
            with self.subTest(content=content), self.assertRaisesRegex(ValueError, "module contract"):
                packaging.metadata(self.root)

    def test_source_version_requires_semver_and_matching_major_path(self):
        path = self.root / "opentlv.go"
        original = path.read_text()
        version = packaging.metadata(self.root)
        for invalid in ("v0.1.0", "0.01.0", "2.0.0", "0.1.0-", "core-main"):
            path.write_text(original.replace(version, invalid))
            with self.subTest(version=invalid), self.assertRaisesRegex(ValueError, "Invalid binding version"):
                packaging.metadata(self.root)

    def test_changelog_is_unreleased_or_closed_under_the_binding_version(self):
        path = self.root / "CHANGELOG.md"
        original = path.read_text()
        version = packaging.metadata(self.root)
        path.write_text(original.replace("## [Unreleased]", f"## [{version}] - 2026-10-28", 1))
        self.assertEqual(packaging.metadata(self.root), version)
        for heading in ("## [0.0.1] - 2026-10-28", f"## [{version}]", "## Unreleased"):
            path.write_text(original.replace("## [Unreleased]", heading, 1))
            with self.subTest(heading=heading), self.assertRaisesRegex(ValueError, "Missing changelog"):
                packaging.metadata(self.root)

    def test_license_requires_original_attribution_and_permission(self):
        path = self.root / "LICENSE"
        original = path.read_text()
        for removed in ("Copyright (c) 2026 Marek Cingel", "Permission is hereby granted, free of charge"):
            path.write_text(original.replace(removed, ""))
            with self.assertRaisesRegex(ValueError, "MIT license"):
                packaging.metadata(self.root)

    def test_zip_has_canonical_prefix_cgo_sources_and_reproducible_bytes(self):
        files = ["go.mod", "opentlv.go", "CHANGELOG.md", *packaging.LICENSE_FILES, "internal/capi/bridge.c"]
        bridge = self.root / files[-1]
        bridge.parent.mkdir(parents=True)
        bridge.write_bytes(b"/* cgo bridge */\n")
        with patch.object(packaging, "module_files", return_value=files):
            version, first = packaging.write_proxy(self.root / "proxy1", self.root)
            _, second = packaging.write_proxy(self.root / "proxy2", self.root)
        self.assertEqual(first, second)
        archive = next((self.root / "proxy1/github.com/!open!t!l!v/opentlv-go/@v").glob("*.zip"))
        with zipfile.ZipFile(archive) as bundle:
            self.assertEqual(bundle.namelist(), [f"{packaging.MODULE}@{version}/{name}" for name in files])
            self.assertEqual(bundle.read(f"{packaging.MODULE}@{version}/{files[-1]}"), bridge.read_bytes())
            for name in packaging.LICENSE_FILES:
                self.assertEqual(bundle.read(f"{packaging.MODULE}@{version}/{name}"),
                                 (self.root / name).read_bytes())

    def test_module_files_reject_legacy_imports_and_missing_inputs(self):
        source = self.root / "bad.go"
        source.write_text('package bad\nimport "github.com/marekcingel/OpenTLV/bindings/go"\n')
        names = "\0".join(["bad.go", *packaging.LICENSE_FILES]).encode()
        with patch.object(packaging.subprocess, "check_output", return_value=names):
            with self.assertRaisesRegex(ValueError, "Legacy module import"):
                packaging.module_files(self.root)
            source.unlink()
            with self.assertRaisesRegex(ValueError, "Invalid module file"):
                packaging.module_files(self.root)


if __name__ == "__main__":
    unittest.main()
