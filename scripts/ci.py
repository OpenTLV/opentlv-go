#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Run Go-owned checks and gate complete, attempt-specific CI evidence."""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath, PureWindowsPath
import re
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]
TIERS = ("pr", "main", "full")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def read(path):
    return json.loads(path.read_text(encoding="utf-8"))


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def matrix(tier):
    require(tier in TIERS, "Unknown CI tier")
    rows = [dict(id="linux-default", os="ubuntu-24.04", go="1.22.x", goos="linux",
                 goarch="amd64", architecture="x64", minimal=False, shared=False)]
    if tier == "full":
        rows += [dict(rows[0], id="linux-minimal", minimal=True),
                 dict(rows[0], id="linux-current", go="stable"),
                 dict(rows[0], id="linux-shared", go="stable", shared=True),
                 dict(rows[0], id="windows-default", os="windows-2022", go="stable", goos="windows"),
                 dict(rows[0], id="macos-default", os="macos-15", go="stable", goos="darwin",
                      goarch="arm64", architecture="arm64")]
    return rows


def definition_hashes():
    files = [ROOT / name for name in (".gitattributes", "go.mod", "opentlv.go", "LICENSE", "core-source.json", "migration/source-inventory.json")]
    files += sorted((ROOT / "scripts").glob("*.py"))
    files += sorted((ROOT / "tests").glob("test_*.py"))
    files += sorted((ROOT / "ci").glob("*.json"))
    files += sorted((ROOT / ".github/workflows").glob("*.yml"))
    return {path.relative_to(ROOT).as_posix(): digest(path) for path in files}


def capture(*command, env=None):
    return subprocess.check_output(command, cwd=ROOT, env=env, text=True).strip()


def identity():
    hosted = os.environ.get("GITHUB_ACTIONS") == "true"
    commit = capture("git", "rev-parse", "HEAD")
    dirty = bool(capture("git", "status", "--porcelain", "--untracked-files=all"))
    result = {"execution": "github" if hosted else "local", "commit": commit, "dirty": dirty}
    if hosted:
        for name in ("REPOSITORY", "SHA", "RUN_ID", "RUN_ATTEMPT", "WORKFLOW", "WORKFLOW_REF"):
            value = os.environ.get("GITHUB_" + name)
            require(value, "Missing GitHub identity: " + name)
            result[name.lower()] = value
        require(commit == result["sha"] and not dirty, "Hosted checkout must match the clean event SHA")
    return result


def stages(tier, config):
    result = [("migration", None), ("infrastructure", None), ("format", None),
              ("vet", ["vet", "./..."]), ("test", ["test", "-count=1", "-v", "./..."]),
              ("package", ["package-check"])]
    if tier == "full" and config["id"] in ("linux-default", "linux-minimal"):
        result += [("race", ["test", "-race", "-count=1", "./..."]),
                   ("cgocheck2", ["test", "-count=1", "./..."])]
    examples = ["version", "quick_start", "formats", "streaming", "schema"]
    if tier == "full":
        examples += ["reader", "writer", "errors", "codec"]
        if not config["minimal"]:
            examples += ["document", "query"]
    result += [("example-" + name, ["run", "./examples/" + name]) for name in examples]
    if tier == "full" and not config["minimal"]:
        result += [("query-build", ["build", "-o", "query-conformance", "./examples/query_conformance"]),
                   ("query-conformance", None)]
    return result


def validate_receipt(receipt, command, config, lock):
    require(receipt.get("schema") == 1 and receipt.get("status") == "passed", "Invalid Core receipt")
    require(receipt.get("mode") == "pinned-source" and receipt.get("dirty") is False,
            "CI requires clean pinned Core sources")
    require(receipt.get("repository") == lock["repository"]
            and receipt.get("commit") == receipt.get("locked_commit") == lock["commit"],
            "Core receipt differs from the source lock")
    require(receipt.get("command") == command and receipt.get("minimal") == config["minimal"]
            and receipt.get("shared") == config["shared"], "Core command/configuration mismatch")
    for key in ("input_sha256", "native_sha256"):
        require(re.fullmatch("[0-9a-f]{64}", receipt.get(key, "")), "Missing Core digest: " + key)


def go_command(command, work, config):
    command = list(command)
    if command[:2] == ["build", "-o"]:
        command[2] = str(work / ("query-conformance.exe" if config["goos"] == "windows" else "query-conformance"))
    return command


