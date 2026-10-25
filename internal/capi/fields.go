// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include "bridge.h"
*/
import "C"

import "runtime"

// Field holds a pointer-free, immutable native configuration snapshot.
type Field struct{ config C.go_field }

type IdentifierPolicy struct {
	ForbiddenLeading                       []byte
	RejectZeroFirstPayload, RequireMinimal bool
}
type VariableIdentifierConfig struct {
	InlineMask, Escape, ContinuationBit, PayloadMask byte
	MaxSize                                          int
	Policy                                           *IdentifierPolicy
}
type LengthPolicy struct {
	AllowShort, AllowLong, RequireMinimal bool
	MaxLongOctets                         int
	MaxValue                              uint64
}
type VariableLengthConfig struct {
	LongFormBit, PayloadMask byte
	ByteOrder                int
	Policy                   *LengthPolicy
}
type EscapedLengthConfig struct {
	Escape                  byte
	ExtendedSize, ByteOrder int
	MinExtended, MaxLength  uint64
}

func unsignedBool(v bool) C.uint {
	if v {
		return 1
	}
	return 0
}
func fieldResult(f Field) (Field, Code) {
	code := Code(C.go_field_check(f.config))
	if code != OK {
		return Field{}, code
	}
	return f, OK
}
func FixedIdentifier(size int) (Field, Code) {
	if size < 0 {
		return Field{}, InvalidArg
	}
	f := Field{}
	f.config.kind = C.GO_FIELD_FIXED_IDENTIFIER
	f.config.fixed_identifier.size = C.size_t(size)
	return fieldResult(f)
}
func FixedLength(size, order int) (Field, Code) {
	if size < 0 || int(C.int(order)) != order {
		return Field{}, InvalidArg
	}
	f := Field{}
	f.config.kind = C.GO_FIELD_FIXED_LENGTH
	f.config.fixed_length.size = C.size_t(size)
	f.config.fixed_length.byte_order = C.tlv_byte_order_t(order)
	return fieldResult(f)
}
func VariableIdentifier(c VariableIdentifierConfig) (Field, Code) {
	if c.MaxSize < 0 {
		return Field{}, InvalidArg
	}
	f := Field{}
	f.config.kind = C.GO_FIELD_VARIABLE_IDENTIFIER
	f.config.variable_identifier = C.tlv_variable_identifier_t{
		inline_mask: C.uint8_t(c.InlineMask), escape: C.uint8_t(c.Escape),
		continuation_bit: C.uint8_t(c.ContinuationBit), payload_mask: C.uint8_t(c.PayloadMask),
		max_size: C.size_t(c.MaxSize)}
	if c.Policy != nil {
		f.config.has_policy = 1
		p := &f.config.identifier_policy
		p.reject_zero_first_payload = unsignedBool(c.Policy.RejectZeroFirstPayload)
		p.require_minimal = unsignedBool(c.Policy.RequireMinimal)
		// Membership is a set: snapshot arbitrary input, including duplicates.
		var seen [256]bool
		for _, b := range c.Policy.ForbiddenLeading {
			if !seen[b] {
				f.config.forbidden[int(p.forbidden_leading_count)] = C.uint8_t(b)
				p.forbidden_leading_count++
				seen[b] = true
			}
		}
	}
	return fieldResult(f)
}
func VariableLength(c VariableLengthConfig) (Field, Code) {
	if int(C.int(c.ByteOrder)) != c.ByteOrder {
		return Field{}, InvalidArg
	}
	f := Field{}
	f.config.kind = C.GO_FIELD_VARIABLE_LENGTH
	f.config.variable_length = C.tlv_variable_length_t{long_form_bit: C.uint8_t(c.LongFormBit),
		payload_mask: C.uint8_t(c.PayloadMask), byte_order: C.tlv_byte_order_t(c.ByteOrder)}
	if c.Policy != nil {
		if c.Policy.MaxLongOctets < 0 {
			return Field{}, InvalidArg
		}
		f.config.has_policy = 1
		f.config.length_policy = C.tlv_length_policy_t{allow_short: unsignedBool(c.Policy.AllowShort),
			allow_long: unsignedBool(c.Policy.AllowLong), require_minimal: unsignedBool(c.Policy.RequireMinimal),
			max_long_octets: C.size_t(c.Policy.MaxLongOctets), max_value: C.tlv_size_t(c.Policy.MaxValue)}
	}
	return fieldResult(f)
}
func EscapedLength(c EscapedLengthConfig) (Field, Code) {
	if c.ExtendedSize < 0 || int(C.int(c.ByteOrder)) != c.ByteOrder {
		return Field{}, InvalidArg
	}
	f := Field{}
	f.config.kind = C.GO_FIELD_ESCAPED_LENGTH
	f.config.escaped_length = C.tlv_escaped_length_t{escape: C.uint8_t(c.Escape),
		extended_size: C.size_t(c.ExtendedSize), byte_order: C.tlv_byte_order_t(c.ByteOrder),
		min_extended: C.tlv_size_t(c.MinExtended), max_length: C.tlv_size_t(c.MaxLength)}
	return fieldResult(f)
}
func (f Field) Valid() bool { return f.config.kind != 0 }
func (f Field) ReadTag(data []byte) ([]byte, int, Code) {
	var tag C.tlv_tag_t
	var consumed C.size_t
	code := Code(C.go_field_read_tag(f.config, bytePointer(data), C.size_t(len(data)), &tag, &consumed))
	runtime.KeepAlive(data)
	if code != OK {
		return nil, int(consumed), code
	}
	if tag.data == nil {
		return nil, int(consumed), OK
	}
	return data[:int(tag.size)], int(consumed), OK
}
func (f Field) WriteTag(out, tag []byte, measure bool) (int, Code) {
	if overlaps(out, tag) {
		return 0, InvalidArg
	}
	if !measure && len(out) == 0 {
		out = make([]byte, 0, 1)
	}
	var written C.size_t
	code := Code(C.go_field_write_tag(f.config, bytePointer(tag), C.size_t(len(tag)),
		bytePointer(out), C.size_t(len(out)), &written))
	runtime.KeepAlive(out)
	runtime.KeepAlive(tag)
	return int(written), code
}
func (f Field) ReadLength(data []byte) (uint64, int, Code) {
	var length C.tlv_size_t
	var consumed C.size_t
	code := Code(C.go_field_read_length(f.config, bytePointer(data), C.size_t(len(data)), &length, &consumed))
	runtime.KeepAlive(data)
	return uint64(length), int(consumed), code
}
func (f Field) WriteLength(out []byte, value uint64, measure bool) (int, Code) {
	if !measure && len(out) == 0 {
		out = make([]byte, 0, 1)
	}
	var written C.size_t
	code := Code(C.go_field_write_length(f.config, C.tlv_size_t(value), bytePointer(out), C.size_t(len(out)), &written))
	runtime.KeepAlive(out)
	return int(written), code
}

