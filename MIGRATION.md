# Migration provenance

Prepared for [Core #685](https://github.com/OpenTLV/opentlv/issues/685), the first
step of [Go extraction epic #684](https://github.com/OpenTLV/opentlv/issues/684).

## Immutable source baseline

| Item | Value |
| --- | --- |
| Original repository | `https://github.com/OpenTLV/opentlv` |
| Original commit | `cb3caed5b09706ce11ecfae839397fd4f99d0025` |
| Implementation subtree | `bindings/go` |
| Original module | `github.com/marekcingel/OpenTLV/bindings/go` |
| Public package / minimum Go | `opentlv` / Go 1.22 |
| Independent binding version | Not assigned; `Version()` currently reports Core |
| Initial standalone commit | `cb63f4e549f22e4d8827acc5b53b09ef4aa8cd8c` |
| License | MIT |

The original implementation is available at the
[pinned source tree](https://github.com/OpenTLV/opentlv/tree/cb3caed5b09706ce11ecfae839397fd4f99d0025/bindings/go).
The companion Core change for #685 records `docs/development/go-migration/`:
its README describes contracts/configurations, `inventory.json` lists original
Git blobs and SHA-256 digests, and `results.json` records local baseline checks.
Use that historical record when transferring sources. The baseline commit above
predates those newly prepared records. Standalone validation is recorded separately
from that original baseline; it does not rewrite the original evidence.

## Source transfer contract

This transfer imports all original public package files, `internal/capi`, tests
and nine examples from the pinned baseline. API behavior, native ownership,
error identity, existing test expectations and source notices are preserved.
`migration/source-inventory.json` records the original hashes, standalone paths,
adapted hashes and reasons. Run `python scripts/verify_migration.py` to verify all
55 imported files. At the #1/#2 transfer, 51 retained their exact original bytes. Three guides
adapt setup and links to standalone ownership; the architecture test excludes
managed build output and Git metadata from its source-boundary walk. No import
path, public API, cgo bridge or behavioral assertion was changed at that stage.
The subsequent #4 module-path/version and #5 test-layout adaptations are recorded below.
The three Go guides (binding README, Core user guide and development guide) are
Go-owned; portal integration follows in Go #6. The public C implementation,
shared Query corpus/oracle, diagnostic enum/name sources and native tests remain
Core-owned. Go may consume those shared inputs from a selected Core revision.

Core's original LICENSE and source headers credit **Marek Cingel, 2026**. The
initial standalone LICENSE credited **OpenTLV, 2026**. The initial transfer retained
both notices; the current LICENSE keeps the original **Marek Cingel, 2026** notice.
The MIT permission notice must accompany imported
sources; repository separation does not replace original authorship attribution.

## Build and release boundary

The baseline requires cgo, a matching C compiler, native public/generated headers
and `-ltlv`. Static linking defines `TLV_STATIC_DEFINE`; shared linking requires
the runtime loader path. Reader, Writer, Query, Schema and Codec must be enabled.
Existing Core Go CI covers Linux amd64 default/minimal static builds; documented
Windows GCC/MinGW and shared-library modes have distinct validation coverage in
the Core baseline record. No pure-Go fallback is introduced.

`core-source.json` pinned the same baseline Core for standalone source builds
during migration; the first Go release moves it to Core `v0.8.0` (#31, see
[packaging](docs/packaging.md#core-compatibility)).
`scripts/go.py` builds it and supplies matching cgo flags without a workspace.
Its explicit `--core-source` option records local Core identity and dirty state;
default selection never discovers a sibling. Static/shared and default/minimal
builds retain the native feature contract. The successful selection receipt is
source-build evidence, not an attested binary or release authorization.

Go #3 establishes Fast/Full CI (see the policy and deviations below);
Go #4 establishes the canonical standalone module path, independent versioning and tag
policy; Go #5 normalizes layout; Go #6 owns documentation integration. Workspace issue #40 integrates this checkout. Core #686 removes the old implementation after its
replacement is ready; Go #7 verifies final cross-repository parity.

This implementation supplies the source-transfer/build prerequisites of workspace
issue #40. The module path changes in Go #4; `Version()` behavior remains unchanged.
Core cleanup, portal source registration and release publication remain separate.
Commit and merge the Go prerequisite before selecting its immutable revision in
the workspace manifest. Local validation cannot establish that later hosted pin.

## Standalone CI and migration deviations (#3)

The [CI policy](docs/development/ci.md) assigns public Go tests and private cgo
integration to this repository. Full retains the original Linux default/minimal
static test, race, `cgocheck2`, vet and example checks and consumes the shared
Query corpus/oracle directly from the locked Core revision. No comprehensive Core
test suite or shared fixture is copied here. PR/Fast and Main run a smaller subset;
Full adds current Go, Linux shared linking, Windows amd64 GCC/UCRT and macOS arm64.
The standalone helper adds a nested module boundary for build output so Go package
discovery cannot collect generated cgo files from another local configuration.

CI builds the immutable Core **source** lock. It does not claim verified binary
consumption, attestations, release readiness or a published Go module. The minimum
Go 1.22 lane preserves the baseline compatibility claim; additional lanes use the
current stable Go selected by setup-go and record its exact version. Race/cgo
instrumentation remains on the two Linux static baseline configurations. Native
sanitizers and the complete native behavioral suite remain Core-owned.

At the #3 stage, the original module path and Core-reporting `Version()` remained unchanged.
Core's old Go workflow remains until the separate #686 cleanup. The only additional
imported content change here is the development guide's link to standalone CI.
The implementation/user guides' adapted hashes are corrected from pre-commit CRLF
bytes to the LF bytes committed under `.gitattributes`; their content is unchanged.
Reasons and adapted hashes are recorded without replacing original source hashes.

Local records are explicitly marked local and cannot pass the hosted quality gate.
Hosted PR/Main/Full acceptance requires successful runs for the committed revision;
adding workflows and passing local checks alone does not establish that evidence.

## Independent module packaging and migration deviations (#4)

The canonical module is now `github.com/OpenTLV/opentlv-go`; all active Go imports,
examples, architecture guards and guides use it. The original module in the
baseline table remains historical evidence. Consumers must update imports; the
old and new module paths identify distinct Go types. No forwarding module is
published. Public parsing/serialization behavior and the native bridge are unchanged.

`BindingVersion` is an additive Go constant, initially `0.1.0-dev`, independent of
Core. `Version()` still calls the linked C library. The version example labels
both identities. [Packaging policy](docs/packaging.md) defines root `vX.Y.Z` tags,
pseudo-versions, Core compatibility, native setup and the unreleased status.
MIT attribution and the original inventory hashes are preserved; adapted hashes
and per-file reasons track each import, documentation and version change.

PR/Fast, Main and Full now consume a local module ZIP from a separate application
with no replacement or workspace, a fresh module cache and the selected native
Core configuration. This proves local external consumption, not public proxy
availability. The synthetic candidate version/time and disabled checksum database
are limited to that temporary test proxy. Core remains a pinned source dependency;
no verified release-binary consumption, tag or publication is introduced.

## Standalone layout and migration deviations (#5)

The standalone root already replaced `bindings/go/` during #1/#2, and #4 established
`github.com/OpenTLV/opentlv-go`. The public facade remains at the module root;
`internal/capi` retains all cgo/C/unsafe implementation and its private tests.
Runnable consumers stay under `examples/`, guides under `docs/`, helpers under
`scripts/`, and standalone workflows under `.github/workflows/`. No `pkg/`, `cmd/`
or additional public package is introduced. The [layout table](README.md#repository-layout)
records these responsibilities.

The fourteen public/architecture Go test files move from `tests/` to the module
root and change from `package tests` to the conventional external test package
`opentlv_test`. Their public imports and behavioral assertions are unchanged.
`element_test.go` keeps `package opentlv` because it exercises private facade
state; `internal/capi/bridge_test.go` stays with the bridge. Consequently,
`go test .` now exercises the public consumer suite, while `go test ./...` still
includes bridge tests and examples. `tests/` retains only Python infrastructure
tests, so Python discovery and existing workflow commands remain valid.

The architecture test now walks from the root, checks the standalone Go source
locations, and prevents external tests as well as examples from importing private
subpackages. Managed `build/` output and Git metadata remain excluded. The source
inventory updates destinations, adapted hashes and reasons while preserving every
original source path and SHA-256. Generated diagnostic vocabulary files remain
imported snapshots: canonical enum/name sources and their generator are Core-owned.

Documentation links follow the moved tests and describe the completed module
identity. Historical `bindings/go` references in baseline provenance are retained.
There are no public import, exported API, native bridge, version, dependency-lock
or runtime behavior changes. Core cleanup, portal integration, final parity and
release publication remain separate work.

## Documentation portal integration and migration deviations (#6)

Usage, build, API contracts, examples and CI instructions are canonical in this
repository. The usage guide now owns its standalone quick start and example
index; exported Go comments remain the local `go doc` reference. Core owns
native semantics, generated C/C++ references, shared Query inputs and the frozen
migration baseline. No native or Go runtime implementation changes are needed.

The portal selects Go separately in `docs-sources.json`, mounts `docs/` at `go/`
and routes public manuals under Languages and CI/implementation under Development.
Superseded Core Go manuals remain historical records with their original source
links and anchors. New technical content is maintained here rather than copied
into Core or the portal. Original inventory identities/hashes are retained;
adapted hashes record only the changed imported usage guide.

`docs-examples.json` exposes the existing quick-start source with the independent
`BindingVersion`. Main documentation notifications propose exact source-pin
updates through the existing portal event model; they do not publish a release.
The initial portal integration can consume the already merged layout baseline.
The new guide and catalog become available in a locked portal build only after
this Go change is merged and its exact SHA is selected there. Local preview
validation is kept separate from that later immutable source set.

Go has no release workflow or independent documentation snapshots yet. The portal
version is not a Go package version, and its selected Core may differ from Go's
native source lock. Example availability is not evidence of compatibility with
that Core revision. Hosted Main/notifier, portal CI, Pages deployment and live
URL validation must be recorded separately; no tags or releases are introduced.

## Core ownership cleanup (#686)

Core removes its old Go source tree, workflow, manuals, baseline files and
reproduction script. The original records remain available at the immutable
[Core baseline-record commit](https://github.com/OpenTLV/opentlv/tree/ec077a7eee7f59110c7b768acbb50015d86ab9df/docs/development/go-migration).
The [original reproduction script](https://github.com/OpenTLV/opentlv/blob/ec077a7eee7f59110c7b768acbb50015d86ab9df/scripts/go_migration_baseline.py)
must be run from that historical Core checkout; current Core no longer supplies
these files. This repository's original source hashes and imported snapshots
remain unchanged.

Go now owns `scripts/generate_diagnostic_vocabulary.py`. It reads canonical enum
and name metadata through the explicitly selected Core checkout's reader, then
generates only the two Go snapshots in this repository. `--check` is read-only.
The rendered snapshots are byte-identical to the existing standalone files;
no runtime/API change, Core lock update, new forwarding module or publication
is introduced. Run snapshot maintenance from this repository using the
standalone command in the development guide.

The portal must support source sets both before and after the old Core records
disappear. Adopt its compatibility change before advancing its Core pin.
Local checks against modified Core are not hosted Full or release evidence.
Final cross-repository parity remains tracked by Go #7.

The prerequisite hosted evidence reviewed for this cleanup is:

| Check | Recorded evidence | Limit |
| --- | --- | --- |
| Go Main and external package consumption | [run 39967471049](https://github.com/OpenTLV/opentlv-go/actions/runs/39967471049), `a4d72386a4392f02b133d9fcf6703bab52c749b6` | Uses the original Core lock |
| Go Full, all six lanes | [run 39856632068](https://github.com/OpenTLV/opentlv-go/actions/runs/39856632068) | Predates module packaging, layout and documentation changes; current-candidate Full remains required |
| Workspace Main configuration/bootstrap checks | [run 39853011741](https://github.com/OpenTLV/opentlv-workspace/actions/runs/39853011741) | Predates the Core cleanup candidate; does not execute the Go smoke |
| Portal Main with standalone Go source selection | [run 39968006266](https://github.com/OpenTLV/opentlv-docs/actions/runs/39968006266), `b0c1adb` | Selects Go `a4d72386a4392f02b133d9fcf6703bab52c749b6`; does not validate this Core cleanup |

Local cleanup validation covers Core CTest, standalone Go default/minimal/shared
builds, external module consumption, examples and Query conformance. Those local
checks do not replace current-candidate hosted Core, Go Full or workspace checks.

## Final cross-repository review (#7)

Reviewed on **2026-10-24** for [Go #7](https://github.com/OpenTLV/opentlv-go/issues/7)
and [Core epic #684](https://github.com/OpenTLV/opentlv/issues/684).
The source migration is implemented and the reviewed behavioral changes are
limited to the adaptations below. **Go candidate PR and Full validation passed**
on `c6af85d2c11cfc6c98897398b2ec3b6e5335a491`, including all six native
configurations and the aggregate quality gate. **Final cross-repository acceptance
remains pending workspace validation** as detailed in the follow-up evidence below.
The closing Go PR must satisfy that prerequisite before merge; no release is
authorized by this review.

### Initial reviewed identities

These are immutable review inputs, not moving references to the latest branches.
The initial documentation review was based on the Go commit below. Later repair
and workspace revisions are recorded separately in the follow-up evidence.

| Input | Exact revision |
| --- | --- |
| Original Go source and current native Core lock | `cb3caed5b09706ce11ecfae839397fd4f99d0025` |
| Original baseline records in Core | `ec077a7eee7f59110c7b768acbb50015d86ab9df` |
| Standalone Go after cleanup support | `8c62688ebe0022a21b2a0cdb4959ad31034e5aff` |
| Core after removal of Go ownership | `a9a6f9470c2b285744738b40196bef9d0341fead` |
| Docs after both source-pin updates | `996628be4afe014c3245ff571b1d5140fa79a56d` |
| Workspace with Go registration | `721c7876ba1fa3ea7aae3f24b7677ab3ec3b81df` |
| Go selected by that workspace manifest | `53663060ac1bb9d198f36c3cb5047182d42cb95d` |
| Core selected by that workspace manifest | `1ed4e0f63360755472bdebee1fa1e1609b190e98` |

The reviewed [Docs manifest](https://github.com/OpenTLV/opentlv-docs/blob/996628be4afe014c3245ff571b1d5140fa79a56d/docs-sources.json)
selects Go `8c62688ebe0022a21b2a0cdb4959ad31034e5aff`
and Core `a9a6f9470c2b285744738b40196bef9d0341fead`. The Go native lock remains
at the original baseline intentionally. Portal source selection is not a native
compatibility claim; a lock update is a separate reviewed dependency change.
The [workspace manifest](https://github.com/OpenTLV/opentlv-workspace/blob/721c7876ba1fa3ea7aae3f24b7677ab3ec3b81df/integration.json)
selected the older component set in remote mode. It is retained here as historical
evidence and cannot establish final migration alignment.

### Source and ownership parity

All 55 original SHA-256 values in `migration/source-inventory.json` were checked
against `git show <original-commit>:<source>` in the Core object database, in
addition to checking every current destination with `scripts/verify_migration.py`.
No original hash was replaced. The direct source comparison found:

| Files | Result |
| --- | --- |
| 14 | Byte-identical to Core's original blobs, including every private bridge source and its tests |
| 35 | Identical after substituting the canonical module import and, for moved public tests, `package tests` with `package opentlv_test` |
| 3 guides | Standalone build, ownership, examples and documentation links; API contracts retained |
| `opentlv.go` | Adds only `BindingVersion = "0.1.0-dev"` and its comment after import substitution; `Version()` remains the Core version |
| `examples/version/main.go` | Labels the binding and linked-Core versions separately |
| `architecture_test.go` | Walks the root layout, skips managed output and checks external-test/private-import boundaries |

To repeat the original-blob check from the Go root with a Core checkout that
contains the historical commit, run the destination verifier and this Python
snippet. Review each adapted destination against its original blob using the
inventory's source/destination mapping; the adaptation reason is not itself
proof of parity.

```sh
python3 -B scripts/verify_migration.py
```

```python
import hashlib
import json
import subprocess
from pathlib import Path

inventory = json.loads(Path("migration/source-inventory.json").read_text())
for item in inventory["files"]:
    original = subprocess.check_output([
        "git", "-C", "../opentlv", "show",
        inventory["commit"] + ":" + item["source"],
    ])
    assert hashlib.sha256(original).hexdigest() == item["sha256"], item["source"]
print("Verified", len(inventory["files"]), "original Core blobs")
```

All nine examples and all original behavioral tests are present. The module
path change requires consumer import updates and changes Go type identity;
there is no forwarding module. This is a documented migration compatibility
change, not a claim of unchanged imports. Apart from the additive binding
version constant, existing exported declarations and implementation are
unchanged after the import substitution. Behavioral test assertions are
unchanged; the architecture test is the explicit structural exception.

The unchanged bridge retains synchronous borrowing, copied diagnostics,
Document/Query native ownership and cleanup, and the existing callback and
pointer rules. No C parser, codec, Query implementation or native behavioral
suite was copied into this repository. Core keeps its native tests and shared
Query corpus/oracle. Go consumes that corpus from the selected Core checkout.

Core's reviewed tree contains no `bindings/go`, Go workflow, Go manuals,
baseline directory or baseline reproduction script. Its diagnostic generator
no longer renders Go. The Go-owned generator produces the existing snapshots
from explicitly selected canonical Core metadata. The original baseline and
reproduction script remain accessible at the immutable links above.
`git diff cb3caed5b09706ce11ecfae839397fd4f99d0025
a9a6f9470c2b285744738b40196bef9d0341fead -- tlv tests/query` in Core is empty:
the native library sources and shared Query corpus are unchanged by this
baseline-record/cleanup sequence.

### Hosted evidence and its limits

All runs below were inspected through GitHub run/job metadata; the cited runs
completed successfully on **attempt 1**. Job logs additionally confirm the Main
gate, actual notification dispatch, receiver proposal, locked Docs source set
and the limited scope of Workspace Main. A passed run applies only to its
recorded revision and configuration.

| Evidence | Revision | What it establishes |
| --- | --- | --- |
| [Go PR 39981945851](https://github.com/OpenTLV/opentlv-go/actions/runs/39981945851) | `e48912e29c2746e15581e525cef7fe8bc03bd676` | Fast passed for the cleanup-support PR head; the workflow tests its PR merge checkout |
| [Go Main 39984809182](https://github.com/OpenTLV/opentlv-go/actions/runs/39984809182) | `8c62688ebe0022a21b2a0cdb4959ad31034e5aff` | Linux Go 1.22 checks, external module consumption, workflow validation and Go quality gate |
| [Go Full 39856632068](https://github.com/OpenTLV/opentlv-go/actions/runs/39856632068) | `3033d67714b7a0b7397c12448c647fa457723406` | All six native configurations and aggregate gate passed; historical only, before #4/#5/#6 and cleanup support |
| [Go notifier 39984998214](https://github.com/OpenTLV/opentlv-go/actions/runs/39984998214) | `8c62688ebe0022a21b2a0cdb4959ad31034e5aff` | Docs accepted an actual notification for Main 39984809182; not a no-change skip |
| [Docs receiver 39985022642](https://github.com/OpenTLV/opentlv-docs/actions/runs/39985022642) | `4b1d9139f3bbc4965e823c3448819c8cdd316bf9` | Verified that Go producer and created [Docs PR #91](https://github.com/OpenTLV/opentlv-docs/pull/91) |
| [Docs Main 39985377039](https://github.com/OpenTLV/opentlv-docs/actions/runs/39985377039) | `996628be4afe014c3245ff571b1d5140fa79a56d` | Locked strict build with Go `8c62688` and cleaned Core `a9a6f94`; no Pages deployment |
| [Core Main 39984829979](https://github.com/OpenTLV/opentlv/actions/runs/39984829979) | `a9a6f9470c2b285744738b40196bef9d0341fead` | Linux native consumer smoke and Core quality gate after cleanup; not Core Full |
| [Workspace Main 39853011741](https://github.com/OpenTLV/opentlv-workspace/actions/runs/39853011741) | `721c7876ba1fa3ea7aae3f24b7677ab3ec3b81df` | Configuration/reference checks and 16 bootstrap tests; **does not build Go** |
| [Workspace Full 39817682277](https://github.com/OpenTLV/opentlv-workspace/actions/runs/39817682277) | `d446afd54031cf7168b952c6556e1b9d2992dcb5` | Predates Go registration; not Go integration evidence |

The Go Main gate log verifies the download of
`go-build-main-linux-default-1` (artifact `12515112008`, ZIP SHA-256
`80db90bdfe7ea3b2dc83db310c8d5221c23aa471c88f76f5026574d59b27002b`),
then passes `scripts/ci.py gate --tier main`. It publishes
`go-quality-main-1` (artifact `12515755553`, ZIP SHA-256
`e01fc0d7d71578cab39f3cc8457b9d03483eb3fb69fc0d3b5d6c69823b40c110`).
The artifact metadata and gate log are the inspected hosted evidence; this
review does not claim independent replay of the downloaded hosted gate.
Artifacts can expire; retain the run, attempt, commit and digests together.

### Follow-up validation and gate repair

[Go Full 40071215912, attempt 1](https://github.com/OpenTLV/opentlv-go/actions/runs/40071215912)
ran on the published review candidate
`2a52389b7ee9dfbd895841a1840153d9cd8a41f4`. All six native configurations and
workflow validation passed, but the aggregate job
[120805281841](https://github.com/OpenTLV/opentlv-go/actions/runs/40071215912/job/120805281841)
failed. This run is not successful Full evidence.

The downloaded Linux and Windows records have identical execution identities.
The only differing CI definition is `LICENSE`: Windows checked it out with CRLF
because its extensionless path had no explicit LF rule. Linux recorded SHA-256
`9d1139c42bde151af259ca7196c9da2f19d85fbd3b8771a3d3f0e1e5f6490c9c`,
Windows `715fdd19baa98d5687210d1b2e141c5ecf313deec8ece76d79b53f232df5ba52`.
The repair declares LF for `LICENSE` and `.gitattributes`, binds the attributes
policy into CI definition hashes, and checks actual Git checkout bytes with
`core.autocrlf=true`. No evidence comparison is relaxed.

The committed repair `c6af85d2c11cfc6c98897398b2ec3b6e5335a491` passed:

| Evidence (attempt 1) | Result |
| --- | --- |
| [Go PR 40071831858](https://github.com/OpenTLV/opentlv-go/actions/runs/40071831858) | Successful PR checks for the repair head; the PR workflow tests its merge checkout |
| [Go Full 40072168513](https://github.com/OpenTLV/opentlv-go/actions/runs/40072168513) | All six native configurations, plan and workflow validation passed |
| [Go quality gate 120808284936](https://github.com/OpenTLV/opentlv-go/actions/runs/40072168513/job/120808284936) | Downloaded all six same-attempt artifacts and passed strict record, source-definition and log verification |

The gate published `go-quality-full-1`, artifact `12551283098`, ZIP SHA-256
`ea17e4b7a75ce2a00eed9345b5dfcf69b5e093edfc6ee9815047e05926d5eb69`.
Run/job metadata, artifact metadata and the successful gate log were inspected;
this is hosted validation, not a claim of independent local artifact replay.
The repaired candidate now has current Windows, macOS and Linux Full evidence.

Workspace [PR #43](https://github.com/OpenTLV/opentlv-workspace/pull/43) merged as
`df2b648bd691c0e970b1f8270934db992790ca42` and passed
[Main 40072116788](https://github.com/OpenTLV/opentlv-workspace/actions/runs/40072116788),
attempt 1. Main validates configuration/bootstrap and does not execute Full.
The merged manifest still selects Go `2a52389b7ee9dfbd895841a1840153d9cd8a41f4`
and Lua `c463066ca24f524d3ae9c7dff949796629367d03`. The local sibling portal
smoke on these pins failed because `lua/quality-gates.md` was missing.
[Workspace Full 40072566623](https://github.com/OpenTLV/opentlv-workspace/actions/runs/40072566623)
ran on that merged workspace revision, attempt 1. Remote integration passed,
but [sibling job 120808990315](https://github.com/OpenTLV/opentlv-workspace/actions/runs/40072566623/job/120808990315)
failed with `Portal navigation references missing documentation: lua/quality-gates.md`.
This confirms the local failure and does not satisfy cross-repository acceptance.
The prepared workspace correction selects Lua
`25d8783f1ef0def6c8790c7060a756fab635d3e4` (the portal's Lua source) and Go
`c6af85d2c11cfc6c98897398b2ec3b6e5335a491`. A successful complete Full on that
committed combination is the remaining migration validation prerequisite. The
corrected local sibling portal assembly/navigation passed on clean isolated
checkouts with all six sources; this verifies the documentation repair, not
hosted native integration.

### Local reproduction

Local runs use Linux amd64 under WSL/Ubuntu 24.04, GCC 13.3.0, CMake 3.28.3,
Ninja 1.11.1 and Python 3.12.3. Default/minimal use Go 1.22.2; shared uses
Go 1.27.3. Go implementation inputs are the reviewed `8c62688` tree, with only
this review's documentation edits during validation. These are local results,
not hosted Full or release evidence.

| Check | Result |
| --- | --- |
| Original and destination source hashes | All 55 entries verified; all 14 bridge files byte-identical |
| Python infrastructure | 41/41 passed |
| Go default static tests | 64 passed, no skipped top-level tests |
| Go minimal static tests | 38 passed, 26 existing feature-dependent skips |
| Go default shared tests | 64 passed, no skipped top-level tests |
| External module ZIP consumer | Passed in all three configurations, without workspace/replacement and with a fresh module cache |
| Race detector | Default and minimal static suites passed, including private bridge tests |
| Strict cgo pointer checks | `GOEXPERIMENT=cgocheck2` passed in default and minimal static suites |
| Complete local Full configurations | Default 18/18 steps, minimal 14/14, shared 16/16; all final records passed |
| Examples and Query conformance | Every planned example passed; default/static and shared each passed 171 fixtures and 2133 native executions, with no unavailable-provider fixtures and `--property-cases 0` |
| Workspace configuration | `scripts/integration.py fast` passed |
| Workspace Go sibling smoke | Existing `integration.go_smoke` passed build, quick-start output and dependency-receipt checks against Core `a9a6f94` |
| Diagnostic vocabulary | `--check` against the sibling Core passed |

The sibling smoke deliberately records `mode: sibling` and `dirty: true` for
the local Core checkout as seen by WSL, and `dirty: true` for Go's documentation
edits. It is development evidence, not a clean pinned-source CI receipt or the
whole-workspace Full gate. The standalone configurations separately use clean
Core clones at the original lock. No claim is made that every component in the
current workspace manifest was built together.

Run the three local configurations from a dedicated, unchanged Go checkout
with the indicated toolchains on `PATH`. Do not edit the checkout during a run:
the runner rejects changes to its Git identity or CI definitions at completion.

```sh
python3 -B scripts/ci.py build --tier full --configuration linux-default
python3 -B scripts/ci.py build --tier full --configuration linux-minimal
python3 -B scripts/ci.py build --tier full --configuration linux-shared
python3 -B scripts/generate_diagnostic_vocabulary.py --core-source ../opentlv --check
python3 -B ../opentlv-workspace/scripts/integration.py fast
```

Local logs and per-stage Core receipts are retained in the ignored
`build/issue7-validation/` directory. Static results come from a frozen local
copy of the reviewed Go implementation and documentation edits, on the Linux
filesystem; their records are copied to `isolated-evidence/`. The initial
static runs under `evidence/` began with a clean checkout, then correctly
failed the final identity guard after review documentation was added during
execution. Passing individual checks does not make those runs successful.
The isolated reruns retain an unchanged Git identity throughout execution.

The first shared attempt under
`evidence/linux-shared/` failed because a pre-existing Go installation lacked
standard-library source directories. The complete cached Go 1.27.3 archive was
unpacked into a separate temporary directory; the retry uses the same compiler,
native configuration and Go version. The failed record is retained separately
from `retry-evidence/linux-shared/`; it is not counted as a successful run.

The final records were checked for complete ordered stages, successful exit
codes, matching log SHA-256 values and valid per-stage pinned-Core receipts.
Their common Core input SHA-256 is
`243fc76f41aa82a8a6b37867283974c7eb6675dfe237780f8d33db94796849dc`.
These digests identify local evidence files, not release attestations:

| Record under `build/issue7-validation/` | SHA-256 |
| --- | --- |
| `isolated-evidence/linux-default/record.json` | `57fba1f64008ee5354406db7a538d50f366dbd5ec12e11a06c61d9bb32a74b0f` |
| `isolated-evidence/linux-minimal/record.json` | `eb9e35d859e9157765c7801b5ee8fa68dc4bb96f43e986d3f06f6ef1bab3b16e` |
| `retry-evidence/linux-shared/record.json` | `c768d7fc29b8d1ad12ef58bc55a779ef1e66c0a1e224596f84e2e28f4888d027` |

### Epic acceptance matrix

Each row corresponds to one acceptance criterion in Core #684. "Verified"
means the stated scope is supported by the source review or cited execution;
it does not convert local results into hosted or release approval.

| Criterion | Review result and evidence |
| --- | --- |
| Original baseline recorded | Verified: immutable source, baseline records and 55 original blob hashes |
| Independently maintainable Go repository | Verified: root module, Go-owned bridge/tests/helpers, standalone workflows and guides |
| Existing API and behavior preserved | Verified within reviewed coverage: direct source parity and documented module/version exceptions; local suites above |
| Existing tests and examples migrated | Verified: inventory, unchanged behavioral assertions and nine examples |
| Go-specific tests run standalone | Verified: Go Main and local Full configurations |
| Core tests remain Core-owned | Verified: source ownership and selected-Core Query corpus consumption |
| Standalone build reproducible | Verified for the pinned source/configurations: explicit dependency selection and per-stage receipts; no claim of bit-identical release binaries |
| Core identity and provenance verified | Verified for source builds: full Git commit plus input/native hashes; no attested binary consumption |
| Local workspace development supported | Verified by explicit Core override and the bounded Go smoke above; whole-workspace final acceptance remains pending |
| Fast CI passes | Verified: repair candidate PR 40071831858 passed; historical Go Main remains recorded separately |
| Full CI passes | Verified: Full 40072168513 on repair c6af85d passed all six native configurations and aggregate gate, attempt 1 |
| Independent module/versioning | Verified: canonical module, Go 1.22 minimum, `BindingVersion`, root tag policy and external-consumer package checks |
| Agreed layout | Verified: root facade/tests, private `internal/capi`, public examples and infrastructure tests |
| Docs ownership/publication integrated | Verified for ownership, hosted notification/receiver and locked portal build; publication of this final source set is not established by Main |
| Old Core implementation/integration removed | Verified in Core `a9a6f94`; original evidence remains immutable |
| No unrelated functionality or redesign | Verified: #7 adds migration documentation and the discovered CI checkout-hash repair; runtime sources and native ownership are unchanged |
| Cross-repository validation passes | Partial: Go/Core and Docs alignment verified; final workspace pins and hosted Full remain pending |
| Coordinated release readiness | Pending: final workspace integration evidence; repaired Go Full is verified and the existing independent Full gate is the technical release prerequisite, while publication authorization remains outside migration |

### Deviations and completion handoff

The earlier stage-specific sections remain historical records. Current
migration deviations are the module import/type identity change, additive
binding version, normalized test layout, standalone build/CI helpers, Go-owned
diagnostic renderer and independently pinned documentation. No change to cgo
architecture or native ownership is required. Additional standalone CI lanes
exercise documented platforms/linking modes; they do not add a new runtime
implementation. The original Linux default/minimal checks remain present.

To complete #7 and the epic, record the following evidence against the actual
reviewed candidate, preserving this review as a dated snapshot:

1. **Go owner:** completed candidate PR/Full validation at repair `c6af85d`
   as recorded above. The final documentation update records those immutable
   results; it does not claim they ran on a later documentation commit. Run
   normal PR checks for the final update and Main after merge. Executable,
   dependency or CI changes require fresh candidate Full evidence.
2. **Workspace owner:** review and advance the relevant integration pins to
   the agreed final component set, then run Full in both sibling and remote
   modes with Go enabled. Retain the resolved Go and Core identities and
   successful gate. Main and the pre-Go Full above cannot satisfy this step.
3. **Docs owner:** consume the merged Go review revision through the existing
   notification/pin workflow and validate the locked source set. Record a
   successful publication separately if claiming that source set is live.
4. **Release readiness:** use the independently callable Go Full quality gate
   as the required technical gate for the exact future release candidate.
   The module/version contract and external-consumer checks are already part
   of each configuration. A release owner must separately select the binding
   version, approve the pinned-source dependency mode and authorize a tag.
   A publishing workflow, signed binary attestations, public proxy registration
   and an actual release are outside this migration. Their absence does not
   itself prevent migration acceptance; CI receipts do not claim any of them.

Keeping the original Core source pin is intentional and does not block source
migration by itself. A newer Core pin, verified release-binary consumption,
public proxy/checksum-database availability and publishing the first module
release require separate review. Publishing is explicitly outside this epic;
recording readiness must not silently authorize a tag or publication.

## Post-migration generic Format and fields (#17)

Issue #21 adds standalone Definition registry and diagnostic location adapters,
public contract tests and executable examples. The reviewed completeness baseline
is Go-owned tooling; it does not change Core semantics, the dependency pin or
official feature defaults. New source entries and adapted documentation hashes
preserve every original migration identity.

The original Core commit, source paths and original SHA-256 values remain
unchanged. Adapted hashes record the bridge ownership and documentation changes
for generic field composition. New Go/C sources, public tests and the formats
example are explicitly listed under `standalone_files` with their hashes and
reasons; they are not presented as files imported from the migration baseline.
The verifier continues to require an inventory entry for every Go/C source.

Issue #20 adds standalone structured codecs, scalar constraints and wire Generator
adapters, tests and a generic generation example. Schema-aware numeric conversion
extends the existing value-codec adapter. Adapted hashes record those additions
and the bridge/documentation changes; original migration source identities and
hashes remain unchanged. All conversion, validation and wire generation semantics
continue to execute in the exact pinned Core.

Issue #19 adds standalone Document extensions, prepared structural schemas,
public contract tests and a schema example. Adapted hashes record the bridge,
Document comments and guide updates; original source hashes and the exact Core
pin remain unchanged. Schema semantic validation stays in Core. Go owns the
immutable copied graph, lifetime guards, diagnostic snapshots and report storage.

Go adds public configuration snapshots and native context lifetime adapters.
Core continues to own field validation, wire encoding, composition, packed table
checks, constructed classification and source-preservation semantics. The exact
Core pin and official feature selection are unchanged. The existing minimal
compatibility lane also exercises the generic APIs without Builtins/Document.

## Licensing and community normalization (#29)

The REUSE review preserves the root MIT attribution, existing source headers and
both generated diagnostic snapshots. Exact-path annotations cover documentation
and metadata, while the module retains its own license text and dependency
notices. Community policies and common templates are linked from `OpenTLV/.github`;
Go build, test, license and packaging instructions stay here.

Only the adapted hash for `docs/development/go.md` changes in the migration
inventory to record the contributor-check instructions. Original source paths,
original hashes, the Core pin and implementation bytes remain unchanged.
