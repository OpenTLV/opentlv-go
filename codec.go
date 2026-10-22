// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// CodecError retains a shared OpenTLV status and conversion evidence.
type CodecError struct {
	Diagnostic
	Detail *CodecDetail
	status StatusError
}

// Error returns the canonical shared result description.
func (e *CodecError) Error() string { return e.status.Error() }

// Unwrap supports matching every common status with errors.Is.
func (e *CodecError) Unwrap() error { return e.status }

// Code returns the original shared C result.
func (e *CodecError) Code() int { return e.status.Code() }

// Codec is an immutable typed facade over a C value codec. Its zero value is
// unsupported. It is safe for concurrent use with independent input storage.
// Decode returns owned representations; Encode validates and measures in C.
type Codec[T any] struct {
	config capi.CodecConfig
	from   func(uint64, int64, []byte) T
	to     func(T) (uint64, int64, []byte)
}

// Decode interprets only Value bytes, without selecting a codec by tag.
func (c Codec[T]) Decode(data []byte) (T, error) {
	var zero T
	if c.from == nil {
		return zero, ErrUnsupported
	}
	n, s, b, code, detail := c.config.Decode(data)
	if code != 0 {
		return zero, codecError(code, detail)
	}
	return c.from(n, s, b), nil
}

// Encode returns an independent encoded Value, or the native conversion error.
func (c Codec[T]) Encode(value T) ([]byte, error) {
	if c.to == nil {
		return nil, ErrUnsupported
	}
	n, s, b := c.to(value)
	result, code, detail := c.config.Encode(n, s, b)
	if code != 0 {
		return nil, codecError(code, detail)
	}
	return result, nil
}
func unsignedCodec[T ~uint8 | ~uint16 | ~uint32 | ~uint64](config capi.CodecConfig) Codec[T] {
	return Codec[T]{config, func(n uint64, _ int64, _ []byte) T { return T(n) }, func(v T) (uint64, int64, []byte) { return uint64(v), 0, nil }}
}

// Uint8Codec converts exactly one byte.
func Uint8Codec() Codec[uint8] { return unsignedCodec[uint8](capi.CodecConfig{Kind: 1}) }

// Uint16BECodec converts exactly two big-endian bytes.
func Uint16BECodec() Codec[uint16] { return unsignedCodec[uint16](capi.CodecConfig{Kind: 2}) }

// Uint32BECodec converts exactly four big-endian bytes.
func Uint32BECodec() Codec[uint32] { return unsignedCodec[uint32](capi.CodecConfig{Kind: 3}) }

// Uint16LECodec converts exactly two little-endian bytes.
func Uint16LECodec() Codec[uint16] { return unsignedCodec[uint16](capi.CodecConfig{Kind: 4}) }

// Uint32LECodec converts exactly four little-endian bytes.
func Uint32LECodec() Codec[uint32] { return unsignedCodec[uint32](capi.CodecConfig{Kind: 5}) }

// Int64Codec converts minimal big-endian two's-complement bytes.
func Int64Codec() Codec[int64] {
	return Codec[int64]{capi.CodecConfig{Kind: 6}, func(_ uint64, s int64, _ []byte) int64 { return s }, func(v int64) (uint64, int64, []byte) { return 0, v, nil }}
}

// NumberEncoding selects a generic unsigned representation.
type NumberEncoding int

const (
	// NumberBE selects binary big endian.
	NumberBE NumberEncoding = iota
	// NumberLE selects binary little endian.
	NumberLE
	// NumberBCD selects packed decimal with leading zero padding.
	NumberBCD
)

// NumberConfig selects width (zero means minimal) and BCD precision (1..18).
// Binary configurations require Digits zero. C validates configuration on use.
type NumberConfig struct {
	Encoding      NumberEncoding
	Width, Digits int
}

// NumberCodec converts configured uint64 values through the C number codec.
func NumberCodec(c NumberConfig) Codec[uint64] {
	return unsignedCodec[uint64](capi.CodecConfig{Kind: 7, Encoding: int(c.Encoding), Width: c.Width, Digits: c.Digits})
}

// BytesCodec copies arbitrary bytes without semantic interpretation.
func BytesCodec() Codec[[]byte] {
	return Codec[[]byte]{capi.CodecConfig{Kind: 8}, func(_ uint64, _ int64, b []byte) []byte { return b }, func(v []byte) (uint64, int64, []byte) { return 0, 0, v }}
}

// TextAlphabet specifies accepted ASCII characters.
type TextAlphabet int

const (
	// ASCIIPrintable accepts 0x20..0x7E including spaces.
	ASCIIPrintable TextAlphabet = iota
	// ASCIIAlnum accepts only ASCII letters and decimal digits.
	ASCIIAlnum
)

// TextConfig selects a repertoire, optional fixed width and trailing zero padding.
// C validates configuration and rejects embedded zeros.
type TextConfig struct {
	Alphabet    TextAlphabet
	Width       int
	ZeroPadding bool
}

