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

The module still uses `github.com/marekcingel/OpenTLV/bindings/go` while source
migration is validated. Go #4 owns the canonical standalone module path,
independent package versioning and release tags. No public module release is
claimed. `Version()` continues to report Core, preserving baseline behavior.

[MIGRATION.md](MIGRATION.md) records the exact source baseline, imported-file
checksums and scoped adaptations. Core owns native semantics and shared Query
fixtures; Go owns its public API, bridge, tests and examples.

Licensed under the [MIT License](LICENSE).
