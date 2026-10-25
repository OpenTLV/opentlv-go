# Go binding (experimental)

For public API setup and a first task, start with [the Go user guide](guides/go.md).
The component details below cover packaging, implementation or development.

This module establishes the Go binding architecture (#472) and internal native
bridge (#473). It exposes
`opentlv.Version()` to verify the connection to the canonical C library.
Public Format and Element APIs are available (#474), along with Reader (#475),
Writer (#476), owned Document (#477), and idiomatic errors with owned diagnostic
snapshots (#478), Document Query and typed generic Value codecs (#479).
Development checks, public consumer tests and CI cover these APIs (#480).

The module path is `github.com/OpenTLV/opentlv-go`; its package
name is `opentlv`. It requires Go 1.22 or newer, cgo and a compatible C compiler.
Run Go commands from the repository root, which contains `go.mod`.

## Package boundary

- The root `opentlv` package owns the public, idiomatic Go API.
- `internal/capi` exclusively owns cgo imports, C types, native calls and any
  unsafe pointer conversions. Its Go-facing signatures use Go types.
- Root `*_test.go` files in `package opentlv_test` exercise the public package as
  consumers and guard the native import boundary. Tests that require private
  facade state use `package opentlv`; bridge tests stay beside `internal/capi`.
- `tests/` contains Python tests for the build, CI and packaging helpers.
- `examples/` contains runnable consumer programs.

The internal bridge validates generic field and Format configuration and selects available native
presets (BER, indefinite BER, DER, CER, EMV, LLDP, Bluetooth LTV, DHCPv4 and NFC
Type 2). It delegates single-element reads, incremental-input status, encoded
size calculation and writes to the canonical C Reader/Writer. Native source
ranges and reader/writer diagnostics are projected into Go values. Disabled
presets return an unsupported status; the bridge contains no wire parser.

### Ownership and lifetime

Prepared structural schemas (#19) copy the complete definition graph into C
allocations before `tlv_schema_prepare`. Table identities are resolved iteratively,
including cycles/shared children. A shared owner serializes validation and Close;
the graph is freed once without recursive traversal. Both single and bounded
multi-finding validation use checked native APIs. Go copies every diagnostic before
report/input/graph storage can expire, translating native definition owner pointers
to stable table indices. The facade never implements structural validation rules.

Document subtree operations retain the checked owner during C access. Optional
source locations are scalar snapshots, and Encode returns owned output. Document
Query visitors use the native Document visitor through a scoped cgo handle and
guard execution reentry and Document mutation/Close while running the Go callback.
Panics return visitor failure to C before propagating; deferred guards restore
state and the native execution remains invalid until Reset.

- Byte slices are borrowed during synchronous bridge operations. Transient schema
  reports pin their input until diagnostic snapshots are copied and native report
  storage is freed; no Go input pointer remains in C after the API returns.
  `runtime.KeepAlive` covers conversion of native
  results after the call. Nil and empty slices are supported without indexing
  their first byte; a non-nil empty slice remains distinct where relevant.
- Reader/Writer calls build their descriptor and cursor on the C stack. Fixed
  contexts remain call-local. Composed/variable/escaped/packed Formats hold an
  immutable C-owned configuration with copied tables and policies. A shared Go
  owner finalizer releases its reference; C Document and Query program objects
  retain independent atomic references and release them on close or failed
  construction. Calls keep the Go owner alive through native-result projection.
  Format copies need no `Close` and may be used concurrently. Document copies
  input and retains its descriptor for its lifetime; use its `Close` for
  deterministic cleanup, with a finalizer only as fallback.
- Standalone field encodings are pointer-free value snapshots. The bridge binds
  their optional policy pointers only in C-local copies. Field callbacks in mixed
  compositions adapt context only and delegate validation and wire operations to
  Core. No Go callback or Go configuration pointer is retained by C.
- Parsed Values, source bytes and source-bound Tags borrow the original Go
  slice, without copying. Returned slices keep its backing array alive. The
  caller must keep those bytes unchanged while results are in use and avoid
  concurrent mutation during calls. Format-supplied Tags (such as LLDP's
  transformed identifier) are copied into Go memory, so no native pointer escapes.
- Diagnostic Tag/raw Length bytes and native text are copied into Go storage.
  Optional diagnostic fields retain their presence flags. The bridge preserves
  C status codes, including end of input and `NEED_MORE_DATA`; the public facade
  adapts them to the Go errors and Reader flow control described below.
- Writes borrow caller-provided output storage. Overlap with Tag or Value is
  rejected before entering C. Insufficient capacity reports the required size;
  other encoder failures retain the C buffer-modification contract.
- Source ranges remain relative to the returned element start. No temporary
  native Format pointer is retained in Source. The Reader retains its slice and
  position in Go and passes each unconsumed window to the bridge.

Document owns native allocations and provides explicit cleanup.

## User documentation

Start with the [Go user guide](guides/go.md) for normal application
usage and the [capability matrix](https://github.com/OpenTLV/opentlv/blob/cb3caed5b09706ce11ecfae839397fd4f99d0025/docs/concepts/bindings.md) for parity.
This guide records native integration and binding development details;
the [Go development guide](development/go.md) links the CI checks.

## Build and run

For the same `Hello, world!` write/read example as C, C++, Rust, Python and Lua,
see the [multi-language quick start](https://github.com/OpenTLV/opentlv/blob/cb3caed5b09706ce11ecfae839397fd4f99d0025/docs/getting-started/README.md#quick-start).
After configuring cgo below, run `go run ./examples/quick_start`.

Use the [standalone build guide](development/go.md) for pinned Core builds,
explicit local overrides, static/shared linking, tests and examples. The native
bridge still links `-ltlv`; it does not compile a second C implementation.

The canonical module is `github.com/OpenTLV/opentlv-go`. See the
[packaging policy](packaging.md) for independent binding versions, root release
tags and external consumption. No public Go release is claimed by this migration.

## Format and Element

`NewFixed(FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: BigEndian})`
validates immutable configuration through C. `Order` defaults to `TLV` and
`LengthScope` to `ValueLength`; byte order must be explicit. `Builtin(BER)`
selects a linked preset. Disabled or unknown presets return an error, with no
fallback. Both constructors return `(Format, error)`; failed construction and
zero-value Formats are invalid (`Valid()` is false). `StatusError` supports
`errors.As` and preserves the native status number through `Code()`.

`NewElement(tag, value)` borrows the supplied `[]byte` slices. `Tag()` and
`Value()` preserve binary identity without normalization or Value decoding.
Keep borrowed bytes unchanged while results are used. `Clone()` copies Tag,
Value and any raw encoding into independent Go storage, retaining metadata.
Returned slices remain mutable; callers must synchronize any mutation.

`Source()` describes raw encoding where available: `Bytes`, the element's
`Offset` in the enclosing input, and Header/Tag/Length/Value/Trailer ranges
relative to the element start. `Range.Present` distinguishes absent fields from
present empty fields. `FormatTag` identifies a semantic Tag supplied by the
Format rather than borrowed from a wire Tag range. Logical elements created by
`NewElement` have no source bytes or ranges and report offset zero. Reader
populates source metadata; Document preserves an owned whole-input snapshot.
Formats own no native allocations and require no cleanup.

## Reader

`NewReader(data, format)` borrows a complete buffer. Iterate with `Next()`,
retrieve the current view with `Element()`, and check `Err()` after the loop.
Clean EOF is not an error. Invalid Formats produce `StatusError`; native parsing
failures produce `*ParseError` wrapping `StatusError`. A failed read leaves the
cursor unchanged and clears the current element. `Offset()` reports the absolute cursor position; each element has its
own absolute source offset. Retained elements borrow their original input even
after the next read. Use `Clone()` before overwriting that storage. Readers need
no `Close` and must not be used concurrently.

```go
reader := opentlv.NewReader(data, format)
for reader.Next() {
    element := reader.Element()
    fmt.Printf("%X @ %d\n", element.Tag(), element.Offset())
}
if err := reader.Err(); err != nil {
    return err
}
```

`NewIncrementalReader(data, format)` starts with non-final input. When `Next()`
returns false, `NeedsMoreData()` distinguishes a temporary pause from EOF;
`Err()` remains nil during that pause. Call `SetInput(window, discard, final)`
to extend or relocate the contiguous window, optionally removing an already
consumed prefix. The new window must retain all old bytes after that prefix,
followed by any additional input. Prefix identity is a caller precondition.
Offsets remain absolute after discarding. Final input cannot be reopened or
extended; an incomplete final element reports a parse error. An invalid update
leaves the reader unchanged. Terminal parse errors cannot be resumed.

The binding does not buffer an `io.Reader` or assemble chunks automatically.
Callers own window assembly and must preserve storage borrowed by retained
elements. Native failures expose owned diagnostic snapshots through `*ParseError`.

Run the independent Fixed-format example with `go run ./examples/reader`.

## Writer

`NewWriter(format)` allocates output as needed. `NewWriterBuffer(format, buffer)`
uses the entire slice length as fixed output capacity, starting empty.
`WriteElement(tag, value)` writes logical content; `Write(element)` does the
same for an Element and regenerates its framing using the selected Format.
`Measure(tag, value)` delegates exact sizing, including content-dependent
framing, to C. `Size()` and `Bytes()` expose the completed root prefix.

```go
writer := opentlv.NewWriter(format)
if err := writer.WriteElement([]byte{1}, []byte{42}); err != nil {
    return err
}
data := writer.Bytes()
```

`Begin(tag)`, `Value(bytes)`, `WriteElement` and `End()` support staged
construction, including nested parents. Begin requires a constructed identifier
recognized by Format; generic Fixed and other primitive-only Formats reject it.
For example, with BER, Begin([]byte{0x30}) opens a SEQUENCE. Value copies raw
Value bytes without interpretation; WriteElement encodes a child. End closes
through the canonical C Tree Writer. Tags are copied on Begin. Finish checks
that every parent is closed; Bytes excludes all unfinished parents.

Open parents use allocated Go staging storage even with a caller-provided root
buffer. Construction is staged rather than streamed. No native pointer survives
a call, and Writer requires no Close. C owns all wire framing and validation.
Writer is not safe for concurrent use.

Errors leave published output and logical open-parent state unchanged.
A `CapacityError` reports total `Required` and `Available` root buffer sizes;
`errors.As` also obtains its native `StatusError`. Use `SetBuffer` with a
larger slice to copy the completed prefix and retry a failed WriteElement or End.
Input may alias previously returned Bytes because encoding uses independent
temporary storage. Bytes borrows output; keep that storage unchanged while in use.

Document serialization is available through `WriteDocument` (#477).
Run the preset-independent example with `go run ./examples/writer`.

### Tree streaming extensions

The [tree streaming guide](guides/go.md#tree-streaming-and-visitors) covers
`TreeReader`, node/event projections, window replacement, limits and resumable
visitors, plus `TreeWriter`, `Writer.WriteEvent`, source writes and exact tree
measurement. See [streaming/main.go](../examples/streaming/main.go) for a generic
example with incremental input and a fixed-buffer retry.

The private Tree Reader adapter stores only Core scalar continuation fields and
offset-only frames between calls. Each call reattaches input, frames and the
retained immutable Format to a local C cursor, invokes Core, and clears all
pointers before storing the next continuation. Borrowed results are projected
onto the original Go input; Format-owned tags and diagnostics are copied. There
are no retained Go pointers in C and no public Close requirement. Window
replacement does not relocate previously returned views.

Visitors execute Go callbacks after native pulls return. This preserves source
metadata, native parse diagnostics and the consumed-current-item rule, with
application errors returned unchanged. Mutating reentrancy is rejected and
callback panics unwind normally without leaving the visitor guard active.

Tree Writer uses existing Go-owned staged frames and copied tags. Event ordering
and traversal budgets are Go adapter checks; native diagnostic measurement,
encoding and parent closure remain authoritative for wire semantics. Temporary
encodings are committed only after success. This deliberately exposes neither
native scratch/tag-storage pointers nor an allocation-free bounded-memory
promise. Native diagnostic offsets keep the existing per-encoding-operation
coordinate system rather than predicting final offsets inside open parents.

## Document

`Parse(data, format)` copies complete input into the canonical C Document.
`ParseWithOptions` accepts explicit `MaxDepth` and `MaxElements` limits;
zero is a literal limit. Parse uses C defaults. Disabled `OPENTLV_DOCUMENT`
returns an unsupported status rather than a fallback.

Call `defer doc.Close()` after successful parsing. Close is idempotent and
nil-safe. A finalizer is a fallback for forgotten cleanup. Document copies
share ownership; closing one closes all. Document is not safe for concurrent use.
Nodes keep the owning Document alive.

`Elements()` returns roots in encoding order, `Element(index)` selects a root,
and `Count()` includes descendants. Nodes expose `Children()`, `Parent()`,
`Constructed()`, `Tag()`, `Value()` and an `Element()` content snapshot.
Tags and primitive Values are independent Go copies. Constructed Values are
empty; use Children. Invalid nodes return nil from Tag/Value/Children and an
error from Element; `Valid()` checks the handle.

`node.SetValue(bytes)` replaces primitive content or parses replacement
children through C. `node.Erase()` removes a subtree.
`doc.Insert(parent, before, element)` inserts or appends copied content;
zero Nodes select roots and append position. Foreign and stale nodes are
rejected before native access. Every successful edit invalidates all existing
node handles, including unaffected siblings. Reacquire handles after edits.
Failed edits preserve the tree and handles.

`Source()` returns a copy of the original complete input, available until the
first successful edit or Close. The C Document does not store node wire ranges;
node Element snapshots have no Source metadata. `Encode()` regenerates framing
in the original Format, including normalization of non-minimal/indefinite
input. `EncodeAs(format)` uses a compatible destination without remapping Tags.
`writer.WriteDocument(doc)` appends this encoding in the Writer's Format,
including inside an open staged parent. Capacity failures leave Writer unchanged.

Run `go run ./examples/document` with Document enabled. Resumable
Tree Reader/Document Builder integration remains follow-up work.

## Query and Value codecs

`ParseQuery("6F/A5/50")` owns a bounded V1 path independently of a Document.
`Count`, `Step` and `Format` expose its canonical C representation. Its
`Matcher` owns a stable path copy and accepts every preorder item through
`Feed(tag, depth)`. `Reset` starts a new traversal; `Rebind` accepts an equivalent
path while preserving continuation. Close matchers when finished.

`CompileQuery` and `LoadQuery` own extended Query programs; `Execution` chooses
explicit streaming or retained storage and runtime limits. `ProgramOptions`
accepts all existing `Format` values, including every `NewFixed` configuration,
typed variables, conversion providers, stable-ID semantic `QueryTagAdapter`
callbacks, and scoped `QueryResolver` callbacks. `QueryDefinitions` uses native
Definition ambiguity rules; `QueryEMV` uses the native EMV symbol dictionary.
Resolvers must return identical Tags during checked preparation and commit;
same-size changes are rejected. Provider callbacks must support concurrent
independent executions; callback panics return a native error after C returns.

Executions copy input and raw feeds into owned storage. `FeedEncoded` decodes one
encoded node through C to preserve Source-dependent metadata. `Finish` publishes
retained selections; `NextResultWithOrdinal` also returns the original preorder
ordinal. `EvaluateDocument` and `NextDocument` expose checked Document handles.
Equal Fixed configurations interoperate as Go values; incompatible configurations
are rejected. `ValidateQueryBuffer` and `ValidateQueryDocument` run bounded native
Query Schema rules and retain owned diagnostic snapshots on failure.

`doc.Query("6F/A5/50")` returns all matching `[]Node` in document order from
the current mutable C Document. Paths are exact hexadecimal tags separated by
single slashes. Leading slashes, `//`, wildcards and predicates are rejected by
the C Query parser. No matches returns an empty slice. Successful edits and
Close invalidate query nodes; their previously copied Value snapshots survive.
Query failures return `*QueryError`, support `errors.Is` for native statuses,
and retain the C parser's byte offset in the query text when provided. Document
query traversal supplies no additional diagnostic context.

Typed `Codec[T]` values delegate conversion, validation and encoding measurement
to C. Use `Uint8Codec`, `Uint16BECodec`, `Uint16LECodec`, `Uint32BECodec`,
`Uint32LECodec`, `Int64Codec`, `BytesCodec`, `NumberCodec`, `TextCodec`,
`DigitsCodec`, `IPv4Codec` and `IPv4ListCodec`. Configurations are copied and
validated on use by C. Decode produces owned Go values. Encode returns owned
Value bytes, which can be supplied to NewElement or Node.SetValue.

`NumberCodec` supports uint64 binary BE/LE and packed numeric BCD; `DigitsCodec`
preserves leading zeros and trailing F padding. `TextCodec` explicitly selects
printable ASCII or alphanumeric ASCII and optional fixed width/zero padding.
IPv4 representations use `[4]byte` and `[][4]byte` in network octet order.
The zero Codec is unsupported. Generic codecs remain usable with protocol
components and Document disabled.

Value conversion failures are `*CodecError` values wrapping a common status.
Use `errors.Is(err, opentlv.ErrInvalidValue)` or `errors.As` with `*CodecError`.
Its `Detail` owns the conversion diagnostic and available delegated cause.
Query providers return shared errors; nil denotes success.

`SchemaNumberCodec` adds canonical field-length composition, copying its entry
metadata before use. `ValueConstraint` snapshots scalar ranges/allowed sets and
uses Core to check definitions, validate decoded scalars and resolve display names.

`StructureCodec[T]` uses `tlv_structure_decode` before a Go decoder callback and
`tlv_structure_encode` after a Go encoder callback. Internal C callbacks only
acknowledge validated input or copy staged bytes; all framing/Schema behavior
remains in Core. Application callbacks run once outside C/locks. A retained
prepared Schema is locked only during native conversion and diagnostic copying;
Close is synchronized and invalidates subsequent operations. Byte spans whose
diagnostics may borrow tags remain pinned until diagnostics are copied. Output is
published only on success, and application callback errors are returned unchanged.

The immutable wire `Generator` copies candidates (including tag presence), keeps
the Format owner alive and calls `tlv_generator_workspace_size`/`tlv_generate`.
Each call owns independent output and scratch, checked against the configured
workspace budget before allocation. Temporary candidate tables contain only C
allocations and are released before returning. No generator cursor or random
state is retained between case indices. `GeneratorVersion` comes from the linked
header. These APIs work with Builtins and Document disabled.

Run `go run ./examples/codec`, and with Document enabled,
`go run ./examples/query` for querying followed by typed Value decoding.

## Errors and diagnostics

The immutable `DefinitionRegistry` owns copied Go entries and a C-allocated tag
table used by `tlv_definition_find`. Names stay as Go strings, preserving embedded
NUL bytes. Copies retain one shared finalizer owner; lookup keeps it alive across
the C call and copies result tags. No Go pointer is retained by C, no native entry
pointer escapes, and concurrent reads need no mutable cursor or lock.

`NewLocation`, `Location.Translate` and `Location.Resolve` use Core's checked
location helpers after rejecting enum/size narrowing. Values return by copy;
overflow clears optional evidence without changing the producing error's identity.
Owned diagnostic snapshots replace native borrowed context/path mutation APIs.

Diagnostic `Path` retains outermost enclosing tags, and `PathOmitted` counts
innermost scopes beyond the native path capacity.

Use `errors.Is(err, opentlv.ErrInvalidTag)` (or another named `Err` value)
for stable error matching. Use `errors.As` to obtain `*ParseError`,
`*WriteError`, the existing value `StatusError`, or `CapacityError`.
`StatusError.Code()` returns the native result for comparison with raw
diagnostic fields such as `ProgramError.Codec`; matching needs no numeric codes. Wrapping with `fmt.Errorf("operation: %w", err)` preserves
matching and structured detail.

Native parsing failures from Reader and `Parse`/`ParseWithOptions` return
`*ParseError`.
Measurement, element encoding and Tree Writer failures return `*WriteError`.
Document encoding also returns `*WriteError`, but the current C Document encoding
API supplies only a status, so its optional detail is absent. Local argument
checks and Document mutation failures may return `StatusError` directly.
Fixed output capacity failures retain the existing `CapacityError` and match
`ErrBufferTooShort`. Final input that ends inside an element matches
`ErrTruncated`. Clean Reader EOF and incremental `NEED_MORE_DATA` remain
flow control (`Err() == nil`), rather than terminal errors.

Diagnostics own copied byte slices, strings, native contexts (innermost first),
and any native path (outermost first). They remain valid after input mutation or
Document cleanup. `Message` uses the canonical C status description. `Expected`
and `Actual`, severity, operation, Tag and raw Length are projected from C;
Go does not infer missing context. `HasOffset`, `HasTag`, `HasRawLength` and
`OptionalSize.Present` distinguish absent fields from zero or empty values.
Reader offsets, field positions and enclosing end are absolute, including after
`SetInput` discards consumed bytes. Byte counts remain unchanged. Writer offsets
refer to the native encoding operation, including its staged parent workspace.
The facade copies only context supplied by C; Query and Codec error contracts
are described above.

```go
if err := reader.Err(); err != nil {
    if errors.Is(err, opentlv.ErrTruncated) {
        fmt.Println("incomplete final input")
    }
    var parseErr *opentlv.ParseError
    if errors.As(err, &parseErr) && parseErr.HasOffset {
        fmt.Printf("parse error at offset %d\n", parseErr.Offset)
    }
}
```

Run `go run ./examples/errors` for a complete example using a generic Fixed
Format; it also works with optional protocol presets and Document disabled.
