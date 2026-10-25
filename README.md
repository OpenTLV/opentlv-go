# opentlv-go

Experimental Go bindings for OpenTLV, using the canonical C engine through a
private cgo bridge. Reader, Writer, Document, Query, diagnostics and codecs retain
the behavior of the recorded pre-extraction baseline.

## Build and run

Install Go 1.22+, Python 3.9+, Git, CMake, Ninja and a C compiler compatible with
cgo. From this repository, run:

```sh
python3 scripts/go.py -- build ./...
python3 scripts/go.py -- test -count=1 ./...
python3 scripts/go.py -- run ./examples/quick_start
```

The helper fetches the immutable Core source in `core-source.json`, builds its
native library and invokes Go with matching headers and library paths. It never
selects a sibling implicitly. This repository builds without the OpenTLV workspace.

For explicit local Core development:

```sh
python3 scripts/go.py --core-source ../opentlv --build-dir build/local -- test -count=1 ./...
```

Use `python` instead of `python3` on Windows. Windows requires GCC/MinGW compatible
with the Go target; an MSVC library cannot be used by this cgo setup. See the
[development guide](docs/development/go.md) for configurations, provenance,
examples and checks, the [user guide](docs/guides/go.md) for public API usage, and
[implementation details](docs/implementation.md) for native ownership boundaries.

Generic fields and composed, variable, escaped and packed Formats are described
in the [field composition guide](docs/guides/go.md#generic-fields-and-composition)
and demonstrated by `python3 scripts/go.py -- run ./examples/formats`.

Import `github.com/OpenTLV/opentlv-go` as package `opentlv`. The binding has its own
source version, `opentlv.BindingVersion` (`0.1.0-dev`); `opentlv.Version()` continues
to report the linked Core version. No public module release is claimed.
See [packaging and versioning](docs/packaging.md) for external consumption,
independent tags, native requirements and migration deviations.

[MIGRATION.md](MIGRATION.md) records the exact source baseline, imported-file
checksums and scoped adaptations. Core owns native semantics and shared Query
fixtures; Go owns its public API, bridge, tests and examples.

The [final migration review](MIGRATION.md#final-cross-repository-review-7)
maps every epic acceptance criterion to source and execution evidence, records
compatibility deviations, and records successful Go Full validation alongside
the remaining workspace integration and release-process handoff.

PR/Fast, Main and manually dispatched/reusable Full CI run Go-owned checks against
the pinned Core. See the [CI policy](docs/development/ci.md) for the matrix,
required quality gate, evidence and migration-specific limits.
The [Core API reference inventory](docs/development/ci.md#core-api-reference-inventory)
reports pinned symbols and known gaps separately from public API parity.

The [tree streaming guide](docs/guides/go.md#tree-streaming-and-visitors) covers
incremental tree cursors, resumable visitors, balanced event writes and exact
source preservation. Run `python3 scripts/go.py -- run ./examples/streaming` for
a generic event transformation with an output-capacity retry.

The [structural schema guide](docs/guides/go.md#structural-schemas) covers copied
recursive definitions, bounded findings and owned diagnostics. Run
`python3 scripts/go.py -- run ./examples/schema` for generic validation that also
works without Builtins or Document support.

The [generic codec and generation guide](docs/guides/go.md#generic-codecs-and-wire-generation)
covers Schema-aware numbers, scalar constraints, structured object callbacks and
reproducible wire cases with explicit memory bounds. Run
`python3 scripts/go.py -- run ./examples/generation`; it also works without
Builtins or Document support.

The central [Go documentation entry point](https://opentlv.github.io/opentlv-docs/latest/languages/go/)
selects an immutable Go documentation revision. Go owns usage, API comments,
build instructions and examples; the portal owns assembly and publication.
See [documentation updates](docs/development/ci.md#documentation-updates) for
Main notifications, version alignment and publication limits.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `*.go`, `go.mod` | Public `opentlv` package and canonical module identity |
| `*_test.go` | Public consumer tests (`opentlv_test`) and package-internal tests (`opentlv`) |
| `internal/capi/` | Private cgo bridge, native calls and bridge tests |
| `examples/<name>/` | Runnable `main` packages using the public API |
| `scripts/`, `tests/` | Build/CI/packaging helpers and their Python tests |
| `docs/` | Go-owned guides, implementation, packaging and CI policy |
| `.github/workflows/` | Standalone PR/Fast, Main and Full workflows |
| `migration/`, `MIGRATION.md` | Immutable source identities and documented adaptations |
| `build/` | Ignored generated output, isolated by its own module boundary |

Run commands from the repository root. Public tests run with `go test .`;
`go test ./...` also includes the private bridge and example packages. Use
`scripts/go.py --` before these commands to supply the selected native Core.
See [layout migration deviations](MIGRATION.md#standalone-layout-and-migration-deviations-5).

Licensed under the [MIT License](LICENSE).
