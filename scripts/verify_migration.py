#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Verify every imported Go baseline file and its explicitly reviewed adaptations."""

import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def main():
    inventory = json.loads((ROOT / "migration/source-inventory.json").read_text())
    destinations = set()
    standalone = inventory.get("standalone_files", [])
    for item in standalone:
        assert item.get("reason") and "source" not in item, item
    for item in inventory["files"] + standalone:
        destination = item["destination"]
        assert destination not in destinations, destination
        destinations.add(destination)
        path = ROOT / destination
        assert path.resolve().is_relative_to(ROOT), destination
        expected = item.get("adapted_sha256", item["sha256"])
        assert hashlib.sha256(path.read_bytes()).hexdigest() == expected, destination
        if "adapted_sha256" in item:
            assert item.get("adaptation"), destination
    implementation = {str(path.relative_to(ROOT)).replace("\\", "/")
                      for folder in (ROOT / "internal", ROOT / "examples", ROOT / "tests")
                      for path in folder.rglob("*") if path.suffix in (".go", ".c", ".h")}
    implementation.update(path.name for path in ROOT.glob("*.go"))
    assert implementation <= destinations, implementation - destinations
    print(f"Verified {len(inventory['files'])} migrated files, {len(standalone)} standalone additions "
          "and all Go/C implementation sources.")


if __name__ == "__main__":
    main()
