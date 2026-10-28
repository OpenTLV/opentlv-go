#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Check module identity and consume a local module ZIP through Go's file proxy."""

import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
MODULE = "github.com/OpenTLV/opentlv-go"
LICENSE_FILES = ("LICENSE", "LICENSES/MIT.txt", "REUSE.toml", "THIRD_PARTY.md")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def metadata(root=ROOT):
    for name in LICENSE_FILES:
        require((root / name).is_file(), f"Missing module licensing file: {name}")
    mod = (root / "go.mod").read_text(encoding="utf-8")
    require(mod.strip() == f"module {MODULE}\n\ngo 1.22", "Unexpected Go module contract")
    source = (root / "opentlv.go").read_text(encoding="utf-8")
    match = re.search(r'^const BindingVersion = "([^"]+)"$', source, re.M)
    require(match is not None, "Missing binding version")
    version = match[1]
    require(re.fullmatch(r'(0|1)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:dev|rc\.[1-9][0-9]*))?', version),
            "Invalid binding version for the v0/v1 module path")
    license_text = (root / "LICENSE").read_text(encoding="utf-8")
    require(all(value in license_text for value in (
        "MIT License", "Copyright (c) 2026 Marek Cingel",
        "Permission is hereby granted, free of charge", "THE SOFTWARE IS PROVIDED")),
        "Missing MIT license or original attribution")
    # Development keeps Unreleased; a release closes it under the binding version.
    changelog = (root / "CHANGELOG.md").read_text(encoding="utf-8")
    heading = r"^## \[(Unreleased|" + re.escape(version) + r"\] - \d{4}-\d{2}-\d{2}$)"
    require(re.search(heading, changelog, re.M), "Missing changelog")
    return version


def module_files(root=ROOT):
    names = subprocess.check_output(["git", "-C", str(root), "ls-files", "-z",
                                     "--cached", "--others", "--exclude-standard"]).decode().split("\0")
    files = sorted(set(filter(None, names)))
    require(set(LICENSE_FILES) <= set(files), "Missing module licensing files in ZIP inputs")
    for name in files:
        path = root / name
        require(path.is_file() and not path.is_symlink() and path.resolve().is_relative_to(root.resolve()),
                f"Invalid module file: {name}")
        if path.suffix == ".go":
            require("github.com/marekcingel/OpenTLV/bindings/go" not in path.read_text(encoding="utf-8"),
                    f"Legacy module import in {name}")
    return files


def write_proxy(proxy, root=ROOT):
    version = "v" + metadata(root)
    escaped = "".join("!" + ch.lower() if ch.isupper() else ch for ch in MODULE)
    destination = proxy / escaped / "@v"
    destination.mkdir(parents=True)
    (destination / "list").write_text(version + "\n", encoding="utf-8")
    (destination / (version + ".mod")).write_bytes((root / "go.mod").read_bytes())
    (destination / (version + ".info")).write_text(json.dumps({"Version": version,
        "Time": "2000-01-01T00:00:00Z"}), encoding="utf-8")
    archive = destination / (version + ".zip")
    files = module_files(root)
    with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as bundle:
        for name in files:
            # Fixed timestamps keep the local candidate reproducible.
            info = zipfile.ZipInfo(f"{MODULE}@{version}/{name}")
            info.compress_type = zipfile.ZIP_DEFLATED
            bundle.writestr(info, (root / name).read_bytes())
    return version, hashlib.sha256(archive.read_bytes()).hexdigest()


def check_consumer(env, root=ROOT):
    # The consumer, proxy and module cache live outside the binding checkout.
    # No replace directive, workspace, shared module cache or public proxy is used.
    with tempfile.TemporaryDirectory(prefix="opentlv-go-consumer-") as temporary:
        work = Path(temporary).resolve()
        proxy = work / "proxy"
        version, digest = write_proxy(proxy, root)
        consumer = work / "consumer"
        consumer.mkdir()
        (consumer / "go.mod").write_text(
            f"module example.test/consumer\n\ngo 1.22\n\nrequire {MODULE} {version}\n", encoding="utf-8")
        program = '''package main
import (
    "bytes"
    "fmt"
    "runtime/debug"
    opentlv "github.com/OpenTLV/opentlv-go"
)
func main() {
    if opentlv.BindingVersion != "BINDING_VERSION" || opentlv.Version() == "" {
        panic("binding/Core version contract")
    }
    info, ok := debug.ReadBuildInfo()
    if !ok { panic("missing module build info") }
    found := false
    for _, dep := range info.Deps {
        if dep.Path == "github.com/OpenTLV/opentlv-go" {
            if dep.Version != "vBINDING_VERSION" || dep.Replace != nil { panic("wrong module resolution") }
            found = true
        }
    }
    if !found { panic("missing module dependency") }
    format, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
    if err != nil { panic(err) }
    writer := opentlv.NewWriter(format)
    if err := writer.WriteElement([]byte{1}, []byte("external consumer")); err != nil { panic(err) }
    reader := opentlv.NewReader(writer.Bytes(), format)
    if !reader.Next() || !bytes.Equal(reader.Element().Value(), []byte("external consumer")) {
        panic("external consumer round trip failed")
    }
    if reader.Next() || reader.Err() != nil { panic("external consumer reader failed") }
    fmt.Println("Go binding:", opentlv.BindingVersion, "Linked Core:", opentlv.Version())
}
'''.replace("BINDING_VERSION", version[1:])
        (consumer / "main.go").write_text(program, encoding="utf-8")
        selected = dict(env, GOPROXY=proxy.as_uri(), GOSUMDB="off", GOPRIVATE="", GONOPROXY="none",
                        GONOSUMDB="none", GOTOOLCHAIN="local", GOWORK="off", GOENV="off",
                        GOMODCACHE=str(work / "modules"), GOFLAGS="-mod=readonly -modcacherw")
        for command in (["go", "mod", "download", MODULE], ["go", "run", "."]):
            print("+ external consumer: " + " ".join(command), flush=True)
            subprocess.run(command, cwd=consumer, env=selected, check=True)
        print(json.dumps({"module": MODULE, "candidate_version": version, "zip_sha256": digest,
                          "external_consumer": "passed", "publication": False}), flush=True)


if __name__ == "__main__":
    print(f"{MODULE} v{metadata()}")
