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

`core-source.json` pins the same baseline Core for standalone source builds.
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
| Workspace Main and Go integration | [run 39853011741](https://github.com/OpenTLV/opentlv-workspace/actions/runs/39853011741) | Predates the Core cleanup candidate |
| Portal Main with standalone Go source selection | [run 39968006266](https://github.com/OpenTLV/opentlv-docs/actions/runs/39968006266), `b0c1adb` | Selects Go `a4d72386a4392f02b133d9fcf6703bab52c749b6`; does not validate this Core cleanup |

Local cleanup validation covers Core CTest, standalone Go default/minimal/shared
builds, external module consumption, examples and Query conformance. Those local
checks do not replace current-candidate hosted Core, Go Full or workspace checks.