type PackedField struct{ StorageSize, BitOffset, BitWidth, ByteOrder int }

func packedField(c PackedField) (C.tlv_packed_field_t, Code) {
	if c.StorageSize < 0 || c.BitOffset < 0 || c.BitWidth < 0 ||
		uint64(C.uint(c.BitOffset)) != uint64(c.BitOffset) || uint64(C.uint(c.BitWidth)) != uint64(c.BitWidth) ||
		int(C.int(c.ByteOrder)) != c.ByteOrder {
		return C.tlv_packed_field_t{}, InvalidArg
	}
	return C.tlv_packed_field_t{storage_size: C.size_t(c.StorageSize), bit_offset: C.uint(c.BitOffset),
		bit_width: C.uint(c.BitWidth), byte_order: C.tlv_byte_order_t(c.ByteOrder)}, OK
}
func (f PackedField) Read(data []byte) (uint64, Code) {
	c, code := packedField(f)
	if code != OK {
		return 0, code
	}
	if len(data) == 0 {
		data = make([]byte, 0, 1)
	}
	var value C.uint64_t
	code = Code(C.tlv_packed_field_read(&c, bytePointer(data), C.size_t(len(data)), &value))
	runtime.KeepAlive(data)
	return uint64(value), code
}
func (f PackedField) Write(data []byte, value uint64) Code {
	c, code := packedField(f)
	if code != OK {
		return code
	}
	if len(data) == 0 {
		data = make([]byte, 0, 1)
	}
	code = Code(C.tlv_packed_field_write(&c, bytePointer(data), C.size_t(len(data)), C.uint64_t(value)))
	runtime.KeepAlive(data)
	return code
}

