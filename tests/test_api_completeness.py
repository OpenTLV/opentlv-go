# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""The reviewed contract gate must fail closed on pin, inventory and evidence drift."""

import copy
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
spec = importlib.util.spec_from_file_location("api_completeness", ROOT / "scripts/api_completeness.py")
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
sys.path.pop(0)


class CompletenessTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.report = {"core": {"commit": "a" * 40, "repository": "OpenTLV/opentlv", "version": None},
                       "references": ["native/*.go"], "exclude": ["**/*_test.go"],
                       "scopes": {"core": {"missing": 0, "headers": {"tlv/sample.h": {
                           "referenced": ["tlv_used"], "missing": [], "ignored": ["tlv_alias"]}}}},
                       "ignored": {"tlv_alias": "Public diagnostic variant used."}, "stale_ignores": []}
        self.baseline = {"schema": 1, "inventory": gate.snapshot(self.report), "contracts": [{
            "name": "sample", "surface": "Sample public operation", "limitations": "Owned storage",
            "headers": ["tlv/sample.h"], "tests": ["sample_test.go:TestPublicContract"]}]}
        self.source = "package opentlv_test\nfunc TestPublicContract(t *testing.T) {}\n"
        (self.root / "sample_test.go").write_text(self.source)

    def test_reviewed_contract_and_builtins_separation(self):
        self.report["scopes"]["builtins"] = {"missing": 291}
        self.report["ignored"]["tlv_builtin"] = "Not generic"
        self.assertEqual(gate.validate(self.root, self.baseline, self.report)["status"], "passed")

    def test_pin_reference_and_exclusion_changes_require_review(self):
        mutations = [lambda r: r["core"].update(commit="b" * 40),
                     lambda r: r.update(references=["tests/*.go"]),
                     lambda r: r.update(exclude=[]),
                     lambda r: r.update(stale_ignores=["tlv_removed"]),
                     lambda r: r["scopes"]["core"].update(missing=1),
                     lambda r: r["scopes"]["core"]["headers"]["tlv/sample.h"]["referenced"].append("tlv_added"),
                     lambda r: r["scopes"]["core"]["headers"]["tlv/sample.h"]["referenced"].clear(),
                     lambda r: r["ignored"].update(tlv_alias="Blanket exclusion")]
        for index, mutate in enumerate(mutations):
            report = copy.deepcopy(self.report)
            mutate(report)
            with self.subTest(index=index), self.assertRaises(ValueError):
                gate.validate(self.root, self.baseline, report)

    def test_missing_duplicate_or_unreviewed_contracts_fail(self):
        mutations = [lambda b: b.update(schema=2), lambda b: b.update(contracts=[]),
                     lambda b: b["contracts"].append(copy.deepcopy(b["contracts"][0])),
                     lambda b: b["contracts"][0].update(headers=["tlv/other.h"]),
                     lambda b: b["contracts"][0].update(tests=[]),
                     lambda b: b["contracts"][0].update(surface=""),
                     lambda b: b["contracts"][0].update(limitations=" "),
                     lambda b: b["contracts"][0].update(tests=["../sample_test.go:TestPublicContract"])]
        for index, mutate in enumerate(mutations):
            baseline = copy.deepcopy(self.baseline)
            mutate(baseline)
            with self.subTest(index=index), self.assertRaises(ValueError):
                gate.validate(self.root, baseline, self.report)

    def test_private_removed_or_commented_test_is_not_public_evidence(self):
        for source in (self.source.replace("opentlv_test", "opentlv"),
                       self.source.replace("TestPublicContract", "TestRemoved"),
                       "package opentlv_test\n/* func TestPublicContract(t *testing.T) {} */"):
            (self.root / "sample_test.go").write_text(source)
            with self.subTest(source=source), self.assertRaises(ValueError):
                gate.validate(self.root, self.baseline, self.report)

    def test_cli_failure_removes_stale_success(self):
        output = self.root / "result.json"
        output.write_text('{"status": "passed"}')
        with patch.object(sys, "argv", ["api_completeness.py", "--baseline", str(self.root / "missing.json"),
                                       "--output", str(output)]), self.assertRaises(SystemExit) as caught:
            gate.main()
        self.assertEqual(caught.exception.code, 1)
        self.assertFalse(output.exists())

    def test_repository_baseline_has_real_public_contracts(self):
        baseline = json.loads((ROOT / "ci/api-completeness.json").read_text())
        inventory = baseline["inventory"]
        report = {**inventory, "ignored": inventory["exclusions"], "stale_ignores": [],
                  "scopes": {"core": {"headers": inventory["headers"], "missing": 0}}}
        self.assertEqual(gate.validate(ROOT, baseline, report)["status"], "passed")


if __name__ == "__main__":
    unittest.main()
