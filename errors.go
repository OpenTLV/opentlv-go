// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import (
	"fmt"
	"github.com/marekcingel/OpenTLV/bindings/go/internal/capi"
)

// Named errors support errors.Is, including through processing error wrappers.
// Their native codes match raw diagnostic fields, but are not needed for matching.
var (
	ErrBufferTooShort = StatusError{code: capi.BufferTooShort}
	ErrInvalidLength  = StatusError{code: capi.InvalidLength}
	ErrNullArg        = StatusError{code: capi.NullArg}
	ErrOutOfMemory    = StatusError{code: capi.OutOfMemory}
	ErrInvalidTag     = StatusError{code: capi.InvalidTag}
	ErrVisitor        = StatusError{code: capi.Visitor}
	ErrLimit          = StatusError{code: capi.Limit}
	ErrSchema         = StatusError{code: capi.Schema}
	ErrInvalidArg     = StatusError{code: capi.InvalidArg}
	ErrInvalidTagSize = StatusError{code: capi.InvalidTagSize}
	ErrOverflow       = StatusError{code: capi.Overflow}
	ErrInvalidValue   = StatusError{code: capi.InvalidValue}
	ErrUnsupported    = StatusError{code: capi.Unsupported}
	ErrInvalidSchema  = StatusError{code: capi.InvalidSchema}
	ErrNativeSize     = StatusError{code: capi.NativeSize}
	ErrNeedMoreData   = StatusError{code: capi.NeedMoreData}
	// ErrEnd is normal end of iteration, not a failure.
	ErrEnd = StatusError{code: capi.End}
	// ErrTruncated reports final input that ends inside an element.
	ErrTruncated = StatusError{code: capi.Truncated}
	// ErrSyntax reports text that violates the requested grammar.
	ErrSyntax = StatusError{code: capi.Syntax}
	// ErrInvalidState reports lifecycle or callback reentrancy misuse.
	ErrInvalidState = StatusError{code: capi.InvalidState}
	// ErrCallback reports a provider contract violation.
	ErrCallback = StatusError{code: capi.Callback}
)

// Is compares native error identity independently of diagnostic detail.
func (e StatusError) Is(target error) bool {
	switch t := target.(type) {
	case StatusError:
		return e.code == t.code
	case *StatusError:
		return t != nil && e.code == t.code
	}
	return false
}

// OptionalSize distinguishes absent native diagnostic information from zero.
type OptionalSize struct {
	Value   uint64
	Present bool
}

// DiagnosticContext is a copied native layer/key/value context entry.
type DiagnosticContext struct{ Layer, Key, Value string }

// LocationDomain identifies the coordinate space of failure evidence.
type LocationDomain int

const (
	LocationDomainUnknown LocationDomain = iota
	LocationInput
	LocationOutput
	LocationExpression
	LocationDefinition
	LocationValue
)

// LocationKind distinguishes absence from points, ranges and missing-content boundaries.
type LocationKind int

const (
	LocationUnknown LocationKind = iota
	LocationPoint
	LocationSpan
	LocationScopeEnd
	LocationInsertion
)

// Location is a snapshot of the primary evidence coordinates.
type Location struct {
	Domain     LocationDomain
	Kind       LocationKind
	Begin, End uint64
}

// Diagnostic owns a snapshot of C diagnostics. No input or native storage is
// borrowed. Optional fields retain native presence; absent detail is not inferred.
// Reader offsets are absolute in the input, including discarded windows.
// Writer offsets use the coordinate system of the native encoding operation.
// Operation retains the native value shared by Reader and Writer evidence; use
// ReaderPhase or WriterPhase for the typed category of the producing layer.
// Location.Kind zero means unknown; Offset projects the known Location.Begin.
type Diagnostic struct {
	Location                                                  Location
	Message                                                   string
	Severity                                                  Severity
	Operation                                                 int
	Offset                                                    uint64
	HasOffset                                                 bool
	Expected, Actual                                          string
	Tag, RawLength                                            []byte
	HasTag, HasRawLength                                      bool
	TagOffset, LengthOffset, ValueOffset                      OptionalSize
	DeclaredLength, Available, EnclosingEnd, Required, Length OptionalSize
	Contexts                                                  []DiagnosticContext
	Path                                                      [][]byte
	// HasPath distinguishes a tracked empty path from absent path evidence.
	HasPath bool
	// PathOmitted counts innermost scopes beyond the retained outermost Path.
	PathOmitted uint64
}