type ConstructedBit struct {
	ByteIndex   int
	Mask, Value byte
}
type Composition struct {
	Identifier, Length Field
	Order, LengthScope int
	TagOnly            [][]byte
	Constructed        *ConstructedBit
}
type PackedConfig struct {
	HeaderSize, TagSize, LengthScope int
	Tag, Length                      PackedField
	TagStorage                       []byte
}

const (
	ComposedFormat = C.GO_COMPOSED
	EscapedFormat  = C.GO_ESCAPED
	VariableFormat = C.GO_VARIABLE
	PackedFormat   = C.GO_PACKED
)

type formatOwner struct{ ptr *C.go_owned_format }

func (o *formatOwner) free() { C.go_owned_format_free(o.ptr) }
func Composed(c Composition, kind int, p PackedConfig) (Format, Code) {
	if int(C.int(c.Order)) != c.Order || int(C.int(c.LengthScope)) != c.LengthScope {
		return Format{}, InvalidArg
	}
	var predicate C.tlv_constructed_bit_t
	var hasPredicate C.int
	if c.Constructed != nil {
		if c.Constructed.ByteIndex < 0 {
			return Format{}, InvalidArg
		}
		hasPredicate = 1
		predicate = C.tlv_constructed_bit_t{byte_index: C.size_t(c.Constructed.ByteIndex),
			mask: C.uint8_t(c.Constructed.Mask), value: C.uint8_t(c.Constructed.Value)}
	}
	var packed C.tlv_packed_layout_t
	if kind == C.GO_PACKED {
		if p.HeaderSize < 0 || p.TagSize < 0 || int(C.int(p.LengthScope)) != p.LengthScope {
			return Format{}, InvalidArg
		}
		tag, code := packedField(p.Tag)
		if code != OK {
			return Format{}, code
		}
		length, code := packedField(p.Length)
		if code != OK {
			return Format{}, code
		}
		packed = C.tlv_packed_layout_t{header_size: C.size_t(p.HeaderSize), tag: tag, length: length,
			length_scope: C.tlv_length_scope_t(p.LengthScope), tag_size: C.size_t(p.TagSize)}
	}
	ptr := C.go_owned_format_new(c.Identifier.config, c.Length.config, C.int(kind), C.int(c.Order),
		C.int(c.LengthScope), predicate, hasPredicate, packed, C.size_t(len(c.TagOnly)))
	if ptr == nil {
		return Format{}, OutOfMemory
	}
	for i, tag := range c.TagOnly {
		code := Code(C.go_owned_format_tag(ptr, C.size_t(i), bytePointer(tag), C.size_t(len(tag))))
		runtime.KeepAlive(tag)
		if code != OK {
			C.go_owned_format_free(ptr)
			return Format{}, code
		}
	}
	if kind == C.GO_PACKED {
		code := Code(C.go_owned_format_storage(ptr, bytePointer(p.TagStorage), C.size_t(len(p.TagStorage))))
		runtime.KeepAlive(p.TagStorage)
		if code != OK {
			C.go_owned_format_free(ptr)
			return Format{}, code
		}
	}
	code := Code(C.go_owned_format_init(ptr))
	if code != OK {
		C.go_owned_format_free(ptr)
		return Format{}, code
	}
	owner := &formatOwner{ptr}
	runtime.SetFinalizer(owner, (*formatOwner).free)
	return Format{config: C.go_format{kind: C.GO_FORMAT_OWNED, owned: ptr}, owner: owner}, OK
}
func (f Format) Inspect(operation int, tag []byte) bool {
	defer runtime.KeepAlive(f)
	result := C.go_format_inspect(f.config, C.int(operation), bytePointer(tag), C.size_t(len(tag)))
	runtime.KeepAlive(tag)
	return result != 0
}

func (f Format) Preserve(source, tag, value, output []byte) Code {
	defer runtime.KeepAlive(f)
	code := Code(C.go_format_preserve(f.config, bytePointer(source), C.size_t(len(source)),
		bytePointer(tag), C.size_t(len(tag)), bytePointer(value), C.size_t(len(value)),
		bytePointer(output), C.size_t(len(output))))
	runtime.KeepAlive(source)
	runtime.KeepAlive(tag)
	runtime.KeepAlive(value)
	runtime.KeepAlive(output)
	return code
}
