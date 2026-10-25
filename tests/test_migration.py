# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel
"""Keep new standalone sources explicit without inventing migration provenance."""

import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("verify_migration", ROOT / "scripts/verify_migration.py")
verify = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify)


class MigrationTests(unittest.TestCase):
    def test_standalone_sources_need_hash_reason_and_unique_identity(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            (root / "migration").mkdir()
            source = root / "field.go"
            source.write_bytes(b"package opentlv\n")
            entry = {"destination": "field.go", "sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
                     "reason": "New generic field API after migration"}
            inventory = root / "migration/source-inventory.json"

            def run(entries):
                inventory.write_text(json.dumps({"files": [], "standalone_files": entries}))
                with patch.object(verify, "ROOT", root):
                    verify.main()

            run([entry])
            for entries in ([], [entry, entry], [dict(entry, sha256="0" * 64)],
                            [dict(entry, reason="")], [dict(entry, source="fictional/import.go")]):
                with self.subTest(entries=entries), self.assertRaises(AssertionError):
                    run(entries)
            source.write_bytes(b"package changed\n")
            with self.assertRaises(AssertionError):
                run([entry])


if __name__ == "__main__":
    unittest.main()
