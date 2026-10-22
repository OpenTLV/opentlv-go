# Go continuous integration

Go owns public facade tests in `tests/*.go`, package tests and private bridge tests
in `internal/capi`. Examples use the public facade. Python tests cover build/CI
orchestration and evidence validation, not duplicate implementations of Go APIs.
Core owns native semantics, native tests and the shared Query corpus/oracle.

## Workflows and coverage

`go-pr.yml` runs Fast on every pull request to `main`, without path filters.
`go-main.yml` checks the merged commit on every push to `main`. `go-full.yml` is
manually dispatched or called by another workflow. All use `go-checks.yml` and
the same matrix/stage definitions in `scripts/ci.py`.

| Configuration | Go | PR/Fast | Main | Full |
| --- | --- | --- | --- | --- |
| Linux amd64, default static | 1.22.x | Yes | Yes | Yes |
| Linux amd64, minimal static | 1.22.x | No | No | Yes |
| Linux amd64, default static | stable | No | No | Yes |
| Linux amd64, default shared | stable | No | No | Yes |
| Windows amd64, default static, GCC/UCRT | stable | No | No | Yes |
| macOS arm64, default static, Clang | stable | No | No | Yes |

Every configuration checks migration hashes, Python infrastructure regressions,
`gofmt`, `go vet`, all Go tests with `-count=1`, and the version/quick-start examples.
Full adds `go test -race` and `GOEXPERIMENT=cgocheck2` on both original Linux static
configurations, and runs the reader/writer/errors/codec examples everywhere.
Default configurations also run document/query examples and shared Query
conformance through the Go executable against the selected Core corpus
(`--property-cases 0`, preserving baseline coverage). Minimal intentionally omits
document-dependent examples/conformance and retains existing feature-aware skips.
The Linux shared lane tests the runtime library path, including the separately
launched Query executable. Race detection is not a substitute for C sanitizers.

All tiers also validate Actions expressions and shell steps with actionlint.
Linux uses Ubuntu 24.04, Windows Server 2022 and macOS 15 runners. Native Python
and Go on Windows use the UCRT64 GCC installed by setup-msys2; no MSVC fallback is
selected. No sibling repository, preinstalled OpenTLV library or Go workspace is
implicitly consumed. The Go module has no `go.sum`; setup-go caching is disabled.
The build helper retains its native-input-specific Go cache within each job.
It creates a nested Go module boundary in `build/` (or an external build directory)
so `./...` cannot collect another configuration's generated cgo or toolchain files.

## Quality gate and evidence

Configure branch protection to require the PR workflow's **Go quality gate**
job (shown under the reusable `checks` job). Workflow files cannot configure
repository branch-protection settings. Full has the same named aggregate gate
in its own workflow; it does not run automatically on each PR.

The gate runs even when a dependency fails or is skipped. It requires successful
plan/build/workflow-validation jobs, exactly the expected matrix records, every
planned check in order, matching logs and clean pinned-Core receipts. Records must
match repository, checked-out/event SHA, workflow/ref, run ID, attempt, tier and
CI definition hashes. Duplicates, missing results, local results and mixed Core
inputs fail the gate. A failed build overwrites stale success evidence; the gate
removes any previous success summary before validation. Re-run **all jobs** after
a failure: evidence from an earlier attempt intentionally cannot satisfy a later
attempt's gate, even if that earlier matrix row passed.

Artifacts named `go-build-<tier>-<configuration>-<attempt>` contain `record.json`,
per-check logs and Query failure evidence when available. Each completed Go stage
retains its own Core receipt, rather than relying only on the helper's last
`core-selection.json`. Records include exact Go/compiler/CMake/Ninja/Python
versions, commands, durations, log hashes, configuration and Core input/native
digests. `go-quality-<tier>-<attempt>` contains the successful aggregate summary.
Setup failures may have no record; that missing evidence still fails the gate.
These are CI execution records, not signed release attestations.

## Local reproduction

Install the prerequisites from the [development guide](go.md), using a compiler
and Go installation that match the selected native platform. From the repository:

```sh
python3 -B scripts/ci.py matrix --tier full
python3 -B scripts/ci.py build --tier pr --configuration linux-default
python3 -B scripts/ci.py build --tier full --configuration linux-default
python3 -B scripts/ci.py build --tier full --configuration linux-minimal
python3 -B scripts/ci.py build --tier full --configuration linux-shared
python3 -B -m unittest discover -s tests -p 'test_*.py' -v
```

Use `python` on Windows with `--configuration windows-default`; macOS uses
`macos-default`. Minimum-version configurations require Go 1.22.x. For local
`stable` configurations, install current stable Go; the record captures the
installed version, while hosted setup-go resolves the alias. Use a new `--work`
directory after a compiler/toolchain change, as required by the build helper.
`--work` and `--evidence` select separate directories under this checkout's `build/`;
the runner appends the configuration ID. Local results may include uncommitted
changes, are marked `local`, and cannot satisfy the hosted gate.

Public API/bridge behavior and the original migration assertions are unchanged.
See [migration deviations](../../MIGRATION.md#standalone-ci-and-migration-deviations-3)
for source-build provenance limits and the remaining extraction work.
