// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// ConstructedBit classifies canonical identifier bytes using an exact masked
// comparison. Mask must be nonzero and Value may contain only masked bits.
type ConstructedBit struct {
	ByteIndex   int
	Mask, Value byte
}

func (p *ConstructedBit) native() *capi.ConstructedBit {
	if p == nil {
		return nil
	}
	return &capi.ConstructedBit{ByteIndex: p.ByteIndex, Mask: p.Mask, Value: p.Value}
}

// CompositionConfig combines independently selected identifier and count fields.
// TagOnly identifiers omit Length/Value and require TLV/ValueLength framing.
// They imply no skip or termination behavior. Constructors copy all tables and
// the optional constructed predicate; no caller storage is retained.
type CompositionConfig struct {
	Identifier  IdentifierEncoding
	Length      LengthEncoding
	Order       ElementOrder
	LengthScope LengthScope
	TagOnly     [][]byte
	Constructed *ConstructedBit
}

func (c CompositionConfig) native() capi.Composition {
	return capi.Composition{Identifier: c.Identifier.native, Length: c.Length.native,
		Order: int(c.Order), LengthScope: int(c.LengthScope), TagOnly: c.TagOnly, Constructed: c.Constructed.native()}
}

// NewComposed validates a snapshot combining fixed/variable identifiers with
// fixed/variable/escaped lengths. Field placement and tag-only handling use Core.
func NewComposed(c CompositionConfig) (Format, error) {
	if !c.Identifier.Valid() || !c.Length.Valid() {
		return Format{}, StatusError{code: capi.InvalidArg}
	}
	f, code := capi.Composed(c.native(), capi.ComposedFormat, capi.PackedConfig{})
	return formatResult(f, code)
}

// VariableConfig configures definite variable identifiers and counts. There is
// no indefinite-length or protocol-specific policy in this generic constructor.
type VariableConfig struct {
	Identifier  VariableIdentifierConfig
	Length      VariableLengthConfig
	Order       ElementOrder
	LengthScope LengthScope
	Constructed *ConstructedBit
}

// NewVariable validates and copies variable configuration through Core.
func NewVariable(c VariableConfig) (Format, error) {
	identifier, err := NewVariableIdentifier(c.Identifier)
	if err != nil {
		return Format{}, err
	}
	length, err := NewVariableLength(c.Length)
	if err != nil {
		return Format{}, err
	}
	config := CompositionConfig{Identifier: identifier, Length: length, Order: c.Order,
		LengthScope: c.LengthScope, Constructed: c.Constructed}
	f, code := capi.Composed(config.native(), capi.VariableFormat, capi.PackedConfig{})
	return formatResult(f, code)
}

// EscapedConfig combines a nonzero fixed identifier width with escaped lengths.
// Optional TagOnly entries require TLV/ValueLength framing and empty values.
type EscapedConfig struct {
	TagSize     int
	Length      EscapedLengthConfig
	Order       ElementOrder
	LengthScope LengthScope
	TagOnly     [][]byte
}

// NewEscaped validates and copies escaped-length configuration through Core.
func NewEscaped(c EscapedConfig) (Format, error) {
	identifier, err := NewFixedIdentifier(c.TagSize)
	if err != nil {
		return Format{}, err
	}
	length, err := NewEscapedLength(c.Length)
	if err != nil {
		return Format{}, err
	}
	config := CompositionConfig{Identifier: identifier, Length: length, Order: c.Order,
		LengthScope: c.LengthScope, TagOnly: c.TagOnly}
	f, code := capi.Composed(config.native(), capi.EscapedFormat, capi.PackedConfig{})
	return formatResult(f, code)
}

// PackedConfig describes nonoverlapping Tag/Length bits in one backing integer.
// TagStorage contains every canonical identifier: entry i is the unsigned
// big-endian encoding of i in TagSize bytes. Core validates table capacity at
// construction and the selected entry on decode. The constructor copies it.
// TagAndValueLength counts the tag's wire byte envelope, not TagSize.
type PackedConfig struct {
	HeaderSize, TagSize int
	Tag, Length         PackedField
	LengthScope         LengthScope
	TagStorage          []byte
	Constructed         *ConstructedBit
}

// NewPacked validates and copies a packed layout. Decoded tags are independent
// Go copies of format storage, while decoded values continue to borrow input.
func NewPacked(c PackedConfig) (Format, error) {
	config := capi.PackedConfig{HeaderSize: c.HeaderSize, TagSize: c.TagSize,
		Tag: c.Tag.native(), Length: c.Length.native(), LengthScope: int(c.LengthScope), TagStorage: c.TagStorage}
	f, code := capi.Composed(capi.Composition{Constructed: c.Constructed.native()}, capi.PackedFormat, config)
	return formatResult(f, code)
}

// CanRead reports Core's decoding capability. An invalid Format returns false.
func (f Format) CanRead() bool { return f.Valid() && f.native.Inspect(0, nil) }

// CanWrite reports Core's complete encoding capability. Invalid returns false.
func (f Format) CanWrite() bool { return f.Valid() && f.native.Inspect(1, nil) }

// IsConstructed classifies a canonical tag using the configured predicate.
// It does not validate wire representability; invalid Formats return false.
func (f Format) IsConstructed(tag []byte) bool { return f.Valid() && f.native.Inspect(2, tag) }

// Preserve returns an independent copy of an element's exact source encoding,
// including accepted nonminimal fields. It revalidates Source.Bytes with f and
// uses Core to check that the tag and value still match. Source bytes must remain
// unchanged since decoding. Logical elements without source return ErrInvalidArg.
// Use the originating Format; Writer instead regenerates canonical framing.
func (f Format) Preserve(element Element) ([]byte, error) {
	source := element.Source().Bytes
	if !f.Valid() || len(source) == 0 {
		return nil, StatusError{code: capi.InvalidArg}
	}
	output := make([]byte, len(source))
	code := f.native.Preserve(source, element.Tag(), element.Value(), output)
	if code != capi.OK {
		return nil, StatusError{code: code}
	}
	return output, nil
}
