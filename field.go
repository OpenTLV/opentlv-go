// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// IdentifierPolicy constrains variable identifier bytes. Constructors copy it
// and ForbiddenLeading; subsequent mutations cannot change an encoding.
type IdentifierPolicy struct {
	ForbiddenLeading                       []byte
	RejectZeroFirstPayload, RequireMinimal bool
}

// VariableIdentifierConfig describes inline bits and escaped continuation octets.
// Identifiers retain their complete wire byte identity, including prefix bits.
type VariableIdentifierConfig struct {
	InlineMask, Escape, ContinuationBit, PayloadMask byte
	MaxSize                                          int
	Policy                                           *IdentifierPolicy
}

// LengthPolicy constrains definite counts. At least one form must be allowed.
// MaxValue is inclusive; zero allows only zero. Constructors copy the policy.
type LengthPolicy struct {
	AllowShort, AllowLong, RequireMinimal bool
	MaxLongOctets                         int
	MaxValue                              uint64
}

// VariableLengthConfig describes a short count or a prefix giving its octet count.
// A nil Policy permits Core's default definite forms, including padded reads.
type VariableLengthConfig struct {
	LongFormBit, PayloadMask byte
	ByteOrder                ByteOrder
	Policy                   *LengthPolicy
}

// EscapedLengthConfig describes a one-byte count below Escape or Escape followed
// by ExtendedSize count bytes. Set every range and ByteOrder explicitly.
type EscapedLengthConfig struct {
	Escape                 byte
	ExtendedSize           int
	ByteOrder              ByteOrder
	MinExtended, MaxLength uint64
}

// IdentifierEncoding is an immutable, concurrently shareable field snapshot.
// Its zero value is invalid. It requires no Close; wire semantics belong to Core.
type IdentifierEncoding struct{ native capi.Field }

// LengthEncoding is an immutable, concurrently shareable definite-count encoding.
// Its zero value is invalid. Counts use uint64 without native-size narrowing.
type LengthEncoding struct{ native capi.Field }

func statusResult(code capi.Code) error {
	if code == capi.OK {
		return nil
	}
	return StatusError{code: code}
}

// NewFixedIdentifier validates a nonzero fixed raw identifier width.
func NewFixedIdentifier(size int) (IdentifierEncoding, error) {
	f, code := capi.FixedIdentifier(size)
	return IdentifierEncoding{f}, statusResult(code)
}

// NewVariableIdentifier validates and copies a variable identifier configuration.
func NewVariableIdentifier(c VariableIdentifierConfig) (IdentifierEncoding, error) {
	config := capi.VariableIdentifierConfig{InlineMask: c.InlineMask, Escape: c.Escape,
		ContinuationBit: c.ContinuationBit, PayloadMask: c.PayloadMask, MaxSize: c.MaxSize}
	if c.Policy != nil {
		config.Policy = &capi.IdentifierPolicy{ForbiddenLeading: c.Policy.ForbiddenLeading,
			RejectZeroFirstPayload: c.Policy.RejectZeroFirstPayload, RequireMinimal: c.Policy.RequireMinimal}
	}
	f, code := capi.VariableIdentifier(config)
	return IdentifierEncoding{f}, statusResult(code)
}

// NewFixedLength validates a one-through-eight-byte unsigned count encoding.
func NewFixedLength(size int, order ByteOrder) (LengthEncoding, error) {
	f, code := capi.FixedLength(size, int(order))
	return LengthEncoding{f}, statusResult(code)
}

// NewVariableLength validates and copies a definite variable count configuration.
func NewVariableLength(c VariableLengthConfig) (LengthEncoding, error) {
	config := capi.VariableLengthConfig{LongFormBit: c.LongFormBit, PayloadMask: c.PayloadMask, ByteOrder: int(c.ByteOrder)}
	if c.Policy != nil {
		config.Policy = &capi.LengthPolicy{AllowShort: c.Policy.AllowShort,
			AllowLong: c.Policy.AllowLong, RequireMinimal: c.Policy.RequireMinimal,
			MaxLongOctets: c.Policy.MaxLongOctets, MaxValue: c.Policy.MaxValue}
	}
	f, code := capi.VariableLength(config)
	return LengthEncoding{f}, statusResult(code)
}

// NewEscapedLength validates an inline or escape-prefixed count configuration.
func NewEscapedLength(c EscapedLengthConfig) (LengthEncoding, error) {
	f, code := capi.EscapedLength(capi.EscapedLengthConfig{Escape: c.Escape, ExtendedSize: c.ExtendedSize,
		ByteOrder: int(c.ByteOrder), MinExtended: c.MinExtended, MaxLength: c.MaxLength})
	return LengthEncoding{f}, statusResult(code)
}

// Valid reports whether this encoding was successfully constructed.
func (f IdentifierEncoding) Valid() bool { return f.native.Valid() }

// Valid reports whether this encoding was successfully constructed.
func (f LengthEncoding) Valid() bool { return f.native.Valid() }

// Read decodes one identifier borrowing data. On failure the identifier is nil;
// consumed preserves Core's available-prefix detail, including truncated input.
func (f IdentifierEncoding) Read(data []byte) (tag []byte, consumed int, err error) {
	tag, consumed, code := f.native.ReadTag(data)
	return tag, consumed, statusResult(code)
}

// Measure validates tag and returns its encoded width without writing.
func (f IdentifierEncoding) Measure(tag []byte) (int, error) {
	n, code := f.native.WriteTag(nil, tag, true)
	return n, statusResult(code)
}

// Write validates and copies tag into dst. Overlap is rejected. Failure leaves
// dst unchanged and returns zero written. An empty dst is not a sizing request.
func (f IdentifierEncoding) Write(dst, tag []byte) (int, error) {
	n, code := f.native.WriteTag(dst, tag, false)
	return n, statusResult(code)
}

// Read decodes one count. Failure returns zero value and Core's consumed prefix.
func (f LengthEncoding) Read(data []byte) (value uint64, consumed int, err error) {
	value, consumed, code := f.native.ReadLength(data)
	return value, consumed, statusResult(code)
}

// Measure validates value and returns its encoded width without writing.
func (f LengthEncoding) Measure(value uint64) (int, error) {
	n, code := f.native.WriteLength(nil, value, true)
	return n, statusResult(code)
}

// Write encodes value into dst. Failure leaves dst unchanged and returns zero
// written. Use Measure for sizing; an empty dst returns a capacity error.
func (f LengthEncoding) Write(dst []byte, value uint64) (int, error) {
	n, code := f.native.WriteLength(dst, value, false)
	return n, statusResult(code)
}

// PackedField selects unsigned bits in a one-through-eight-byte backing integer.
// BitOffset counts from its least significant bit after applying ByteOrder.
// Core validates each operation; copies own no storage.
type PackedField struct {
	StorageSize, BitOffset, BitWidth int
	ByteOrder                        ByteOrder
}

func (f PackedField) native() capi.PackedField {
	return capi.PackedField{StorageSize: f.StorageSize, BitOffset: f.BitOffset, BitWidth: f.BitWidth, ByteOrder: int(f.ByteOrder)}
}

// Read extracts the field from data, returning zero on failure.
func (f PackedField) Read(data []byte) (uint64, error) {
	value, code := f.native().Read(data)
	return value, statusResult(code)
}

// Write inserts value into initialized dst, preserving bits outside the field.
// All errors leave dst unchanged. Values are never silently truncated.
func (f PackedField) Write(dst []byte, value uint64) error {
	return statusResult(f.native().Write(dst, value))
}
