#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Build the selected Core and run Go with matching native headers and library."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
REPOSITORY = "https://github.com/OpenTLV/opentlv.git"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def run(args, *, cwd=None, env=None):
    print("+ " + subprocess.list2cmdline(list(map(str, args))), flush=True)
    subprocess.run(list(map(str, args)), cwd=cwd, env=env, check=True)


def git(source, *args):
    return subprocess.check_output(["git", "-C", str(source), *args], text=True).strip()


def load_lock(root=ROOT):
    lock = json.loads((root / "core-source.json").read_text(encoding="utf-8"))
    require(lock.get("schema") == 1 and lock.get("repository") == REPOSITORY
            and isinstance(lock.get("commit"), str)
            and re.fullmatch("[0-9a-f]{40}", lock["commit"])
            and re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", str(lock.get("version"))),
            "Invalid immutable Core source lock")
    return lock


def identity(source):
    require(source.is_dir() and Path(git(source, "rev-parse", "--show-toplevel")).resolve()
            == source.resolve(), "Core source must be a Git checkout root")
    origin = git(source, "config", "--get", "remote.origin.url")
    require(origin in (REPOSITORY, REPOSITORY[:-4], "git@github.com:OpenTLV/opentlv.git",
                       "ssh://git@github.com/OpenTLV/opentlv.git"), "Unexpected Core repository")
    return {"source_dir": str(source), "repository": REPOSITORY,
            "commit": git(source, "rev-parse", "HEAD"),
            "dirty": bool(git(source, "status", "--porcelain", "--untracked-files=all"))}


def select_core(build, override, lock):
    if override is not None:
        source = override.resolve()
        return source, dict(identity(source), mode="local-source", locked_commit=lock["commit"])
    source = build / "core"
    if not source.exists():
        run(["git", "clone", "--no-checkout", "--", lock["repository"], source])
        run(["git", "-C", source, "checkout", "--detach", lock["commit"]])
    selected = identity(source)
    require(selected["commit"] == lock["commit"] and selected["dirty"] is False,
            "Managed Core differs from its lock; use a fresh build directory")
    return source, dict(selected, mode="pinned-source", locked_commit=lock["commit"])


def core_digest(source):
    # Go does not invalidate its cache when an external C header/library changes.
    # Partition our cache by all selected Core inputs, including dirty local edits.
    names = subprocess.check_output(["git", "-C", str(source), "ls-files", "-z",
                                     "--cached", "--others", "--exclude-standard"]).decode().split("\0")
    digest = hashlib.sha256()
    for args in (("rev-parse", "HEAD"), ("rev-parse", "--abbrev-ref", "HEAD"),
                 ("describe", "--tags", "--always", "HEAD"), ("rev-list", "--count", "HEAD")):
        digest.update(git(source, *args).encode() + b"\0")
    for name in sorted(set(filter(None, names))):
        path = source / name
        digest.update(name.encode() + b"\0")
        digest.update(path.read_bytes() if path.is_file() else b"<missing>")
    return digest.hexdigest()


def native_digest(native):
    files = list((native / "tlv").glob("libtlv*")) + list((native / "tlv").glob("tlv*.dll"))
    require(files, "Selected Core library is missing")
    files += list((native / "generated/include").rglob("*.h"))
    digest = hashlib.sha256()
    for path in sorted(files):
        require(path.is_file(), f"Invalid native input: {path}")
        digest.update(path.relative_to(native).as_posix().encode() + b"\0" + path.read_bytes())
    return digest.hexdigest()


def isolate_build_output(build, root=ROOT):
    # Git ignores do not affect Go's ./... traversal. Keep temporary cgo files,
    # cached toolchains and parallel configurations outside the public module.
    boundary = root / "build" if build.is_relative_to(root / "build") else build
    marker = boundary / "go.mod"
    content = "module opentlv.local/build-output\n\ngo 1.22\n"
    if marker.exists():
        require(marker.read_text(encoding="utf-8") == content,
                "Unexpected Go module in build output; use a dedicated build directory")
    else:
        # Publish a complete file: parallel helpers must never observe an empty
        # marker between creation and writing. All helpers publish the same text.
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=boundary, delete=False) as stream:
            stream.write(content)
            temporary = Path(stream.name)
        try:
            temporary.replace(marker)
        finally:
            temporary.unlink(missing_ok=True)


