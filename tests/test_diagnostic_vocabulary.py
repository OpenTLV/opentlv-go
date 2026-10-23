# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel
"""Exercise explicit Core selection and read-only diagnostic snapshot checks."""

import contextlib
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "go_vocabulary", ROOT / "scripts/generate_diagnostic_vocabulary.py")
vocabulary = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(vocabulary)


class VocabularyTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.core = self.root / "core"
        self.output = self.root / "go"
        self.output.mkdir()
        (self.core / "tlv/include/tlv").mkdir(parents=True)
        (self.core / "tlv/include/tlv/diagnostic.h").write_text("/* fixture */\n")
        (self.core / "scripts").mkdir()
        # The selected reader supplies metadata; importing it must not invoke main.
        (self.core / "scripts/generate_diagnostic_vocabulary.py").write_text(
            "from pathlib import Path\nROOT = Path('invalid')\n"
            "def load():\n"
            "    assert (ROOT / 'tlv/include/tlv/diagnostic.h').is_file()\n"
            "    return [('ReaderPhase', '', '', '', [('NONE', 0, 'none')])]\n"
            "if __name__ == '__main__':\n    raise RuntimeError('must only import')\n")
        self.addCleanup(patch.stopall)
        patch.object(vocabulary, "ROOT", self.output).start()

    def run_generator(self, *args):
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            return vocabulary.main(["--core-source", str(self.core), *args])

    def test_selected_core_and_snapshot_drift(self):
        core_before = {p.relative_to(self.core): p.read_bytes()
                       for p in self.core.rglob("*") if p.is_file()}
        self.assertEqual(self.run_generator("--check"), 1)
        self.assertEqual(list(self.output.iterdir()), [])
        self.assertEqual(self.run_generator(), 0)
        self.assertEqual({p.name for p in self.output.iterdir()},
                         {"diagnostic_kind.go", "diagnostic_vocabulary_test.go"})
        self.assertEqual(self.run_generator("--check"), 0)
        generated = (self.output / "diagnostic_kind.go").read_text()
        self.assertIn('"github.com/OpenTLV/opentlv-go/internal/capi"', generated)
        self.assertIn("ReaderPhaseNone ReaderPhase = 0", generated)
        test = (self.output / "diagnostic_vocabulary_test.go").read_text()
        self.assertIn("package opentlv_test", test)
        self.assertIn('{tlv.ReaderPhase(1), "unknown"}', test)
        target = self.output / "diagnostic_kind.go"
        target.write_text("drift\n")
        self.assertEqual(self.run_generator("--check"), 1)
        self.assertEqual(target.read_text(), "drift\n")
        self.assertEqual({p.relative_to(self.core): p.read_bytes()
                          for p in self.core.rglob("*") if p.is_file()}, core_before)

    def test_missing_core_fails_before_writes(self):
        (self.core / "tlv/include/tlv/diagnostic.h").unlink()
        with self.assertRaises(SystemExit) as error:
            self.run_generator()
        self.assertEqual(error.exception.code, 2)
        self.assertEqual(list(self.output.iterdir()), [])

    def test_core_selection_is_required(self):
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
            vocabulary.main([])
        self.assertEqual(error.exception.code, 2)


if __name__ == "__main__":
    unittest.main()
