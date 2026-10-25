# Go binding development

The [user guide](../guides/go.md) describes public API behavior; the
[implementation guide](../implementation.md) preserves native ownership and cgo
contracts. All cgo, unsafe conversions and native handles stay in `internal/capi`.

## Standalone Core selection

Prerequisites: Go 1.22+, Python 3.9+, Git, CMake, Ninja and a C compiler supported
by cgo. The root contains the Go module; run commands from this repository.

```sh
python3 scripts/go.py -- build ./...
python3 scripts/go.py -- test -count=1 ./...
python3 scripts/go.py -- package-check
python3 scripts/go.py -- run ./examples/quick_start
```

`scripts/go.py` fetches exactly the full commit in `core-source.json` into its
dedicated build directory. It rejects dirty or changed managed checkouts instead
of falling back to another source, installed library or sibling. Reader, Writer,
Query, Schema and Codec are required. The default is a Release static build.
The Go command itself still links `-ltlv`; CMake is invoked only by this helper.

Successful commands write `build/go/core-selection.json`: source repository,
locked and effective commit, dirty state, input digest, configuration and executed
Go command. This proves source selection and local execution, not signed binary
provenance or release readiness. Failed commands remove stale success evidence.

The [packaging policy](../packaging.md) defines canonical imports, independent
binding versions, Core compatibility and external application setup. Use
`--working-dir /path/to/app` to run a Go command in an external module with this
checkout's selected Core. `package-check` validates a separate consumer of a
local module ZIP with a fresh cache and no replacement or workspace.

The helper disables ambient Go workspaces and persistent Go settings, replaces
external cgo flags with matching native include/library paths, and partitions its
Go cache by the selected Core inputs. Reuse a build directory only for the same
source path, compiler, feature configuration and target. Use a fresh directory
after a compiler/toolchain upgrade. Build outputs must stay outside Core.

## Local workspace development

The workspace is optional. To select a sibling Core explicitly:

```sh
python3 scripts/go.py --core-source ../opentlv --build-dir build/local -- build ./...
python3 scripts/go.py --core-source ../opentlv --build-dir build/local -- run ./examples/quick_start
```

The path is relative to the invoking directory. No environment variable or editor
setting silently activates it. The receipt reports `local-source`, actual commit
and dirty state; local edits are allowed for development but do not authorize
release publication. Native behavior remains owned by the selected Core checkout.

## Configurations and tests

```sh
python3 scripts/verify_migration.py
python3 -m unittest discover -s tests -p 'test_*.py' -v
gofmt -l *.go internal tests examples
python3 scripts/go.py -- vet ./...
python3 scripts/go.py -- test -race -count=1 ./...
GOEXPERIMENT=cgocheck2 python3 scripts/go.py -- test -count=1 ./...
python3 scripts/go.py --minimal --build-dir build/minimal -- test -count=1 ./...
python3 scripts/go.py --shared --build-dir build/shared -- test -count=1 ./...
```

Minimal disables ASN.1, Bluetooth, LLDP, DHCP, NFC and Document; mandatory generic
Reader/Writer/Query/Schema/Codec remain enabled. Shared builds omit
`TLV_STATIC_DEFINE` and select the matching runtime library directory. Race checks
need a supported target and toolchain (the baseline uses Linux amd64). They do not
replace native sanitizers. `GOEXPERIMENT=cgocheck2` checks the Go/C pointer boundary.

For Windows PowerShell, use a GCC/MinGW toolchain matching Go's target:

```powershell
$env:CC = 'gcc'
# Only for a 32-bit MinGW compiler:
$env:GOARCH = '386'
python scripts/go.py --build-dir build/windows-386 -- test -count=1 ./...
python scripts/go.py --build-dir build/windows-386 -- run ./examples/quick_start
```

A 64-bit compiler uses Go's `amd64` target instead. There is no MSVC or pure-Go
fallback. Direct `go build`/`go test` also work after manually setting cgo include,
library and runtime paths for a separately built Core. Such manual inputs are not
covered by the helper's selection receipt; clear Go's cache when replacing them.

## Examples and shared tests

Use `python3 scripts/go.py -- run ./examples/<name>` for `version`, `quick_start`,
`reader`, `writer`, `errors`, `codec`, `formats`, `streaming`, `schema`,
`generation`, `document` and `query`. Document and Query
examples require the default feature configuration.

The `query_conformance` example consumes Core-owned shared Query inputs:

```sh
python3 scripts/go.py -- build -o build/go-query-conformance ./examples/query_conformance
python3 build/go/core/tests/query/run.py --native build/go-query-conformance --property-cases 0
```

Keep the shared corpus and Python oracle in the selected Core source. The helper
does not copy native C sources, shared fixtures or release workflows into Go.
The [standalone CI policy](ci.md) defines PR/Fast, Main and Full coverage, test
ownership and required evidence. Migration-specific deviations are recorded in
[migration provenance](../../MIGRATION.md).

## Diagnostic vocabulary maintenance

The Go repository owns the generator and generated snapshots. Core owns the C
enums, name tables and their canonical reader. Select the Core checkout explicitly
from this repository root:

```sh
python3 scripts/generate_diagnostic_vocabulary.py --core-source ../opentlv --check
python3 scripts/generate_diagnostic_vocabulary.py --core-source ../opentlv
```

The first command detects missing or changed snapshots without writing; the
second refreshes `diagnostic_kind.go` and `diagnostic_vocabulary_test.go` here.
Neither command writes into Core, discovers a sibling implicitly or changes
`core-source.json`. Review generated changes with the selected Core revision and
run the binding tests against the same native library. The reader is available
in both the original Core lock and the Core checkout after extraction.
