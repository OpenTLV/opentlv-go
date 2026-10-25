# Using OpenTLV from Go

Start with setup below, then run the [Go quick start](#quick-start)
for input, expected output and the public API. You can use this guide without
studying native C implementation contracts.

The experimental `opentlv` package exposes the canonical C engine through Go
types, slices and errors. Application code imports
`github.com/OpenTLV/opentlv-go`; cgo and native handles stay inside
the binding. See the [binding capability reference](https://github.com/OpenTLV/opentlv/blob/main/docs/concepts/bindings.md)
for current parity, and the [standalone quick-start source](../../examples/quick_start/main.go)
for a complete runnable program.

## Install and integrate

For external projects, see [module packaging](../packaging.md#external-consumption).
`opentlv.BindingVersion` identifies the binding source; `opentlv.Version()` reports
the linked Core library. The binding is currently unreleased.

Use Go 1.22 or newer, cgo enabled, a C compiler supported by cgo, and matching
OpenTLV headers and a compiled library. Build or install the C library separately;
`go build` neither invokes CMake nor compiles a second copy of the C sources.
Generated `tlv/config.h`, `tlv/export.h` and `tlv/version.h` must match the library.

From the standalone repository root, the build helper selects the exact Core
source commit in `core-source.json`, builds matching native inputs, and runs Go:

```sh
python3 scripts/go.py -- run ./examples/quick_start
```

See the [standalone build guide](../development/go.md) for local Core overrides,
static/shared linking and PowerShell commands. The helper leaves the public cgo
link contract unchanged. Direct Go commands remain usable with matching manually
supplied headers/library; the helper avoids stale native inputs with a dedicated
Go cache. No workspace repository or sibling checkout is required.

## Quick start

From the standalone repository root:

```sh
python3 scripts/go.py -- run ./examples/quick_start
```

Expected output:

```text
Hello, world!
```

The [complete program](../../examples/quick_start/main.go) imports
`github.com/OpenTLV/opentlv-go`, constructs a one-byte Tag/Length Format, writes
Tag `01` and the UTF-8 Value `Hello, world!`, then reads the encoded bytes.
It checks construction, write and Reader errors. Elements borrow the Writer
output, which remains unchanged until reading completes.

## API reference and runnable examples

This guide documents public contracts and limitations. For the selected checkout's
exported API comments, run `go doc .` or `go doc . NewReader` from the module root.
The [public package sources](../../opentlv.go) and tests belong to this repository;
Core's generated C/C++ reference is not the Go API reference.

Run examples with `python3 scripts/go.py -- run ./examples/<name>`:

| Example | Purpose |
| --- | --- |
| [version](../../examples/version/main.go) | Independent binding and linked Core identities |
| [quick_start](../../examples/quick_start/main.go) | Fixed-format write/read round trip |
| [formats](../../examples/formats/main.go) | Mixed field encodings, tag-only elements and packed headers |
| [reader](../../examples/reader/main.go) | Reader and borrowed input |
| [writer](../../examples/writer/main.go) | Writer output and sizing |
| [streaming](../../examples/streaming/main.go) | Incremental tree events and output-capacity retry |
| [errors](../../examples/errors/main.go) | Status and diagnostic handling |
| [codec](../../examples/codec/main.go) | Typed Value conversion |
| [document](../../examples/document/main.go) | Owned Document editing; requires Document support |
| [query](../../examples/query/main.go) | Document queries; requires Document support |

The [query_conformance adapter](../../examples/query_conformance/main.go) is a test
driver for Core's shared corpus, not an interactive tutorial. Its build and runner
commands are in [Examples and shared tests](../development/go.md#examples-and-shared-tests).

## Format and Element

`NewFixed(FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: BigEndian})` creates
an immutable generic Format. `Order` defaults to TLV and `LengthScope` to Value
length. Byte order is explicit. `Builtin(BER)` selects a compiled preset;
disabled presets return an error rather than selecting another Format.
Constructors return `(Format, error)`; zero and failed Formats are invalid.

`NewElement(tag, value)` borrows binary Tag and Value slices. `Tag()` preserves
byte identity and `Value()` does not interpret content. `Clone()` copies all
bytes, including available raw encoding. `Source()` exposes Header, Tag,
Length, Value and Trailer ranges relative to the element start, with presence
flags; `Offset()` is absolute in the input. Logical Elements have no source
encoding. Format-supplied identifiers are copied into Go storage.

### Generic fields and composition

`NewFixedIdentifier` and `NewVariableIdentifier` create immutable
`IdentifierEncoding` values. `NewFixedLength`, `NewVariableLength` and
`NewEscapedLength` create `LengthEncoding` values. Each validates its configuration
through Core and exposes `Read`, `Measure` and `Write`. Identifier reads borrow
input; count reads return `uint64`. Failed reads return no semantic result and
preserve Core's consumed-prefix detail. Failed field writes leave the destination
unchanged and return zero written. `Write(nil, ...)` is a capacity error; use
`Measure` to query width. Identifier writes reject overlapping slices.

`NewComposed(CompositionConfig{Identifier: identifier, Length: length})` combines
any of those identifier/count encodings. `Order` selects TLV or LTV, and
`LengthScope` selects Value or Tag-and-Value counting. `TagOnly` is an optional
table of identifiers that omit Length and require an empty Value. It requires
TLV/ValueLength framing and implies no skipping or termination. A fixed field
identifier width must be nonzero; use `NewFixed` for its existing absent-Tag case.

`NewVariable(VariableConfig)` configures both variable fields and optional
`IdentifierPolicy`, `LengthPolicy` and `ConstructedBit`. `NewEscaped(EscapedConfig)`
combines fixed identifiers with inline or escape-prefixed counts. These APIs
describe definite generic framing; protocol-specific and indefinite-length
semantics remain with existing Builtin presets.

`PackedField.Read` and `PackedField.Write` extract/insert unsigned bit fields in
initialized backing bytes. Writes preserve unrelated bits and reject overflow.
`NewPacked(PackedConfig)` composes nonoverlapping Tag/Length bit fields. Supply
`TagStorage` as a complete table: entry `i` is `i` in `TagSize` big-endian bytes.
For a two-bit Tag and one-byte canonical identifiers, this is `[]byte{0, 1, 2, 3}`.
Core checks layout and capacity at construction and checks the selected table
entry when decoding. Packed semantic tags are copied into Go storage; Values
and source bytes still borrow input. Tag-and-Value scope counts the Tag's wire
envelope rather than its canonical width.

Constructors snapshot all policies, tables and predicates, so later changes to
the input configuration do not affect the result. Encodings and Formats are
safe to share concurrently; each consumer still owns its cursor and output.
Composed native configuration is reclaimed automatically and needs no `Close`.
Document and Query consumers retain it for their full lifetime.

`Format.CanRead()` and `CanWrite()` inspect Core capabilities. `IsConstructed(tag)`
applies the configured predicate to canonical Tag bytes without validating their
wire representability. All return false for an invalid Format.
`ConstructedBit{ByteIndex: 0, Mask: 0x20, Value: 0x20}` recognizes Tags with that
bit set; supply it to variable, mixed or packed configuration for nested values.

`format.Preserve(element)` returns an independent copy of the original encoding,
including accepted nonminimal fields. It revalidates `element.Source().Bytes`
and checks semantic equality through Core. Use the originating Format and keep
source bytes unchanged; a logical Element without source returns `ErrInvalidArg`.
`Writer.Write` instead regenerates framing according to the Format's policy.

Run the [complete generic format example](../../examples/formats/main.go):

```sh
python3 scripts/go.py -- run ./examples/formats
```

Expected output:

```text
wire=0102476f tag=01 value=476f
wire=00 tag=00 value=
wire=82476f tag=02 value=476f
```

## Reader and input windows

Use `NewReader(data, format)` for complete input. `Next()` publishes one Element;
call `Element()` inside the loop and check `Err()` afterward. Clean EOF is not
an error. Retained Elements borrow their original input even after another
read. Keep it unchanged or clone the Element before reusing a receive buffer.

`NewIncrementalReader(data, format)` starts with non-final input. A false `Next()`
with `NeedsMoreData()` and nil `Err()` is a pause. Use
`SetInput(window, discard, final)` to replace the contiguous window, preserving
all undiscarded bytes and appending new input. Offsets stay absolute. An
incomplete final element is an error; final input cannot be reopened.
The caller assembles windows: the binding does not buffer an `io.Reader`.
Readers require no `Close`.

Runnable version: [reader/main.go](../../examples/reader/main.go).

## Writer and nested construction

`NewWriter(format)` grows owned output. `NewWriterBuffer(format, buffer)` uses
the slice's length as fixed capacity. `WriteElement(tag, value)` and
`Write(element)` regenerate framing through C; `Measure(tag, value)` obtains
exact sizing. `Bytes()` borrows the completed output prefix and `Size()` reports
its length. Do not mutate it while a Reader or retained Element uses it.

`Begin(tag)`, `Value(bytes)`, `WriteElement` and `End()` stage constructed
output through the C Tree Writer; `Finish()` checks that parents are closed.
The Format must recognize the parent as constructed. Generic Fixed cannot do
so; BER SEQUENCE can. Tags are copied at Begin. Open parents allocate Go staging
storage even with fixed root output; this construction does not stream bytes
to a sink. `Bytes()` excludes unfinished parents.

Errors preserve published output and logical parent state. A `CapacityError`
reports total `Required` and `Available` sizes. `SetBuffer` copies the completed
prefix into larger storage so the failed write or End can be retried. Writer
requires no `Close`.

Runnable version: [writer/main.go](../../examples/writer/main.go).

## Tree streaming and visitors

`NewTreeReader(input, format, nil)` borrows a final input window.
`NewIncrementalTreeReader` starts with non-final input. Both return `(*TreeReader,
error)` and need no `Close`. Choose one projection for the cursor's lifetime:

- `Next()` / `Item()` produces preorder `TreeItem` values with depth, absolute
  offset and constructed classification.
- `NextEvent()` / `Event()` produces balanced `TreeBegin`, `TreeElement` and
  `TreeEnd` events, including BEGIN/END for empty parents. END has no borrowed
  payload; its offset is the end of the parent's value.

Check `Err()` after a false pull. `NeedsMoreData()` identifies an incremental
pause. BEGIN requires the complete encoded parent; this is not an early-header
parser. `Consumed()` is the discardable prefix of the current window, while
`Offset()` includes prior discards. `AtEnd()` requires final input and closure of
all scopes. `SetInput(window, discard, final)` preserves the undiscarded suffix
unchanged; retained views still borrow their original storage. Clone them before
overwriting it. These consumption/end accessors are also available on `Reader`.

Nil `TreeReaderOptions` selects 64 frames, maximum depth 64 and an unlimited item
count. With explicit options, zero is literal: `FrameCapacity: 0` allows roots
and skipping, `MaxDepth: 0` permits roots only, and `MaxElements: 0` permits no
nodes. A frame shortage leaves descent pending: call `GrowFrames` and retry.
`SkipSubtree()` can recover from a descent limit without validating descendants;
event mode emits a skipped END. Limits and parse errors preserve the native
continuation; a retry repeats the failing operation unless the caller changes
input/storage or skips a pending subtree. Mixing projections fails without
advancing the cursor.

`Reader.Visit` and `TreeReader.Visit` take callbacks returning `(VisitAction,
error)`. `VisitContinue` advances; `VisitStop` returns successfully. A callback
error is returned unchanged. The current node has already been consumed, so
another Visit resumes without replay; a stopped constructed item still has
pending children. `ErrNeedMoreData` reports a resumable pause. A nil TreeReader
callback validates only; a nil Reader callback is invalid. Callbacks must not
advance or mutate their cursor; reentrant operations are rejected. Callback
side effects are never rolled back.

`Writer.WriteEvent(event)` consumes a balanced stream, copying BEGIN tags and
ignoring source metadata. Its source Value is ignored on BEGIN; subsequent
events build the new children. A skipped event cannot represent complete output
and is rejected. On failure, keep the current event for retry before advancing
the producer. `NewTreeWriter` and `NewTreeWriterBuffer` add explicit maximum depth
and node-count budgets (`TreeWriterOptions`); END does not consume that budget.
Nil options select depth 64 and unlimited nodes. They use the same Go staging
as Writer, including with fixed root output; frame/tag/scratch memory is not
bounded or allocation-free. `Finish` checks balance, and `SetBuffer` permits a
failed root write/close to be retried. No writer streams unfinished parents to
an I/O sink, and native Format restrictions remain in force.

`format.EncodeEvents(events, options)` and `EncodeTree(preorderItems, options)`
return staged bytes. `MeasureEvents` and `MeasureTree` return their exact size
by performing the same encoding, necessary for content-dependent formats. They
allocate complete output; when bytes are also needed, call Encode once and use
`len`. Preorder construction closes parents from depth transitions and retains
empty constructed nodes. Measurement and encoding errors return no partial result.

For exact framing, `writer.Preserve(element)` revalidates the original bytes
with the writer's Format and checks unchanged logical content through Core.
Use the originating Format. `CopyEncoded(bytes)` deliberately copies without
validation or conversion. Both work at roots and inside staged parents, copy
input before publication, support aliasing with `Bytes`, and preserve state on
failure. `Remaining()` returns `(bytes, true)` for fixed root capacity or
`(0, false)` for growable output; open parents are not subtracted.

Run the generic [streaming example](../../examples/streaming/main.go):

```sh
python3 scripts/go.py -- run ./examples/streaming
```

It demonstrates incomplete-parent input, balanced events and retrying END after
enlarging the output buffer. It runs in all CI tiers, including minimal builds.

## Document, edits and cleanup

`Parse(data, format)` copies input into the canonical owned mutable Document.
`ParseWithOptions` accepts explicit depth and element limits; zero is a literal
limit. A disabled `OPENTLV_DOCUMENT` returns an unsupported error.
After success, use `defer doc.Close()`. Close is nil-safe and idempotent; the
finalizer is a fallback. Copies of a Document share ownership and closing one
closes all. Nodes retain their Document.

`Elements()` returns roots; `Children()` and `Parent()` navigate Nodes.
`Tag()`, primitive `Value()` and `Element()` produce independent Go snapshots;
constructed Values are empty, so navigate children instead. `SetValue`, `Erase`
and `Insert` delegate mutation to C. Every successful edit invalidates all old
Node handles: reacquire them before further use. Failed edits preserve handles.
Foreign and stale Nodes are rejected before native access.

`Source()` copies the original whole input until the first successful edit or
Close. Nodes have no individual wire ranges. `Encode()` regenerates framing;
`EncodeAs(format)` selects a compatible destination without remapping Tags.
`writer.WriteDocument(doc)` writes in the Writer's Format, including inside an
open parent. Capacity failures preserve the Writer.

Runnable version: [document/main.go](../../examples/document/main.go).

For direct navigation, `doc.Find(parent, tag)` searches one scope (zero parent
means roots), and `node.Next()`/`NextSameTag()` navigate siblings. A missing match
is an invalid Node with no error; a stale or foreign parent is rejected.
`doc.EncodedSize()`/`EncodedSizeAs(format)` measure the whole tree, while
`node.Encode()`/`EncodeAs(format)` and `EncodedSize()`/`EncodedSizeAs(format)`
operate on exactly one subtree. Encoding returns owned bytes and never remaps tags.
An incompatible destination returns a `WriteError` with native status; the Core
Document encoder does not provide optional diagnostic detail.

With `DocumentOptions.RetainSourceLocations`, `node.SourceLocation()` returns
independent optional `Offset` and `HeaderSize` values from the original input.
They are coordinates, not ranges into the current encoding. Without retention
they are absent. After mutation, reacquire all Nodes: affected nodes and ancestors
lose their source coordinates, while unaffected nodes may retain old coordinates.
An already returned location or encoding snapshot remains valid after Close.

### Document result visitors

After `execution.EvaluateDocument(doc, context, capacity)`, use
`execution.VisitDocument(func(node Node) QueryVisit)` to visit remaining results.
`QueryStop` returns successfully and the next call resumes after that node.
`QueryVisitorError` returns `ErrVisitor`; an invalid action returns `ErrCallback`.
Both preserve native callback diagnostics and invalidate the execution until Reset.
Panics propagate after C returns and guards are restored, also requiring Reset.
Keep the returned `*QueryExecution`; do not copy its mutable value. The callback
cannot reenter the execution or edit/close the Document, including through copies
of the Document. Read-only Node inspection is permitted. Document access
still requires external serialization, and any edit outside a callback invalidates
the retained Query results. `doc.EncodedSize()` supplies the complete snapshot
capacity; passing a negative capacity to EvaluateDocument already measures it.

## Structural schemas

Structural validation operates directly on encoded input, independently of
Document and compiled Query assertions. Core checks framing, nesting, lengths,
occurrences, ordering, alternative groups and child membership; it does not decode
semantic Values. The existing `ValidateQueryBuffer`/`ValidateQueryDocument` APIs
continue to handle contextual boolean assertions with Query execution limits.

```go
schema, err := opentlv.PrepareStructure(&opentlv.StructureDefinition{
    Rules: []opentlv.StructureRule{{
        Entry: opentlv.SchemaEntry{
            Tag: []byte{1}, Name: "payload", MinLength: 2, MaxLength: 8,
            LengthMultiple: 2,
        },
        MinOccurs: 1, MaxOccurs: 1, Kind: opentlv.SchemaPrimitive,
    }},
})
if err != nil { panic(err) }
defer schema.Close()
err = schema.Validate(data, format, opentlv.DefaultSchemaLimits())
```

Prepare copies all tags, names, tables and groups into native-owned immutable
storage and checks the definition once. Shared and recursive `Children` references
are supported, up to 256 distinct tables. Do not mutate the input graph during
preparation; afterward it can be changed or discarded. Copies of the prepared
handle share ownership. Close and validation serialize access; closing one copy
closes all, and future validation returns `ErrInvalidState`. Garbage collection
is a fallback for deterministic Close. No Go pointers are retained by C.

Length and occurrence bounds are inclusive; **zero is literal** and `-1` means
unbounded only for `MaxLength` and `MaxOccurs`. `Endpoints` permits only the two
length bounds; `LengthMultiple` additionally requires divisibility. Individual
length checks use `SchemaEntry.ValidateLength`. `Children` requires
`SchemaConstructed`; a nil child schema leaves membership unrestricted while
wire nesting is still validated. `SchemaOrderSequence` orders matched siblings.
Nonzero rule `Group` references a `StructureGroup` in the same table; grouped
rules require `MinOccurs == 0`, and the group bounds the combined occurrences.
Names cannot contain NUL. Invalid definitions match `ErrInvalidSchema`;
unrepresentable Go arguments match `ErrInvalidArg`.

`SchemaLimits` explicitly bounds input depth and total elements. Zero limits are
literal; `DefaultSchemaLimits()` returns 64 and 65536. These are traversal bounds,
not an allocation-free guarantee or a bound on definition storage. Preparation
allocates the copied graph; collection allocates up to the requested report capacity.

`Validate` returns the first `*SchemaError`. `ValidateAll(data, format, limits,
policy, capacity)` returns a `SchemaReport` with total `Count` and at most
`capacity` stored `Findings`. Capacity zero counts without storing. Schema
violations return `ErrSchema` together with the report. Fatal wire/resource
errors return an empty report and `*SchemaError`. The unknown-tag policy either
follows each scope's `AllowUnknown`, allows everywhere, or rejects everywhere.
Finding order within one scope is not a public ordering guarantee.

`SchemaError` preserves common location/path metadata, the discriminated Schema,
Reader or Codec cause, and owned expected-versus-actual detail. Definition failures
identify `DefinitionTable` (breadth-first table index, root zero) and
`Detail.DefinitionIndex`; they have no wire location, and absent native fields
are not inferred. Diagnostics survive input mutation and schema Close. `errors.Is`
continues to match native statuses. Run the complete generic example with:

```sh
python3 scripts/go.py -- run ./examples/schema
```

## Query and typed Values

Compiled expressions use `CompileQuery(text, ProgramOptions)` and immutable
`QueryProgram` owners. `LoadQuery` validates same-release images using the
original compile options; `Info`, `Variables`, `Format`, `Explain` and `Image`
expose native metadata. Compile-time declarations use `QueryType`; runtime
values use `QueryValue` and `Bind` rather than text interpolation.

`Execution(limits, retained)` creates independent bounded execution state.
`SetInput(data, discard, final)`, `Next`, `Visit`, `Exists`, `Context`, `Pruning`
and `Reset` preserve canonical continuation and validation behavior.
`EvaluateDocument` and `NextDocument` return checked Nodes; `Result` exposes
scalars. Input and match bytes are copied and retained as required. Callback
panics invalidate an execution until reset; reentrant operations are rejected.
Call `Close` for deterministic cleanup. `ProgramError` retains Query diagnostics.
Its `Kind` is a `QueryErrorKind` with canonical `String()` names; use
`QueryErrorKindSyntax` or `QueryErrorKindState` instead of numeric literals.
Common `Diagnostic` evidence survives failures without a Reader cause; when
`HasReader` is true, the same `Diagnostic` also carries the Reader detail.
`Expression` is the related expression span, with `LocationUnknown` kind when
the primary `Location` already is that span, and `SourceOffset()` projects a
primary input location. `CodecDetail.Reported` is the raw provider result.
See [consumer tests](../../program_test.go).

`ProgramOptions.Providers` maps `QueryNum`, `QueryBCD`, `QueryText` and `QueryDate`
to `QueryProvider` values. Each owns a stable nonzero ID and a `Decode` callback
receiving copied input and optional `QueryMetadata`. Text outputs use explicit
`MaxResultBytes` scratch. Provider panics become codec diagnostics. Independent
executions may call a provider concurrently, so shared callback state must be
synchronized by its owner.

`Feed(QueryEvent)` and `Finish` support canonical events without a Reader; native
state rejects mixing input modes until reset. `EditDocument` applies a completed
selection with explicit target capacity and returns the applied count, including
on partial failure. Short target storage preserves the selection for retry.

`ValidateQueryBuffer` and `ValidateQueryDocument` accept `[]QueryRule` with context
selectors, Boolean assertions and diagnostic names. Pass
`DefaultQuerySchemaLimits()` or explicit resource limits. Validation retains
programs/providers. `QuerySchemaError.Schema` holds the failed assertion's `Kind`,
`Tag` and `Field`, and is nil for any other failure; its location, path and
expected text are read from `Failure`.
Document mutation and close are rejected while provider callbacks are active.

`doc.Query("6F/A5/50")` delegates path parsing and matching to C and returns all
matching Nodes in document order. Exact hexadecimal tags and direct-child `/`
steps are supported; wildcards, predicates and `//` are rejected. No matches
returns an empty slice. Edits and Close invalidate Query Nodes; previously
copied Value snapshots remain valid. Borrowed/resumable Query is not exposed.

`Codec[T]` delegates Value conversion, validation and measurement to C.
Available constructors include fixed-width integer codecs, `Int64Codec`,
`BytesCodec`, configured `NumberCodec`, `TextCodec`, `DigitsCodec`, `IPv4Codec`
and `IPv4ListCodec`. Decode returns owned Go values; Encode returns owned Value
bytes for a Writer or edit. Numbers explicitly select width and byte order;
decimal digit codecs preserve leading zeros. Codecs do not infer semantics from
an enclosing Tag. Zero Codecs are unsupported. Protocol-specific and
application-object Structure codecs remain unbound.

Runnable versions: [query/main.go](../../examples/query/main.go)
and [codec/main.go](../../examples/codec/main.go).

## Errors, diagnostics and concurrency

Use `errors.Is` with named statuses such as `ErrInvalidTag`; use `errors.As` for
`*ParseError`, `*WriteError`, `*QueryError`, `CapacityError` or `StatusError`.
Wrapping with `%w` preserves matching. Value conversion returns `*CodecError`
with an unwrapped common status, for example `ErrInvalidValue`; its `Detail`
retains the owned conversion evidence. Query providers return a shared error
or nil, and `ProgramError.CodecDetail` preserves native conversion detail.
EOF and incremental pauses remain flow control with nil `Err()`.

Diagnostics copy byte slices, text, context and paths into Go storage, surviving
input reuse or Document cleanup. Presence flags distinguish absent information
from zero or empty values. Only detail supplied by C is exposed: Document
encoding can return a status without wire detail, Query reports its text error
offset where available, and Value/Structure codecs preserve their native
conversion evidence in `CodecError.Detail`.
Runnable version: [errors/main.go](../../examples/errors/main.go).

Immutable Formats and Codecs can be shared concurrently. Readers, Writers,
Documents and their mutable buffers require separate instances or external
synchronization. Returned slices are mutable; synchronize their use as well.
C retains no Go pointer across calls. See the
[Go development guide](../development/go.md) for race and cgo-boundary checks.

## Generic codecs and wire generation

`SchemaNumberCodec(entry, config)` composes `SchemaEntry` byte-length rules with
the existing unsigned BE, LE or BCD conversion. Width zero chooses the shortest
schema-permitted width; a fixed width is never silently widened. The codec copies
the entry's tag and name, validates on use and returns owned `CodecError` evidence
for representation or Schema failures. `MaxLength: -1` means unrestricted, while
zero is a literal bound. Existing scalar, byte, text, digit and IPv4 codecs remain
available; `IPv4ListCodec` returns an ordinary owned slice for indexed access.

```go
entry := opentlv.SchemaEntry{Tag: []byte{1}, Name: "counter", MinLength: 2, MaxLength: 4}
number := opentlv.SchemaNumberCodec(entry, opentlv.NumberConfig{Encoding: opentlv.NumberBE})
wire, err := number.Encode(42) // 00 2a: shortest permitted width
if err != nil { return err }
value, err := number.Decode(wire)
```

`NewValueConstraint` snapshots inclusive signed bounds or an allowed-value set.
Call `Validate` on the decoded scalar; violations match `ErrSchema`, malformed
native definitions match `ErrInvalidSchema`. Optional `Names` parallels
`AllowedValues`; `Name(value)` returns `(name, found, error)`, preserving a named
empty string separately from an absent name. NUL names and mismatched name counts
match `ErrInvalidArg`. An empty allowed set rejects all values; the zero
`ValueConstraint` permits all values. Convert unsigned numbers to `int64` only
after checking they are representable. Length and occurrence rules remain
separate from scalar constraints.

`NewStructureCodec[T](format, schema, options, decode, encode)` maps an entire
sequence to an application object. A nil Schema validates framing and nesting;
a prepared Schema also validates its rules. Decode invokes Core before the Go
callback and gives the callback a private input copy that it may retain. Encode
calls the Go callback once and sends its complete bytes through Core's structure
encoder for validation and an owned output copy. Invalid output is never returned.
The callback must keep its returned bytes stable until Encode returns. A nil
callback disables its operation with `ErrUnsupported`.

Callbacks run outside C and native locks; application errors are returned
unchanged and decode errors return the zero object. Native failures wrap
`CodecError` with owned Reader/Schema evidence; encode locations refer to output.
Closing the retained Schema invalidates future native conversions, including
encode validation if a callback closes it. A successful decode callback may close
the Schema after validation without invalidating the returned Go object. Codec
copies share this lifetime, and concurrent use requires concurrency-safe callbacks.
Callback panics propagate normally. No Go callback is retained by C.

`DefaultStructureCodecOptions` permits depth 64, 65536 elements and a 16 MiB
sequence. Custom zero limits are literal. `MaxBytes` checks incoming or callback
output size before binding copies; callbacks must bound their own allocations.
Core performs traversal checks, with its pinned native depth ceiling. These are
owned Go operations, without a caller-owned native workspace or allocation-free
promise.

`NewGenerator(format, options)` snapshots the ordered candidate table and retains
Format. `Generate(caseIndex)` calls the canonical wire Generator and returns owned
bytes. Same `GeneratorVersion`, Format, candidates, options, seed and case index
give identical results independent of call order. Different inputs need not yield
distinct bytes. The zero Generator is invalid; valid instances need no Close and
can be shared concurrently.

```go
options := opentlv.DefaultGeneratorOptions()
options.Seed = 42
options.Candidates = []opentlv.GeneratorCandidate{
    {Tag: []byte{1}, MinValueSize: 0, MaxValueSize: 16},
}
generator, err := opentlv.NewGenerator(format, options)
if err != nil { return err }
wire, err := generator.Generate(7)
```

`MaxElements`, `MaxDepth`, `MaxValueSize` and `MaxCaseSize` constrain generation.
`MaxWorkspaceSize` explicitly bounds native scratch: Core requires
`(MaxDepth + 1) * MaxCaseSize` bytes, exposed by `WorkspaceSize()`. Output requires
another `MaxCaseSize` bytes, and candidate tables/tags require their own copied
storage; the scratch budget is not a total process-memory limit. Defaults use
depth 8, 64 elements, 256 Value bytes, 4096 case bytes and a 1 MiB scratch budget.
Zero seed, case index, depth and Value size are valid. Missing candidates,
negative bounds or zero element/case/workspace limits are invalid. Unsupported
depth, native overflow, insufficient scratch budget and an unproductive bounded
search preserve their corresponding errors; failures return no partial bytes.

Primitive Values contain arbitrary bytes, not Schema-valid or Codec-valid
content. Format alone identifies constructed tags. Candidates must be
independently composable: exclude terminators and identifiers requiring
message-level ordering. Nil tags are absent; nonnil empty slices are explicitly
present-empty identifiers. Run the generic example in every feature configuration:

```sh
python3 scripts/go.py -- run ./examples/generation
```

The example combines a counter object codec with a length Schema and scalar
constraint, then reproduces a generated wire case. Core owns algorithms and
protocol behavior; Go tests its input mapping, ownership, limits and errors.

## Definition registries and diagnostic locations

`NewDefinitionRegistry(entries)` snapshots an application-selected list of
`Definition{Tag, Name}` values. `registry.Find(tag)` uses Core's first exact
tag-byte match and returns `(Definition, bool)`. Tags may be empty; nil and empty
lookup tags are equal. Duplicate tags retain entry order. Names need not be
unique; empty means unnamed, and Go strings preserve embedded NUL bytes.
An unknown tag returns false without implying that it is structurally invalid.

```go
registry, err := opentlv.NewDefinitionRegistry([]opentlv.Definition{
    {Tag: []byte{1}, Name: "counter"},
})
if err != nil {
    return err
}
definition, found := registry.Find([]byte{1})
```

The registry is immutable and safe for concurrent lookups. Copies share ownership
and need no `Close`; native storage follows Go object lifetime. Construction copies
tag bytes, and every returned tag is independent. Do not mutate constructor or
lookup inputs concurrently with the call. The zero registry is empty. Lookup
does not select a Format, Codec or Schema, resolve a protocol context or convert
canonical tag bytes into a wire header.

`NewLocation(domain, kind, begin, end)` checks coordinates through Core. Invalid
domains, kinds, reversed spans, unequal point/boundary endpoints and coordinates
outside native `size_t` become unknown evidence. A known point or empty span at
zero remains distinct from unknown. Coordinates use `uint64` in Go; valid native
ranges therefore depend on the target architecture.

`location.Translate(origin)` returns a checked offset in the same domain.
`location.Resolve(origin, domain)` places Value-local evidence in its enclosing
coordinate space, for example:

```go
local := opentlv.NewLocation(opentlv.LocationValue, opentlv.LocationSpan, 0, 2)
wire := local.Resolve(4, opentlv.LocationInput) // input span 4..6
```

Both return new values. Overflow becomes unknown; Resolve leaves other domains
and invalid enclosing domains unchanged after checking the original coordinates.
Use the actual Value origin supplied by your input/source metadata. These helpers
do not change the error code. Assigning exported `Diagnostic` snapshot fields
does not change its error wrapper's identity or automatically update legacy
`Offset`/`HasOffset` projections. Diagnostic contexts and paths are owned Go data;
applications enrich/render snapshots with Go values, slices and formatting tools.
There is no native borrowed path-stack or exact native path-formatting API.

The executable `ExampleDefinitionRegistry_Find` and `ExampleLocation_Resolve`
examples run via `python3 scripts/go.py -- test -run Example ./...`.

Copy operations use existing Go APIs: `bytes.Clone(element.Value())` makes an
owned value copy, and `element.Clone()` also retains copied tag/source metadata.
`Writer.Measure` and `Writer.Write`/`WriteElement` measure and serialize canonical
framing through Core. `Writer.CopyEncoded` copies exact bytes without validation;
`Format.Preserve`/`Writer.Preserve` check original framing. Fixed Writer buffers
reject short capacity without publishing partial output. These are the reviewed
alternatives to native copy helpers, not additional raw-pointer APIs.

## Current limitations

There is no custom Format callback adapter, borrowed Query or resumable Document
Builder. Generic structured codecs use
application-supplied Go object mappings; protocol-specific Builtins codec parity
is outside this binding's generic API work.
Go's copied Node snapshots, owned diagnostics and explicit Close are intentional
ergonomic differences. The [reviewed generic contract baseline](../development/ci.md#generic-completeness-review-and-gate-21)
records supported families and these limitations against the exact Core pin;
zero unreviewed symbols does not promise every C storage or callback ABI. The
[binding matrix](https://github.com/OpenTLV/opentlv/blob/main/docs/concepts/bindings.md#capability-implementation-matrix)
distinguishes these from disabled native components.

## Next step

Use [the basic model](https://github.com/OpenTLV/opentlv/blob/main/docs/concepts/learning-model.md) to choose borrowed processing
or owned editing, then [processing choices](https://github.com/OpenTLV/opentlv/blob/main/docs/guides/processing.md) for your next task.
