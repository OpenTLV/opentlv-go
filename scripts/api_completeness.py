#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Require the reviewed generic API inventory and public contract evidence map.

This review gate complements runtime contract tests; it cannot infer functional
parity from source references. Builtins-specific parity is outside this gate.
"""

import argparse
import json
from pathlib import Path
import re

import api_coverage as api

ROOT = Path(__file__).resolve().parents[1]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def snapshot(report):
    headers = report["scopes"]["core"]["headers"]
    excluded = {name for row in headers.values() for name in row["ignored"]}
    return {"core": report["core"], "references": report["references"],
            "exclude": report["exclude"], "headers": headers,
            "exclusions": {name: report["ignored"][name] for name in sorted(excluded)}}


def validate(root, baseline, report):
    require(baseline.get("schema") == 1, "Unsupported completeness baseline schema")
    require(not report["stale_ignores"], "Stale exclusions require review")
    require(report["scopes"]["core"]["missing"] == 0, "Unreviewed generic Core symbols")
    expected = baseline["inventory"]
    actual = snapshot(report)
    require(actual["core"] == expected["core"], "Core pin changed; review functional contracts before updating the baseline")
    for key in ("references", "exclude", "headers", "exclusions"):
        require(actual[key] == expected[key], f"Generic inventory {key} changed; review the baseline")
    covered = set()
    names = set()
    contracts = baseline.get("contracts", [])
    require(contracts, "Missing functional contract map")
    for contract in contracts:
        name = contract["name"]
        require(name.strip() and name not in names, "Duplicate or empty contract name")
        names.add(name)
        require(contract["surface"].strip() and contract["limitations"].strip(),
                f"Missing public surface or limitations: {name}")
        headers = contract["headers"]
        require(headers and len(headers) == len(set(headers)) and not covered.intersection(headers),
                f"Duplicate or empty contract headers: {name}")
        covered.update(headers)
        require(contract["tests"], f"Missing public contract tests: {name}")
        for anchor in contract["tests"]:
            # Only root public consumer tests are evidence for the facade.
            require(re.fullmatch(r"[a-z0-9_]+_test\.go:Test[A-Za-z0-9_]+", anchor),
                    f"Invalid public test anchor: {anchor}")
            path, function = anchor.split(":")
            source = (root / path).read_text(encoding="utf-8")
            code = api.go_source(source)
            require(re.search(r"(?m)^package\s+opentlv_test\b", code),
                    f"Contract test must use public package: {anchor}")
            require(re.search(r"(?m)^func\s+" + re.escape(function) + r"\(\w+\s+\*testing\.T\)", code),
                    f"Missing public contract test: {anchor}")
    require(covered == set(actual["headers"]), "Contract map must cover every generic header exactly once")
    return {"schema": 1, "status": "passed", "inventory": actual, "contracts": contracts,
            "scope": "Reviewed generic API inventory and test anchors; runtime tests are separately required."}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--core", type=Path, help="existing Git object store containing the exact Core pin")
    parser.add_argument("--baseline", type=Path, default=ROOT / "ci/api-completeness.json")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    # Never leave a stale successful result after a failed validation.
    if args.output:
        args.output.unlink(missing_ok=True)
    try:
        baseline = json.loads(args.baseline.read_text(encoding="utf-8"))
        report = api.report(ROOT, api.load_config(ROOT / "ci/api-coverage.json"), args.core)
        result = validate(ROOT, baseline, report)
        print(api.markdown(report), end="")
        print(f"Reviewed {len(result['contracts'])} public contract families; exact pin, "
              "generic headers, exclusions and public test anchors match.")
        print(result["scope"])
        if args.output:
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    except (OSError, ValueError, KeyError, TypeError) as error:
        parser.exit(1, f"API completeness failed: {error}\n")


if __name__ == "__main__":
    main()