def run_build(tier, config, work, evidence):
    work = work.resolve()
    evidence = evidence.resolve()
    require(work.is_relative_to(ROOT / "build") and evidence.is_relative_to(ROOT / "build")
            and not evidence.is_relative_to(work), "Use separate work/evidence directories under build/")
    work.mkdir(parents=True, exist_ok=True)
    evidence.mkdir(parents=True, exist_ok=True)
    record = {"schema": 1, "tier": tier, "configuration": config, "status": "failed", "checks": [],
              "work": str(work)}
    # Overwrite stale success before preflight or any command can fail.
    output = evidence / "record.json"
    write(output, record)
    try:
        record.update(identity=identity(), definitions=definition_hashes(), core_lock=read(ROOT / "core-source.json"))
        env = dict(os.environ, PYTHONDONTWRITEBYTECODE="1", GOENV="off", GOWORK="off", GOTOOLCHAIN="local",
                   GOOS=config["goos"], GOARCH=config["goarch"], GOEXPERIMENT="")
        host = {"linux": "linux", "win32": "windows", "darwin": "darwin"}.get(sys.platform)
        require(host == config["goos"], "Run the configuration on its native OS")
        compiler = "gcc" if host != "darwin" else "clang"
        env["CC"] = compiler
        tools = {"go": json.loads(capture("go", "env", "-json", "GOVERSION", "GOHOSTARCH", "GOHOSTOS", env=env)),
                 "cc": capture(compiler, "--version", env=env), "cmake": capture("cmake", "--version", env=env),
                 "ninja": capture("ninja", "--version", env=env), "python": sys.version}
        require(tools["go"]["GOHOSTARCH"] == config["goarch"], "Go host architecture mismatch")
        if config["go"] == "1.22.x":
            require(tools["go"]["GOVERSION"].startswith("go1.22."), "Minimum-Go row requires Go 1.22")
        record["tools"] = tools
        for name, requested in stages(tier, config):
            check = {"name": name, "status": "failed", "log": name + ".log"}
            record["checks"].append(check)
            log = evidence / check["log"]
            step_env = dict(env, GOEXPERIMENT="cgocheck2" if name == "cgocheck2" else "")
            if requested is not None:
                command = go_command(requested, work, config)
                check["go_command"] = command
                argv = [sys.executable, "-B", "scripts/go.py", "--build-dir", str(work)]
                argv += [flag for flag, enabled in (("--minimal", config["minimal"]), ("--shared", config["shared"])) if enabled]
                argv += ["--", *command]
            elif name == "migration":
                argv = [sys.executable, "-B", "scripts/verify_migration.py"]
            elif name == "infrastructure":
                argv = [sys.executable, "-B", "-m", "unittest", "discover", "-s", "tests", "-p", "test_*.py", "-v"]
            elif name == "format":
                files = sorted(ROOT.glob("*.go"))
                for folder in ("internal", "tests", "examples"):
                    files += sorted((ROOT / folder).rglob("*.go"))
                argv = ["gofmt", "-l", *map(str, files)]
            else:
                source = work / "core"
                # Do not let the oracle create bytecode/untracked output in the managed checkout.
                argv = [sys.executable, "-B", str(source / "tests/query/run.py"), "--native",
                        str(work / ("query-conformance.exe" if host == "windows" else "query-conformance")),
                        "--property-cases", "0", "--evidence", str(evidence / "query")]
                if config["shared"]:
                    loader = "PATH" if host == "windows" else "DYLD_LIBRARY_PATH" if host == "darwin" else "LD_LIBRARY_PATH"
                    step_env[loader] = str(work / "native/tlv") + os.pathsep + step_env.get(loader, "")
            check.update(argv=argv, goexperiment=step_env["GOEXPERIMENT"])
            print("+ " + name + ": " + subprocess.list2cmdline(argv), flush=True)
            started = time.monotonic()
            with log.open("w", encoding="utf-8") as stream:
                result = subprocess.run(argv, cwd=ROOT, env=step_env, stdout=stream, stderr=subprocess.STDOUT)
            check.update(exit_code=result.returncode, seconds=round(time.monotonic() - started, 3), log_sha256=digest(log))
            require(result.returncode == 0, f"{name} failed; see {log}")
            if name == "format":
                require(not log.read_text().strip(), "gofmt reported unformatted files")
            if requested is not None:
                receipt = read(work / "core-selection.json")
                validate_receipt(receipt, command, config, record["core_lock"])
                check["core"] = receipt
            check["status"] = "passed"
            write(output, record)
        require(identity() == record["identity"] and definition_hashes() == record["definitions"],
                "Go inputs changed during CI")
        record["status"] = "passed"
    except Exception as error:
        record["error"] = str(error)
        raise
    finally:
        write(output, record)


