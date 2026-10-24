# Go continuous integration

Go owns public facade tests in root `*_test.go` files (`package opentlv_test`),
package-internal tests (`package opentlv`) and private bridge tests in
`internal/capi`. Examples use the public facade. Python tests in `tests/` cover build/CI
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
`gofmt`, `go vet`, all Go tests with `-count=1`, external module ZIP consumption
(`package-check`), and the version/quick-start examples. The package check runs
without `replace` or workspace access using a fresh module cache and local file
proxy; it validates module/version/license metadata and a native round trip.
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

## Documentation updates

After successful Main checks, `go-main.yml` records whether documentation,
examples or their version/dependency metadata changed since the previous
successful Main push. Failed or cancelled intervening pushes remain included.
The `docs-notification-<attempt>` artifact identifies the exact commit and run.

`docs-notify.yml` runs only after successful official Main pushes. It reads that
attempt's JSON without checking out or executing producer code, validates its
identity and sends `component-docs-updated` to `OpenTLV/opentlv-docs`. Configure
`OPENTLV_DOCS_DISPATCH_TOKEN` with access to dispatch to that repository; the
credential is available only to the final notification step. No token is needed
when there are no relevant changes. A missing token or failed dispatch fails the
notifier and can be retried without rerunning native tests.

The portal's documentation update policy validates the successful Main run and
opens or updates a reviewed pin proposal on `automation/component-go`. Replays
are idempotent; stale or divergent revisions are rejected. A notification does
not deploy Pages, invoke Docs Full, or establish a Go release.

The portal mounts this repository's `docs/` independently of Core and preserves
Go-owned source/edit links and example links at the selected Go commit. Its
version selector describes the portal snapshot, not `BindingVersion` or the
linked Core version. `docs-examples.json` supplies the Go quick-start tab from
the same checkout; its version must match `BindingVersion`. No runtime
compatibility result for the portal-selected Core is implied by that catalog.

Go currently has no release workflow or independent versioned documentation
snapshot. Merge Go changes first, then review and merge the exact Go SHA into
the portal manifest and validate the locked build. Local preview permits dirty
sources but cannot authorize publication. Hosted notification, portal CI,
Pages deployment and published URL checks remain separate evidence.

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
for source-build provenance limits and stage-specific adaptations.

The [final cross-repository review](../../MIGRATION.md#final-cross-repository-review-7)
records the reviewed revisions, hosted attempts, source parity and epic acceptance
matrix. Historical Full results, local checks and Workspace Main are not evidence
of final hosted Go/workspace Full or release authorization.
