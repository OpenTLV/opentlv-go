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
offset where available, and Value codecs supply no structured diagnostic.
Runnable version: [errors/main.go](../../examples/errors/main.go).

Immutable Formats and Codecs can be shared concurrently. Readers, Writers,
Documents and their mutable buffers require separate instances or external
synchronization. Returned slices are mutable; synchronize their use as well.
C retains no Go pointer across calls. See the
[Go development guide](../development/go.md) for race and cgo-boundary checks.

## Current limitations

There is no public pull Tree Reader, Visitor, Schema, generic Definition registry,
custom Format callback adapter, borrowed Query or resumable Document Builder.
Writer has staged nesting, not the full public event-based Tree Writer facade.
Go's copied Node snapshots, owned diagnostics and explicit Close are intentional
ergonomic differences; missing operations remain capability gaps. The
[binding matrix](https://github.com/OpenTLV/opentlv/blob/main/docs/concepts/bindings.md#capability-implementation-matrix)
distinguishes these from disabled native components.

## Next step

Use [the basic model](https://github.com/OpenTLV/opentlv/blob/main/docs/concepts/learning-model.md) to choose borrowed processing
or owned editing, then [processing choices](https://github.com/OpenTLV/opentlv/blob/main/docs/guides/processing.md) for your next task.
