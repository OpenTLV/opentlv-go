#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Marek Cingel

"""Publish the Go module tag and release evidence from a clean main dispatch and Full attempt."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import time

import ci
import go
import package_check

ROOT = ci.ROOT
REPOSITORY = "OpenTLV/opentlv-go"
CORE_REPOSITORY = "OpenTLV/opentlv"
MODULE = package_check.MODULE
WORKFLOW = ".github/workflows/go-release.yml"
SIGNER = REPOSITORY + "/" + WORKFLOW
NOTES = ROOT / "ci/release-notes.md"
ASSETS = ("release-manifest.json", "release-notes.md")
require, read, write, digest = ci.require, ci.read, ci.write, ci.digest


def run(*args):
    command = list(map(str, args))
    print("+ " + subprocess.list2cmdline(command), flush=True)
    subprocess.run(command, cwd=ROOT, check=True)


def api(endpoint):
    return json.loads(subprocess.check_output(["gh", "api", endpoint], cwd=ROOT, text=True))


def pages(endpoint, key=None):
    result = []
    for page in range(1, 101):
        separator = "&" if "?" in endpoint else "?"
        data = api(endpoint + f"{separator}per_page=100&page={page}")
        items = data[key] if key else data
        result.extend(items)
        if len(items) < 100:
            return result
    raise ValueError("GitHub pagination limit exceeded")


def version_for(tag, root=ROOT):
    require(re.fullmatch(r"v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", tag),
            "Use a stable vMAJOR.MINOR.PATCH tag")
    version = package_check.metadata(root)
    require(tag == "v" + version, "Tag differs from BindingVersion")
    catalog = json.loads((root / "docs-examples.json").read_text(encoding="utf-8"))
    require(catalog.get("component_version") == version, "Documentation catalog version differs")
    changelog = (root / "CHANGELOG.md").read_text(encoding="utf-8")
    require(re.search(r"^## \[" + re.escape(version) + r"\] - \d{4}-\d{2}-\d{2}$", changelog, re.M),
            "Close the changelog under the release version before releasing")
    notes = (root / NOTES.relative_to(ROOT)).read_text(encoding="utf-8")
    require(notes.startswith(f"# OpenTLV Go {tag}\n") and re.search(r"^- \S", notes, re.M),
            "Missing versioned release notes")
    return version


def core_release():
    lock = go.load_lock()
    tag = "v" + lock["version"]
    require(api(f"repos/{CORE_REPOSITORY}/commits/{tag}")["sha"] == lock["commit"],
            "Core release tag differs from the source lock")
    release = api(f"repos/{CORE_REPOSITORY}/releases/tags/{tag}")
    require(release.get("tag_name") == tag and release.get("draft") is False
            and release.get("prerelease") is False, "Core lock is not a published Core release")
    return {"repository": lock["repository"], "version": lock["version"], "tag": tag,
            "commit": lock["commit"], "release": release["html_url"]}


def hosted_identity():
    current = ci.identity()
    require(current["execution"] == "github" and not current["dirty"]
            and current.get("repository") == REPOSITORY,
            "Release requires the official clean hosted checkout")
    require(os.environ.get("GITHUB_EVENT_NAME") == "workflow_dispatch"
            and os.environ.get("GITHUB_REF") == "refs/heads/main"
            and os.environ.get("GITHUB_WORKFLOW_REF") == SIGNER + "@refs/heads/main"
            and os.environ.get("GITHUB_WORKFLOW_SHA") == current["commit"],
            "Release requires a main dispatch with matching workflow SHA")
    return current


def unused_tag(tag, commit):
    require(api(f"repos/{REPOSITORY}/git/ref/heads/main")["object"]["sha"] == commit,
            "Main moved; validate and release the new commit")
    refs = api(f"repos/{REPOSITORY}/git/matching-refs/tags/{tag}")
    require(not any(ref["ref"] == "refs/tags/" + tag for ref in refs), "Tag already exists")
    releases = pages(f"repos/{REPOSITORY}/releases")
    require(not any(item["tag_name"] == tag for item in releases), "Release already exists")


def check_main(run_info, commit):
    require(run_info.get("head_sha") == commit and run_info.get("event") == "push"
            and run_info.get("head_branch") == "main"
            and run_info.get("path") == ".github/workflows/go-main.yml"
            and run_info.get("repository", {}).get("full_name") == REPOSITORY
            and run_info.get("head_repository", {}).get("full_name") == REPOSITORY
            and run_info.get("status") == "completed" and run_info.get("conclusion") == "success",
            "Successful Main CI on the release commit is required")


def main_evidence(commit):
    runs = pages(f"repos/{REPOSITORY}/actions/workflows/go-main.yml/runs"
                 f"?head_sha={commit}&event=push&branch=main", "workflow_runs")
    require(runs, "Main CI is missing on the release commit")
    latest = max(runs, key=lambda item: item["id"])
    check_main(latest, commit)
    return {"run_id": latest["id"], "run_attempt": latest["run_attempt"], "url": latest["html_url"]}


def check_gate(run_id, attempt):
    jobs = pages(f"repos/{REPOSITORY}/actions/runs/{run_id}/attempts/{attempt}/jobs", "jobs")
    gates = [job for job in jobs if job["name"].endswith(" / Go quality gate")]
    require(len(gates) == 1 and gates[0].get("status") == "completed"
            and gates[0].get("conclusion") == "success", "Missing successful attempt-bound quality gate")


def preflight(tag):
    version = version_for(tag)
    current = hosted_identity()
    unused_tag(tag, current["commit"])
    main = main_evidence(current["commit"])
    check_gate(main["run_id"], main["run_attempt"])
    return {"schema": 1, "tag": tag, "version": version, "module": MODULE, "identity": current,
            "definitions": ci.definition_hashes(), "core": core_release(), "main": main,
            "notes_sha256": digest(NOTES)}


def validate_request(request):
    require(request == preflight(request["tag"]), "Release request, Main evidence or source changed")


def check_full(quality, evidence, request):
    paths = sorted((evidence / "builds").rglob("record.json"))
    ci.validate_records("full", paths, request["identity"], request["definitions"],
                        read(ci.ROOT / "core-source.json"))
    require(quality == {"schema": 1, "tier": "full", "status": "passed", "identity": request["identity"],
                        "configurations": [row["id"] for row in ci.matrix("full")]},
            "Full gate differs from complete build evidence")
    current = request["identity"]
    check_gate(current["run_id"], current["run_attempt"])
    return [read(path) for path in paths]


def checksums(directory, names):
    return "".join(f"{digest(directory / name)}  {name}\n" for name in sorted(names))


def assemble(evidence, output):
    request = read(evidence / "request.json")
    validate_request(request)
    quality = read(evidence / "quality.json")
    records = check_full(quality, evidence, request)
    output.mkdir(parents=True, exist_ok=False)
    shutil.copyfile(NOTES, output / "release-notes.md")
    write(output / "release-manifest.json", {"schema": 1, "request": request, "quality": quality,
                                             "records": records})
    (output / "SHA256SUMS").write_text(checksums(output, ASSETS), encoding="utf-8")


def verify_payload(directory, evidence):
    manifest = read(directory / "release-manifest.json")
    require(manifest.get("schema") == 1, "Unsupported release manifest")
    request = manifest["request"]
    validate_request(request)
    records = check_full(manifest["quality"], evidence, request)
    require(manifest["records"] == records, "Release manifest differs from Full evidence")
    require({path.name for path in directory.iterdir()} == {*ASSETS, "SHA256SUMS"},
            "Unexpected release asset set")
    require((directory / "SHA256SUMS").read_text(encoding="utf-8") == checksums(directory, ASSETS),
            "Release checksums differ")
    require(digest(directory / "release-notes.md") == request["notes_sha256"], "Release notes changed")
    return request


def verify_published_assets(release, directory, request, draft):
    require(release.get("tag_name") == request["tag"] and release.get("draft") is draft
            and release.get("prerelease") is False
            and release.get("body", "").strip() == NOTES.read_text(encoding="utf-8").strip(),
            "Published release metadata differs")
    assets = release.get("assets", [])
    paths = {path.name: path for path in directory.iterdir()}
    require(len(assets) == len(paths) and {asset["name"] for asset in assets} == set(paths),
            "Published release asset set differs")
    for asset in assets:
        path = paths[asset["name"]]
        require(asset.get("state") == "uploaded" and asset.get("size") == path.stat().st_size
                and asset.get("digest") == "sha256:" + digest(path), "Published asset digest differs")


def publish(directory, evidence):
    request = verify_payload(directory, evidence)
    commit = request["identity"]["commit"]
    for path in sorted(directory.iterdir()):
        run("gh", "attestation", "verify", path, "--repo", REPOSITORY,
            "--signer-workflow", SIGNER, "--source-digest", commit)
    # Final read before the first write; never overwrite an existing tag/release.
    unused_tag(request["tag"], commit)
    run("gh", "api", "--method", "POST", f"repos/{REPOSITORY}/git/refs",
        "-f", "ref=refs/tags/" + request["tag"], "-f", "sha=" + commit)
    run("gh", "release", "create", request["tag"], "--repo", REPOSITORY, "--verify-tag",
        "--draft", "--title", "OpenTLV Go " + request["tag"],
        "--notes-file", directory / "release-notes.md", *sorted(directory.iterdir()))
    releases = pages(f"repos/{REPOSITORY}/releases")
    matches = [item for item in releases if item["tag_name"] == request["tag"]]
    require(len(matches) == 1, "Missing or duplicate draft release")
    release = api(f"repos/{REPOSITORY}/releases/{matches[0]['id']}")
    verify_published_assets(release, directory, request, True)
    require(api(f"repos/{REPOSITORY}/commits/{request['tag']}")["sha"] == commit, "Release tag changed")
    run("gh", "release", "edit", request["tag"], "--repo", REPOSITORY, "--draft=false")
    release = api(f"repos/{REPOSITORY}/releases/{release['id']}")
    verify_published_assets(release, directory, request, False)
    require(api(f"repos/{REPOSITORY}/commits/{request['tag']}")["sha"] == commit, "Published tag changed")
    write(ROOT / "build/published.json", {"url": release["html_url"], "tag": request["tag"],
                                          "module": MODULE, "identity": request["identity"],
                                          "core": request["core"],
                                          "assets": [asset["name"] for asset in release["assets"]]})
    print(release["html_url"])


def module_files(directory):
    return {path.relative_to(directory).as_posix(): path.read_bytes()
            for path in directory.rglob("*") if path.is_file()}


def tagged_files(commit):
    names = subprocess.check_output(["git", "-C", str(ROOT), "ls-tree", "-r", "-z", "--name-only", commit])
    return {name: subprocess.check_output(["git", "-C", str(ROOT), "cat-file", "blob", f"{commit}:{name}"])
            for name in filter(None, names.decode().split("\0"))}


def check_module(info, files, tag, commit, expected):
    require(info.get("Path") == MODULE and info.get("Version") == tag
            and str(info.get("Sum", "")).startswith("h1:") and str(info.get("GoModSum", "")).startswith("h1:"),
            "Public module identity differs")
    origin = info.get("Origin")
    require(origin is None or origin.get("Hash") == commit, "Public module origin differs")
    require(files == expected, "Public module ZIP differs from the release commit")


def verify_module(published, attempts=10, delay=30):
    tag, commit = published["tag"], published["identity"]["commit"]
    require(ci.capture("git", "rev-parse", "HEAD") == commit, "Checkout differs from the release commit")
    with tempfile.TemporaryDirectory(prefix="opentlv-go-module-") as temporary:
        work = Path(temporary).resolve()
        # The public proxy and checksum database are the Go distribution channel.
        env = dict(os.environ, GOPROXY="https://proxy.golang.org", GOSUMDB="sum.golang.org",
                   GOPRIVATE="", GONOPROXY="", GONOSUMDB="", GOFLAGS="-modcacherw", GOWORK="off",
                   GOENV="off", GOTOOLCHAIN="local", GOMODCACHE=str(work / "modules"))
        command = ["go", "mod", "download", "-json", f"{MODULE}@{tag}"]
        for attempt in range(attempts):
            result = subprocess.run(command, cwd=work, env=env, capture_output=True, text=True)
            if result.returncode == 0:
                break
            print(result.stdout + result.stderr, file=sys.stderr, flush=True)
            if attempt + 1 < attempts:
                time.sleep(delay)
        require(result.returncode == 0, "Public Go module proxy did not serve the release")
        info = json.loads(result.stdout)
        check_module(info, module_files(Path(info["Dir"])), tag, commit, tagged_files(commit))
    result = {"module": MODULE, "version": tag, "commit": commit, "sum": info["Sum"],
              "go_mod_sum": info["GoModSum"], "proxy": "https://proxy.golang.org"}
    write(ROOT / "build/module.json", result)
    print(json.dumps(result))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("prepare", "assemble", "publish", "module"))
    args = parser.parse_args()
    if args.command == "prepare":
        output = ROOT / "build/request.json"
        output.unlink(missing_ok=True)
        write(output, preflight(os.environ["RELEASE_TAG"]))
    elif args.command == "assemble":
        assemble(ROOT / "build/evidence", ROOT / "build/release")
    elif args.command == "publish":
        (ROOT / "build/published.json").unlink(missing_ok=True)
        publish(ROOT / "build/release", ROOT / "build/evidence")
    else:
        (ROOT / "build/module.json").unlink(missing_ok=True)
        verify_module(read(ROOT / "build/published.json"))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError, KeyError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        sys.exit(1)