// ParseError describes a native Reader or Document parsing failure.
// Offset is meaningful only when HasOffset is true. Unwrap exposes StatusError.
type ParseError struct {
	Diagnostic
	status StatusError
}

// Error renders the canonical status and the source offset when supplied by C.
func (e *ParseError) Error() string {
	if e.HasOffset {
		return fmt.Sprintf("%s at offset %d", e.status.Error(), e.Offset)
	}
	return e.status.Error()
}

// Unwrap preserves errors.Is and errors.As compatibility with StatusError.
func (e *ParseError) Unwrap() error { return e.status }

// WriteError describes a native measurement, encoding or Tree Writer failure.
type WriteError struct {
	Diagnostic
	status StatusError
}

// Error returns the canonical native failure description.
func (e *WriteError) Error() string { return e.status.Error() }

// Unwrap preserves errors.Is and errors.As compatibility with StatusError.
func (e *WriteError) Unwrap() error { return e.status }

func publicDiagnostic(d capi.Diagnostic) Diagnostic {
	size := func(v capi.OptionalSize) OptionalSize { return OptionalSize{v.Value, v.Present} }
	result := Diagnostic{Location: Location{LocationDomain(d.Location.Domain), LocationKind(d.Location.Kind), d.Location.Begin, d.Location.End}, Message: d.Code.String(), Severity: Severity(d.Severity), Operation: d.Operation,
		Offset: d.Offset.Value, HasOffset: d.Offset.Present, Expected: d.Expected, Actual: d.Actual,
		Tag: d.Tag, RawLength: d.RawLength, HasTag: d.HasTag, HasRawLength: d.HasRawLength,
		TagOffset: size(d.TagOffset), LengthOffset: size(d.LengthOffset), ValueOffset: size(d.ValueOffset),
		DeclaredLength: size(d.DeclaredLength), Available: size(d.Available), EnclosingEnd: size(d.EnclosingEnd),
		Required: size(d.Required), Length: size(d.Length), Path: d.Path, HasPath: d.HasPath, PathOmitted: d.PathOmitted}
	for _, c := range d.Contexts {
		result.Contexts = append(result.Contexts, DiagnosticContext{c.Layer, c.Key, c.Value})
	}
	return result
}

func parseError(code capi.Code, d capi.Diagnostic, base uint64) error {
	// These fields are positions in the native input window, not byte counts.
	for _, offset := range []*capi.OptionalSize{&d.Offset, &d.TagOffset, &d.LengthOffset, &d.ValueOffset, &d.EnclosingEnd} {
		if offset.Present {
			if offset.Value > ^uint64(0)-base {
				*offset = capi.OptionalSize{}
			} else {
				offset.Value += base
			}
		}
	}
	if d.Location.Kind != 0 {
		if d.Location.Begin > d.Location.End || d.Location.End > ^uint64(0)-base {
			d.Location = capi.Location{}
			d.Offset = capi.OptionalSize{}
		} else {
			d.Location.Begin += base
			d.Location.End += base
		}
	}
	d.Code = code
	return &ParseError{Diagnostic: publicDiagnostic(d), status: StatusError{code: code}}
}

func writeError(code capi.Code, d capi.Diagnostic) error {
	d.Code = code
	return &WriteError{Diagnostic: publicDiagnostic(d), status: StatusError{code: code}}
}

// ReaderPhase types Operation for Reader evidence, including Query Reader causes.
func (d Diagnostic) ReaderPhase() ReaderOperation { return ReaderOperation(d.Operation) }

// WriterPhase types Operation for Writer evidence.
func (d Diagnostic) WriterPhase() WriterOperation { return WriterOperation(d.Operation) }

// Phase identifies the failed Reader operation.
func (e *ParseError) Phase() ReaderOperation { return e.ReaderPhase() }

// Phase identifies the failed Writer operation.
func (e *WriteError) Phase() WriterOperation { return e.WriterPhase() }

// String returns the canonical C coordinate-domain name.
func (v LocationDomain) String() string { return capi.LocationDomainName(int(v)) }

// String returns the canonical C location-anchor name.
func (v LocationKind) String() string { return capi.LocationKindName(int(v)) }

// String distinguishes unknown evidence from a known point at byte zero.
func (v Location) String() string {
	if v.Kind == LocationUnknown {
		return "unknown location"
	}
	return fmt.Sprintf("%s %s %d..%d", v.Domain, v.Kind, v.Begin, v.End)
}
