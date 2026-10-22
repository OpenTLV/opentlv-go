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
The subsequent #4 module-path and version adaptations are recorded below.
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
