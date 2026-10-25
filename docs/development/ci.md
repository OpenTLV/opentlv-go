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
(`package-check`), the reviewed generic API completeness gate, and the version,
quick-start, generic format, streaming, schema and generation examples.
The package check runs
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

## Core API reference inventory

PR and Full run the independent, informational **Core API coverage** job. It
publishes `build/api-coverage.json` as `go-api-coverage-<tier>-<attempt>` and writes
the per-header report to the GitHub Step Summary. Missing references, stale ignore
patterns and report-job failures do not block the Go quality gate. Invalid input
or an unrecognized `TLV_API` declaration does fail the report job visibly; it does
not produce a successful report. Main omits this report, matching the Python
coverage workflow. No percentage threshold is enforced. Separately, every native
build now requires the [reviewed generic contract baseline](#generic-completeness-review-and-gate-21);
generic inventory drift, stale exclusions or pin changes fail that required stage.

`scripts/api_coverage.py` is adapted from Python's standard-library reporter at
revision `74d7ed756229ab11544d5eba38568ae6f2bfddb0`. Its binding-specific
configuration is [`ci/api-coverage.json`](../../ci/api-coverage.json). It reads
`TLV_API` function and data declarations from Git blobs under `tlv/include` at
the full immutable `core-source.json` commit, independent of Core's working tree.
Headers under `tlv/builtins/` are reported separately. Conditional declarations
are inventoried without preprocessing; static inline helpers, types and macros
are outside this metric. A referenced descriptor can call unreferenced callbacks.

Only production `internal/capi/*.go`, `*.c` and `*.h` files contribute references.
The scanner includes C preambles attached to `import "C"`, while excluding
ordinary comments, C/Go string literals (including Go raw strings), bridge tests,
examples, documentation and generated build output. It is a lexical inventory,
not a call graph: a declaration, identity comparison or conditional reference can
count without proving an operation is reachable through the public Go facade.
Public Go API and lifetime/error tests remain the evidence for usable behavior;
Core owns native semantics and its shared Query corpus.

Each exclusion has a reason in the configuration and the JSON/Markdown report.
The initial 44 exclusions covered Go storage primitives, reduced-output wrappers
whose richer diagnostic operations are used, and callbacks installed by bound
format/codec constructors. Subsequent reviews below account for the remaining
generic symbols. For example, #21 maps `tlv_copy_element` to canonical diagnostic
Writer serialization, not merely to copying a Go byte slice. Builtins are not
blanket ignored; their 291-symbol inventory is visible but parity is outside epic #15.

The PR report reads the previous lock at `github.event.pull_request.base.sha`
and inventories both exact Core commits. Added symbols include their current
reference/ignore status; removed symbols retain their former header and kind.
An unchanged pin is stated explicitly. This is a symbol-set diff, not a C ABI or
signature compatibility check. A standalone Full run has no implicit base.

From the repository root:

```sh
python3 -B scripts/api_coverage.py --output build/api-coverage.json
python3 -B scripts/api_coverage.py --base origin/main --output build/api-coverage.json
python3 -B scripts/api_coverage.py --core ../opentlv --base HEAD --output build/api-coverage.json
```

The default shallow-fetches the exact pin(s) into `build/api-coverage/core.git`.
`--core` explicitly selects an existing Git object store and never fetches into
or changes it; every requested commit must already exist there. Its checked-out
HEAD need not equal the lock. `--summary PATH` appends Markdown, `--base-commit SHA`
selects a previous Core revision directly, and `--commit SHA` is an explicit
exploratory override, not pinned-build evidence. Use `python` on Windows.

### Initial baseline

The pre-change implementation at Go `6edf392293eb039986f0ca2aaa6936f8bc51607e`
was inventoried before implementation against Core
`cb3caed5b09706ce11ecfae839397fd4f99d0025`. Before exclusions, generic Core has
107 references and 190 missing symbols out of 297. With the reviewed exclusions,
the initial report is **107/253 (42.3%) referenced**, 146 missing and 44 ignored.
Builtins has **12/291 (4.1%) referenced**, 279 missing and no exclusions. There
are no stale ignore patterns. These numbers describe source references, not the
percentage of supported Go functionality, and must be regenerated after changes.

| Existing public Go surface | Remaining review work |
| --- | --- |
| `NewFixed`, `Builtin`, immutable `Format` and slice-backed `Element` | General field/format composition and format inspection (completed in #17 below) |
| `Reader`, incremental input, `Writer` and staged constructed writes | Reader/Writer/tree streaming and visitor gaps (#18) |
| Owned `Document`, navigation, edits and encoding; parsed/compiled Query, matchers, execution and Query Schema validation | Remaining Document/Query/Schema operations (#19) |
| Typed scalar, bytes, text, digits, IPv4 and numeric codecs | Generic structure codecs and wire generation (#20) |
| Structured diagnostic snapshots and Go errors | Remaining generic API and final evidence/exclusion review (#21) |

The initial #16 reporting change added no public Go operations, changed no
ownership/error contracts, and did not modify migration baselines or Core build settings.
Official builds retain all supported Core components and Builtins; the existing
minimal Full lane is compatibility testing, not a reduced official distribution.

### Generic Format and field inventory (#17)

Before coding #17, Go `04bbbb8` was inventoried against the same exact Core pin.
It matched the baseline above: 107 referenced, 146 missing and 44 ignored generic
symbols, with no stale exclusions. Inspection included the public types and
callback/lifetime contracts excluded from the lexical `TLV_API` metric.

| Pinned header family | Before #17 | Public operation after #17 |
| --- | --- | --- |
| `field/fixed.h` | No field operations | `NewFixedIdentifier`, `NewFixedLength`; `Read`, `Measure`, `Write` |
| `field/variable.h` | No field operations or policies | `NewVariableIdentifier`, `NewVariableLength`; copied identifier/count policies |
| `field/escaped.h` | No escaped count operation | `NewEscapedLength`; same field operations |
| `field/packed.h` | No bit-field operation | `PackedField.Read`/`Write`, preserving unrelated bits |
| `formats/compose.h` | `NewFixed` only | `NewComposed`, mixed encodings, TLV/LTV, count scope, tag-only tables |
| `formats/escaped.h` | Missing | `NewEscaped`, copied tag-only tables |
| `formats/variable.h` | Missing | `NewVariable`, copied policies and constructed predicate |
| `formats/packed.h` | Missing | `NewPacked`, copied canonical Tag table and owned decoded Tags |
| `format.h` | Reader/Writer already decode/measure/encode | `CanRead`, `CanWrite`, `IsConstructed`, `Preserve` for exact original framing |

The updated generic report is **134/234 (57.3%) referenced**, 100 missing and
63 ignored out of 297 symbols. Builtins remains **12/291 (4.1%) referenced**,
279 missing and zero exclusions. All field/format header symbols are now either
referenced or justified exclusions; unrelated gaps remain visible for #18-#21.
These numbers remain source-reference evidence, not a functional coverage ratio.

The 19 additional exclusions cover constructor-installed composition callbacks,
the redundant tagged-binary path represented by fixed encodings in `NewComposed`,
and raw format measurement/encoding already served by diagnostic Writer wrappers.
`Format.Preserve` uses Core decode/source-preservation APIs to preserve original
framing. Builtins-specific expansion, arbitrary Go wire callbacks,
custom terminated-value resolvers and direct calls to low-level format callbacks
are intentionally outside this declarative API. No Core wire logic is reimplemented.

Public contract tests cover configuration snapshots, borrowing/copying, invalid
and narrowed inputs, field capacity failures, composition combinations, constructed
consumers, source preservation, GC and concurrency. Private bridge tests release
the Go owner's reference explicitly to verify independent native Document/Query
retention and failure-path cleanup. The generic formats example runs in every CI
tier, including the existing minimal compatibility lane.

### Reader, Writer and tree inventory (#18)

Before coding #18, Go `443080565786675cf17a65668fe42cd930a5a095` was inventoried
against the unchanged exact Core pin `cb3caed5b09706ce11ecfae839397fd4f99d0025`.
The baseline was 134 referenced, 100 missing and 63 ignored generic symbols.
Inspection distinguished existing sequential/incremental Reader, staged Writer
and `Format.Preserve` from tree operations only used internally by Document/Query.

| Pinned header | Before: referenced / missing / ignored | Public operation after #18 |
| --- | --- | --- |
| `reader/reader.h` | 5 / 6 / 2 | Existing source-aware Reader; add `Consumed`, `AtEnd` |
| `reader/tree.h` | 3 / 8 / 0 | `TreeReader`, node/event pulls, window replacement, frame growth, skipping, limits and absolute offsets |
| `reader/visitor.h` | 0 / 3 / 0 | Go `Visit` adapters with STOP/error/pause resumption and guarded reentrancy |
| `writer/writer.h` | 5 / 15 / 1 | Existing transactional Writer; add `CopyEncoded`, `Preserve`, `Remaining` |
| `writer/tree.h` | 4 / 8 / 2 | `WriteEvent`, budgeted `TreeWriter`, preorder/event encoding and exact staged measurement |

After #18, generic Core has **144/204 (70.6%) referenced**, **60 missing** and
**93 ignored** out of 297 symbols. Builtins remains **12/291 (4.1%) referenced**,
279 missing and no exclusions. No stale ignore patterns remain. Every symbol in
these Reader/Writer headers is now either referenced or intentionally excluded;
this does not assert allocation-free parity or full generic Core coverage.

The 30 new exclusions are reviewed semantic alternatives: reduced-output and
one-shot Reader wrappers; sequential cursor access already represented by Go
state; Go visitor callbacks over diagnostic pulls; duplicate Writer entry points;
and native Tree Writer callback/workspace APIs represented by Go-owned staging.
Writer events validate kind/depth/classification and dispatch to canonical
diagnostic encoding. Exact tree measurement encodes actual children, including
content-dependent framing. Complete output is allocated; native bounded scratch,
tag-storage configuration and callback replay are intentionally not public APIs.
`TreeWriterOptions` bounds depth/count, while fixed buffers bound completed roots.
These choices and callback errors are documented in the public guide.

Public tests exercise borrowed views and clones, Format lifetime across GC,
every input split, discard inside a parent, absolute diagnostics, frame-growth
and skip recovery, trees deeper than the default, visitor STOP/error/panic and
reentrancy, malformed event streams, exact source preservation, output aliasing,
fixed-buffer retries and native Format restrictions. Core retains wire behavior
tests. The generic streaming example runs in all tiers and configurations.

### Document, Query and Schema inventory (#19)

Before coding #19, Go `e22fbb9c1996744b756d4c27c224fb690ec6f221` was inventoried
against Core `cb3caed5b09706ce11ecfae839397fd4f99d0025`: 144 referenced, 60 missing
and 93 ignored generic symbols. The inventory included public reachability,
ownership, mutation guards, diagnostics and resource limits, not only references.

| Pinned header | Before: referenced / missing / ignored | Result of review |
| --- | --- | --- |
| `document/document.h` | 24 / 17 / 0 | Add direct Find/sibling navigation, source coordinates, document sizing and subtree encoding/sizing; existing edits and builder-backed Parse remain |
| `query/query.h` | 8 / 2 / 0 | Existing exact-path parsing/formatting/matcher plus compiled visitors cover selection; native callback variants intentionally excluded |
| `query/program.h` | 27 / 4 / 0 | Existing compilation/images, typed bindings, streaming/retained execution, scalar results, Document evaluation/editing; reduced wrappers/storage reset intentionally excluded |
| `query/adapters.h` | 2 / 1 / 0 | Existing Go providers and codecs cover conversion; native adapter callback intentionally excluded |
| `query/plan.h` | 0 / 2 / 0 | Existing copied LoadQuery/Info; borrowed raw instruction plans and frontend-disabled builds intentionally excluded |
| `schema/query.h` | 3 / 0 / 0 | Existing buffer/Document Query assertions, names, rule diagnostics and bounded workspace; no duplicate API added |
| `schema/schema.h` | 1 / 9 / 0 | Add prepared copied structural graphs, explicit traversal limits, first/all validation, unknown policy and bounded owned reports; length validation included |
| `schema/detail.h` | 2 / 0 / 0 | Preserve typed details, add definition table identity without exposing native pointers |
| `schema/constraint.h`, `schema/number.h` | 0 / 5 / 0 | Value constraints and schema-number conversion belong to the generic codec review (#20), not structural validation |

After #19, generic Core has **155/180 (86.1%) referenced**, **25 missing** and
**117 ignored** out of 297 symbols. Builtins remains **12/291 (4.1%) referenced**,
279 missing, no exclusions. There are no stale patterns. All symbols in the
Document, Query and structural-schema headers above are referenced or explicitly
excluded. This does not assert complete generic API parity: the remaining codec,
generator, copy/definition and diagnostic symbols remain visible for #20/#21.

The 24 new exclusions document equivalent Go operations or deliberate ownership
boundaries: default-format wrappers, empty Parse plus Insert, first-result path
selection, generation-checked Nodes, canonical snapshot sizing,
explicit-length compilation, owned plan loading, and once-prepared schemas.
`VisitDocument` adds native stop/resume, callback diagnostics and execution
invalidation after callback failure, with Go lifetime/reentrancy guards. Native
allocator callbacks, incremental Document-builder storage and borrowed plan/schema
memory are not promised. Preparation owns allocation; limits bound traversal and
report capacity bounds stored findings, not all process memory.

Binding tests cover graph copies/cycles/shared references, optional source
coordinates, subtree output ownership, stale/foreign Nodes, callback reentry and
panic unwind, diagnostic lifetime/discriminants, definition indices, bounded report
counts, zero limits, unknown policy, concurrent Close and minimal-build support.
Core remains responsible for semantic behavior suites. The schema example runs
in every CI tier/configuration; official feature selection and the pin are unchanged.

### Generic Codec and Generator review (#20)

Before coding #20, Go `af92a3dadd4c5064140893ada54c714fefbf772a` was inventoried
against unchanged Core `cb3caed5b09706ce11ecfae839397fd4f99d0025`: 155 referenced,
25 missing and 117 ignored generic symbols. Existing scalar, numeric, byte, text,
digits and IPv4 conversions already use canonical descriptors and wrappers.

| Pinned header | Before: referenced / missing / ignored | Result of review |
| --- | --- | --- |
| `codec/codec.h`, `codec/values.h` | 9 / 0 / 0 | Existing generic encode/decode and all scalar/bytes descriptors; no duplicate API |
| `codec/number.h`, `codec/text.h`, `codec/digits.h` | 0 / 0 / 6 | Existing constructors install canonical callbacks; direct callback symbols remain excluded |
| `codec/ipv4.h` | 2 / 1 / 0 | Existing owned `[4]byte` and `[][4]byte` results; native borrowed-list accessor excluded in favor of Go slice indexing |
| `codec/structure.h` | 0 / 2 / 0 | Add typed complete-sequence conversion with Core validation, optional prepared Schema, bounded copies and Go object callbacks |
| `schema/number.h` | 0 / 2 / 0 | Add copied Schema/number composition through its inline constructor; direct callback symbols explicitly excluded |
| `schema/constraint.h` | 0 / 3 / 0 | Add copied scalar definitions, validation and owned name lookup |
| `generator.h` | 0 / 2 / 0 | Add copied candidates, random-access seeded cases, native workspace sizing and an explicit scratch budget |

After #20, generic Core has **162/177 (91.5%) referenced**, **15 missing** and
**120 ignored** out of 297 symbols. Builtins remains **12/291 (4.1%) referenced**,
279 missing and no exclusions. No ignore is stale. All codec, numeric-schema,
scalar-constraint and generator declarations are referenced or justified in
`ci/api-coverage.json`. The remaining 15 generic symbols are copy (3), definition
(1) and common diagnostic helpers (11), outside this issue. Reference counts do
not claim complete public parity or Builtins-specific support.

Go retains object mapping callbacks, owned results and diagnostic snapshots;
Core owns all conversion/validation/generation algorithms. No native callback
ABI, borrowed object pointer or allocation-free workspace contract is exposed.
The generation example runs in every tier/configuration, including the existing
minimal compatibility lane. Official feature defaults and the Core pin remain
unchanged. Tests target copies, callback errors/reentry, shared Schema lifetime,
memory/traversal bounds, error publication and deterministic seed/index mapping.

### Generic completeness review and gate (#21)

Before coding #21, Go `68af66efd7c482a1a8bf9e46e4b5ab2d294449b1` was inventoried
against Core `cb3caed5b09706ce11ecfae839397fd4f99d0025`: 162 referenced,
15 missing and 120 excluded generic symbols. The 15 remaining symbols were
three copy functions, Definition lookup and eleven diagnostic/location helpers.

The review adds `DefinitionRegistry` and checked `NewLocation`, `Translate` and
`Resolve` operations, with owned results, concurrent immutable lookup, exact
error identity and executable public examples. Four symbols become referenced.
The remaining eleven have explicit reasons: Go value/exact copies, diagnostic
Writer serialization, owned diagnostic enrichment and producer-side borrowed
path helpers. Native path text rendering and allocation-free storage are not
promised. Existing constructor-installed callbacks and equivalent variants were
reviewed with the public contract families, including ownership and limitations.

The final inventory is **166 referenced, zero missing and 131 excluded** generic
symbols across **43 headers** (297 total). Builtins remains 12 referenced and
279 missing with no exclusions. These counts are an inventory, not a functional
coverage percentage or full C API parity claim. Official builds retain all
supported Core/Builtins; the minimal lane remains a compatibility configuration.

[`ci/api-completeness.json`](../../ci/api-completeness.json) records the exact
pin, every generic header's symbol disposition, exclusion reasons and eleven
reviewed public contract families. Each family describes its public surface and
limitations and points to named `opentlv_test` tests. It includes non-symbol
contracts such as Go ownership, callbacks and native-size conversion.

Every PR, Main and Full native build runs `scripts/api_completeness.py` after
the runtime tests/package check, using that build's exact Core Git object store.
It fails on changed pins (even with identical symbol sets), missing generic
symbols, reference/exclusion drift, stale patterns, incomplete header mapping
or missing/public-to-private test anchors. The baseline is not refreshed by CI.
This is a reviewed scope and regression gate: test existence cannot establish
test adequacy, and source references cannot establish reachability. Human review
of the mapped contracts plus the separately required runtime tests is essential.
Full adds race, cgocheck2, shared/native platform and Query conformance evidence.

```sh
python3 -B scripts/api_completeness.py --core ../opentlv --output build/api-completeness.json
```

For a Core pin update, first generate the informational report with `--base`
against the previous binding revision. Review changed declarations/signatures,
callbacks, ownership and inline/type/macro contracts at both exact revisions;
the lexical inventory cannot detect semantic changes. Implement meaningful gaps
or document narrow alternatives, update public tests and the family limitations,
then explicitly update the baseline and per-header table in the same reviewed
change. Run the required tests and Full configurations; never refresh the
baseline solely to restore a score. `api-completeness.json` and the per-header
log accompany each build artifact. A failed gate removes stale success output.
Local results cannot substitute for hosted quality or release evidence.

The current generic per-header counts follow. JSON and Step Summary list every
missing symbol as well as the separate Builtins headers. Original baseline totals
above remain historical; regenerate this table when implementation changes.

| Header | Referenced | Missing | Ignored |
| --- | ---: | ---: | ---: |
| `tlv/codec/codec.h` | 2 | 0 | 0 |
| `tlv/codec/diagnostic.h` | 5 | 0 | 0 |
| `tlv/codec/digits.h` | 0 | 0 | 2 |
| `tlv/codec/ipv4.h` | 2 | 0 | 1 |
| `tlv/codec/number.h` | 0 | 0 | 2 |
| `tlv/codec/structure.h` | 2 | 0 | 0 |
| `tlv/codec/text.h` | 0 | 0 | 2 |
| `tlv/codec/values.h` | 7 | 0 | 0 |
| `tlv/copy.h` | 0 | 0 | 3 |
| `tlv/definition.h` | 1 | 0 | 0 |
| `tlv/diagnostic.h` | 6 | 0 | 8 |
| `tlv/document/document.h` | 30 | 0 | 11 |
| `tlv/endian.h` | 0 | 0 | 15 |
| `tlv/field/escaped.h` | 2 | 0 | 0 |
| `tlv/field/fixed.h` | 4 | 0 | 0 |
| `tlv/field/packed.h` | 2 | 0 | 0 |
| `tlv/field/variable.h` | 4 | 0 | 0 |
| `tlv/format.h` | 6 | 0 | 2 |
| `tlv/formats/compose.h` | 3 | 0 | 12 |
| `tlv/formats/escaped.h` | 1 | 0 | 3 |
| `tlv/formats/fixed.h` | 1 | 0 | 0 |
| `tlv/formats/packed.h` | 4 | 0 | 0 |
| `tlv/formats/variable.h` | 2 | 0 | 4 |
| `tlv/generator.h` | 2 | 0 | 0 |
| `tlv/query/adapters.h` | 2 | 0 | 1 |
| `tlv/query/plan.h` | 0 | 0 | 2 |
| `tlv/query/program.h` | 27 | 0 | 4 |
| `tlv/query/query.h` | 8 | 0 | 2 |
| `tlv/reader/diagnostic.h` | 1 | 0 | 0 |
| `tlv/reader/reader.h` | 5 | 0 | 8 |
| `tlv/reader/tree.h` | 9 | 0 | 2 |
| `tlv/reader/visitor.h` | 0 | 0 | 3 |
| `tlv/result.h` | 1 | 0 | 0 |
| `tlv/schema/constraint.h` | 3 | 0 | 0 |
| `tlv/schema/detail.h` | 2 | 0 | 0 |
| `tlv/schema/number.h` | 0 | 0 | 2 |
| `tlv/schema/query.h` | 3 | 0 | 0 |
| `tlv/schema/schema.h` | 6 | 0 | 4 |
| `tlv/size.h` | 0 | 0 | 4 |
| `tlv/tag.h` | 0 | 0 | 5 |
| `tlv/value.h` | 0 | 0 | 7 |
| `tlv/writer/tree.h` | 4 | 0 | 10 |
| `tlv/writer/writer.h` | 9 | 0 | 12 |

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

Original migration source identities are preserved. #17 adds generic APIs and
records adapted bridge hashes plus explicit standalone source entries.
See [migration deviations](../../MIGRATION.md#standalone-ci-and-migration-deviations-3)
for source-build provenance limits and stage-specific adaptations.

The [final cross-repository review](../../MIGRATION.md#final-cross-repository-review-7)
records the reviewed revisions, hosted attempts, source parity and epic acceptance
matrix. Historical Full results, local checks and Workspace Main are not evidence
of final hosted Go/workspace Full or release authorization.