def go_environment(build, source, native, shared, digest, base=None):
    env = {key: value for key, value in (os.environ if base is None else base).items()
           if not key.startswith(("CGO_", "OPENTLV_", "CMAKE_", "FETCHCONTENT_"))
           and key not in ("GOFLAGS", "GOWORK", "GOENV", "GOCACHE", "GOTMPDIR",
                           "CFLAGS", "CXXFLAGS", "CPPFLAGS", "LDFLAGS", "LIBRARY_PATH",
                           "CPATH", "C_INCLUDE_PATH", "CPLUS_INCLUDE_PATH")}
    env.update(CGO_ENABLED="1", GOWORK="off", GOENV="off", GOFLAGS="-mod=readonly",
               GOCACHE=str(build / "go-cache" / digest), GOTMPDIR=str(build / "go-tmp"))
    # Quotes are interpreted by cgo, not a shell; they preserve paths with spaces.
    env["CGO_CPPFLAGS"] = (f'"-I{source.as_posix()}/tlv/include" '
                           f'"-I{native.as_posix()}/generated/include"'
                           + ("" if shared else " -DTLV_STATIC_DEFINE"))
    env["CGO_LDFLAGS"] = f'"-L{native.as_posix()}/tlv"'
    if shared:
        loader = "PATH" if os.name == "nt" else "DYLD_LIBRARY_PATH" if sys.platform == "darwin" else "LD_LIBRARY_PATH"
        env[loader] = str(native / "tlv") + os.pathsep + env.get(loader, "")
    Path(env["GOCACHE"]).mkdir(parents=True, exist_ok=True)
    Path(env["GOTMPDIR"]).mkdir(parents=True, exist_ok=True)
    return env


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--core-source", type=Path, help="explicit local development Core checkout")
    parser.add_argument("--build-dir", type=Path, default=ROOT / "build/go")
    parser.add_argument("--shared", action="store_true")
    parser.add_argument("--minimal", action="store_true")
    parser.add_argument("--jobs", type=int, default=2)
    parser.add_argument("--working-dir", type=Path, default=ROOT,
                        help="Go command directory (for an external consumer module)")
    parser.add_argument("command", nargs=argparse.REMAINDER, help="-- build ./... (or another Go command)")
    args = parser.parse_args(argv)
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    require(command and (command[0] in ("build", "test", "run", "vet", "env")
                         or command == ["package-check"]),
            "Supply -- followed by go build, test, run, vet, env or package-check")
    require(args.jobs > 0, "--jobs must be positive")
    lock = load_lock()
    build = args.build_dir.resolve()
    require(not build.is_relative_to(ROOT / "internal") and build != ROOT,
            "Use a dedicated build directory")
    build.mkdir(parents=True, exist_ok=True)
    # Remove stale success evidence before any operation that may fail.
    receipt = build / "core-selection.json"
    receipt.unlink(missing_ok=True)
    isolate_build_output(build)
    source, selected = select_core(build, args.core_source, lock)
    require(not build.is_relative_to(source), "Build output must be outside the selected Core checkout")
    native = build / "native"
    compiler = os.environ.get("CC") or ("gcc" if os.name == "nt" else "cc")
    require(shutil.which(compiler), f"C compiler is unavailable: {compiler}")
    config = {"source_dir": str(source), "compiler": str(Path(shutil.which(compiler)).resolve()),
              "shared": args.shared, "minimal": args.minimal,
              "goos": os.environ.get("GOOS", ""), "goarch": os.environ.get("GOARCH", "")}
    config_path = build / "build-config.json"
    if config_path.exists():
        require(json.loads(config_path.read_text()) == config,
                "Build configuration changed; use a fresh --build-dir")
    else:
        require(not native.exists(), "Unmanaged native build; use a fresh --build-dir")
        config_path.write_text(json.dumps(config, indent=2) + "\n")
    before = core_digest(source)
    env = go_environment(build, source, native, args.shared, before)
    env["CC"] = config["compiler"]
    flags = [f"-DCMAKE_C_COMPILER={config['compiler']}", "-DCMAKE_BUILD_TYPE=Release",
             "-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
             f"-DOPENTLV_BUILD_SHARED_LIBS={'ON' if args.shared else 'OFF'}"]
    flags += [f"-D{name}=OFF" for name in (
        "OPENTLV_BUILD_TESTS", "OPENTLV_BUILD_EXAMPLES", "OPENTLV_BUILD_CXX",
        "OPENTLV_BUILD_DOCS", "OPENTLV_BUILD_BENCHMARKS", "OPENTLV_BUILD_FUZZING",
        "OPENTLV_BUILD_PYTHON", "OPENTLV_BUILD_LUA", "OPENTLV_BUILD_CLI", "OPENTLV_BUILD_WASM")]
    flags += [f"-DOPENTLV_{name}=ON" for name in ("READER", "WRITER", "QUERY", "SCHEMA", "CODEC")]
    if args.minimal:
        flags += [f"-DOPENTLV_{name}=OFF" for name in (
            "FORMAT_ASN1", "BLUETOOTH", "LLDP", "DHCP", "NFC", "DOCUMENT")]
    run(["cmake", "-S", source, "-B", native, "-G", "Ninja", *flags], env=env)
    compiled = json.loads((native / "compile_commands.json").read_text())
    require(any(Path(item["file"]).resolve().is_relative_to(source / "tlv/src")
                for item in compiled), "Build did not select the requested Core sources")
    run(["cmake", "--build", native, "--target", "tlv", "--parallel", args.jobs], env=env)
    native_hash = native_digest(native)
    cache_key = hashlib.sha256((before + native_hash).encode()).hexdigest()
    env["GOCACHE"] = str(build / "go-cache" / cache_key)
    Path(env["GOCACHE"]).mkdir(parents=True, exist_ok=True)
    if command == ["package-check"]:
        from package_check import check_consumer
        check_consumer(env)
    else:
        run(["go", *command], cwd=args.working_dir.resolve(), env=env)
    require(core_digest(source) == before and native_digest(native) == native_hash and identity(source) == {
        key: selected[key] for key in ("source_dir", "repository", "commit", "dirty")},
        "Core inputs changed during the build")
    selected.update(schema=1, input_sha256=before, native_sha256=native_hash, build_dir=str(native),
                    shared=args.shared, minimal=args.minimal, command=command,
                    working_dir=str(args.working_dir.resolve()), status="passed")
    receipt.write_text(json.dumps(selected, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        sys.exit(1)
