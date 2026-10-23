# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel
"""Keep the portal example catalog aligned with actual Go source inputs."""

import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]


class DocumentationTests(unittest.TestCase):
    def test_catalog_version_and_executable_source(self):
        catalog = json.loads((ROOT / "docs-examples.json").read_text())
        version = re.search(r'const BindingVersion = "([^"]+)"',
                            (ROOT / "opentlv.go").read_text()).group(1)
        self.assertEqual(catalog["version"], 1)
        self.assertEqual(catalog["component_version"], version)
        self.assertEqual(catalog["examples"], [{
            "feature": "fixed.roundtrip", "language": "go", "status": "supported",
            "path": "examples/quick_start/main.go"}])
        source = (ROOT / catalog["examples"][0]["path"]).read_text()
        self.assertIn('"github.com/OpenTLV/opentlv-go"', source)
        self.assertIn('[]byte("Hello, world!")', source)
        guide = (ROOT / "docs/guides/go.md").read_text()
        self.assertIn("python3 scripts/go.py -- run ./examples/quick_start", guide)
        self.assertIn("Hello, world!", guide)


if __name__ == "__main__":
    unittest.main()