def validate_records(tier, paths, expected_identity, definitions, lock):
    require(expected_identity.get("execution") == "github" and expected_identity.get("dirty") is False,
            "Local results cannot satisfy a hosted gate")
    expected = {row["id"]: row for row in matrix(tier)}
    found = set()
    for path in paths:
        record = read(path)
        config = record.get("configuration", {})
        name = config.get("id")
        require(name in expected and name not in found, "Unexpected or duplicate configuration")
        found.add(name)
        require(record.get("schema") == 1 and record.get("status") == "passed" and record.get("tier") == tier,
                "Failed or invalid configuration record")
        require(config == expected[name] and record.get("identity") == expected_identity
                and record.get("definitions") == definitions and record.get("core_lock") == lock,
                "Record does not match this run, configuration or source definitions")
        tools = record.get("tools", {})
        require(all(tools.get(key) for key in ("go", "cc", "cmake", "ninja", "python")), "Missing toolchain evidence")
        require(tools["go"].get("GOHOSTOS") == config["goos"]
                and tools["go"].get("GOHOSTARCH") == config["goarch"], "Toolchain target mismatch")
        require(re.fullmatch(r"go1\.\d+\.\d+", tools["go"].get("GOVERSION", "")), "Missing Go version")
        if config["go"] == "1.22.x":
            require(tools["go"]["GOVERSION"].startswith("go1.22."), "Minimum Go version mismatch")
        checks = record.get("checks", [])
        planned = stages(tier, config)
        require([item.get("name") for item in checks] == [name for name, _ in planned], "Missing/reordered/extra checks")
        core_inputs = set()
        for check, (step, requested) in zip(checks, planned):
            require(check.get("status") == "passed" and check.get("exit_code") == 0, "Failed check")
            require(check.get("goexperiment") == ("cgocheck2" if step == "cgocheck2" else ""), "Wrong cgo check mode")
            require(check.get("log") == step + ".log", "Invalid check log path")
            log = path.parent / check["log"]
            require(log.is_file() and digest(log) == check.get("log_sha256"), "Missing or changed check log")
            if requested is not None:
                # Use PureWindowsPath on Linux when interpreting a Windows producer's path.
                work = (PureWindowsPath if config["goos"] == "windows" else PurePosixPath)(record["work"])
                command = go_command(requested, work, config)
                require(check.get("go_command") == command, "Unexpected Go command")
                receipt = check.get("core", {})
                validate_receipt(receipt, command, config, lock)
                core_inputs.add((receipt["input_sha256"], receipt["native_sha256"]))
        require(len(core_inputs) == 1, "Core inputs changed between checks")
    require(found == set(expected), "Missing required configuration evidence")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("matrix", "build", "gate"))
    parser.add_argument("--tier", choices=TIERS, required=True)
    parser.add_argument("--configuration")
    parser.add_argument("--work", type=Path, default=ROOT / "build/ci/work")
    parser.add_argument("--evidence", type=Path, default=ROOT / "build/ci/evidence")
    parser.add_argument("--output", type=Path, default=ROOT / "build/quality.json")
    args = parser.parse_args(argv)
    if args.action == "matrix":
        print(json.dumps({"include": matrix(args.tier)}, separators=(",", ":")))
    elif args.action == "build":
        rows = {row["id"]: row for row in matrix(args.tier)}
        require(args.configuration in rows, "Configuration is not part of this tier")
        run_build(args.tier, rows[args.configuration], args.work / args.configuration,
                  args.evidence / args.configuration)
    else:
        args.output.unlink(missing_ok=True)
        needs = json.loads(os.environ.get("CHECK_RESULTS", "{}"))
        require(set(needs) == {"plan", "build", "quality"}
                and all(item.get("result") == "success" for item in needs.values()),
                "Required CI jobs failed, were skipped, or have no result")
        current = identity()
        validate_records(args.tier, list(args.evidence.rglob("record.json")), current,
                         definition_hashes(), read(ROOT / "core-source.json"))
        write(args.output, {"schema": 1, "tier": args.tier, "status": "passed", "identity": current,
                            "configurations": [row["id"] for row in matrix(args.tier)]})
        print("Go quality gate passed")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError, KeyError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        sys.exit(1)