func stringCodec(c capi.CodecConfig) Codec[string] {
	return Codec[string]{c, func(_ uint64, _ int64, b []byte) string { return string(b) }, func(v string) (uint64, int64, []byte) { return 0, 0, []byte(v) }}
}

// TextCodec converts configured ASCII text through the C text codec.
func TextCodec(c TextConfig) Codec[string] {
	return stringCodec(capi.CodecConfig{Kind: 9, Encoding: int(c.Alphabet), Width: c.Width, Padding: c.ZeroPadding})
}

// DigitsCodec converts packed decimal digit strings, retaining leading zeros.
// Width zero selects minimal encoding; positive widths use trailing F padding.
func DigitsCodec(width int) Codec[string] {
	return stringCodec(capi.CodecConfig{Kind: 10, Width: width})
}

// IPv4Codec converts an address in network octet order without address policy.
func IPv4Codec() Codec[[4]byte] {
	return Codec[[4]byte]{capi.CodecConfig{Kind: 11}, func(_ uint64, _ int64, b []byte) [4]byte { return [4]byte(b) }, func(v [4]byte) (uint64, int64, []byte) { return 0, 0, v[:] }}
}

// IPv4ListCodec converts address lists, preserving order and duplicates.
func IPv4ListCodec() Codec[[][4]byte] {
	return Codec[[][4]byte]{capi.CodecConfig{Kind: 12}, func(_ uint64, _ int64, b []byte) [][4]byte {
		out := make([][4]byte, len(b)/4)
		for i := range out {
			copy(out[i][:], b[i*4:i*4+4])
		}
		return out
	}, func(v [][4]byte) (uint64, int64, []byte) {
		b := make([]byte, 0, len(v)*4)
		for _, ip := range v {
			b = append(b, ip[:]...)
		}
		return 0, 0, b
	}}
}

// CodecDetail owns conversion evidence in the shared result domain.
type CodecDetail struct {
	Diagnostic
	Operation      CodecOperation
	Reported       int
	Violation      CodecViolation
	Reason         CodecReason
	Cause          CodecCause
	Representation string
	Reader         *Diagnostic
	Schema         *CodecSchemaDetail
}

// CodecSchemaDetail owns delegated Schema details. DefinitionIndex identifies
// the native rule within its definition; native owner addresses are not exposed.
type CodecSchemaDetail struct {
	Kind                                                             SchemaIssue
	DefinitionKind                                                   SchemaDefinitionKind
	DefinitionIndex                                                  uint64
	Tag                                                              []byte
	Field                                                            string
	IsGroup, HasOccurs, HasLength, HasForm                           bool
	MinOccurs, MaxOccurs, Occurs, MinLength, MaxLength, ActualLength uint64
	ExpectedForm                                                     int
	ActualConstructed                                                bool
	LengthMultiple                                                   uint64
	LengthFlags                                                      uint32
}

func publicCodecDetail(d *capi.CodecDetail) *CodecDetail {
	if d == nil {
		return nil
	}
	result := &CodecDetail{Diagnostic: publicDiagnostic(d.Diagnostic), Operation: CodecOperation(d.Operation), Reported: d.Reported, Violation: CodecViolation(d.Violation), Reason: CodecReason(d.Reason), Cause: CodecCause(d.Cause), Representation: d.Representation}
	if d.Reader != nil {
		v := publicDiagnostic(*d.Reader)
		result.Reader = &v
	}
	if d.Schema != nil {
		v := CodecSchemaDetail{
			Kind:              SchemaIssue(d.Schema.Kind),
			DefinitionKind:    SchemaDefinitionKind(d.Schema.DefinitionKind),
			DefinitionIndex:   d.Schema.DefinitionIndex,
			Tag:               d.Schema.Tag,
			Field:             d.Schema.Field,
			IsGroup:           d.Schema.IsGroup,
			HasOccurs:         d.Schema.HasOccurs,
			HasLength:         d.Schema.HasLength,
			HasForm:           d.Schema.HasForm,
			MinOccurs:         d.Schema.MinOccurs,
			MaxOccurs:         d.Schema.MaxOccurs,
			Occurs:            d.Schema.Occurs,
			MinLength:         d.Schema.MinLength,
			MaxLength:         d.Schema.MaxLength,
			ActualLength:      d.Schema.ActualLength,
			ExpectedForm:      d.Schema.ExpectedForm,
			ActualConstructed: d.Schema.ActualConstructed,
			LengthMultiple:    d.Schema.LengthMultiple,
			LengthFlags:       d.Schema.LengthFlags,
		}
		result.Schema = &v
	}
	return result
}
func codecError(code int, d *capi.CodecDetail) *CodecError {
	result := &CodecError{status: StatusError{code: capi.Code(code)}, Detail: publicCodecDetail(d)}
	if result.Detail != nil {
		result.Diagnostic = result.Detail.Diagnostic
	}
	return result
}
