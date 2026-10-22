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
predates those newly prepared records. No standalone test results are claimed.

## Source transfer contract

Go #1 transfers all original public package files, `internal/capi`, tests and
nine examples from the pinned baseline. Preserve API behavior, native ownership,
error identity, existing test expectations and all source copyright/SPDX notices.
Record any path/import adjustment separately from unchanged imported blobs.
The three Go guides (binding README, Core user guide and development guide) are
Go-owned; portal integration follows in Go #6. The public C implementation,
shared Query corpus/oracle, diagnostic enum/name sources and native tests remain
Core-owned. Go may consume those shared inputs from a selected Core revision.

Core's original LICENSE and source headers credit **Marek Cingel, 2026**. The
initial standalone LICENSE credited **OpenTLV, 2026**. Both notices are retained
in this repository's LICENSE. The MIT permission notice must accompany imported
sources; repository separation does not replace original authorship attribution.

## Build and release boundary

The baseline requires cgo, a matching C compiler, native public/generated headers
and `-ltlv`. Static linking defines `TLV_STATIC_DEFINE`; shared linking requires
the runtime loader path. Reader, Writer, Query, Schema and Codec must be enabled.
Existing Core Go CI covers Linux amd64 default/minimal static builds; documented
Windows GCC/MinGW and shared-library modes have distinct validation coverage in
the Core baseline record. No pure-Go fallback is introduced.

Go #2 establishes pinned/verified Core integration; Go #3 owns Fast/Full CI;
Go #4 determines canonical standalone module path, independent versioning and tag
policy; Go #5 normalizes layout; Go #6 owns documentation integration. Workspace issue #40 integrates this checkout. Core #686 removes the old implementation after its
replacement is ready; Go #7 verifies final cross-repository parity.

This bootstrap changes repository metadata and provenance only. It does not move
sources, rename the module, change cgo behavior, create a release, or establish
standalone build/release readiness.
