// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include "bridge.h"
*/
import "C"

import (
	"bytes"
	"runtime"
	"unsafe"
)

// Code preserves the C engine's status, including end and resumable input.
type Code int

const (
	OK             Code = C.TLV_OK
	BufferTooShort Code = C.TLV_ERR_BUFFER_TOO_SHORT
	InvalidLength  Code = C.TLV_ERR_INVALID_LENGTH
	NullArg        Code = C.TLV_ERR_NULL_ARG
	OutOfMemory    Code = C.TLV_ERR_OUT_OF_MEMORY
	End            Code = C.TLV_END
	Truncated      Code = C.TLV_ERR_TRUNCATED
	InvalidTag     Code = C.TLV_ERR_INVALID_TAG
	Visitor        Code = C.TLV_ERR_VISITOR
	Limit          Code = C.TLV_ERR_LIMIT
	Schema         Code = C.TLV_ERR_SCHEMA
	InvalidArg     Code = C.TLV_ERR_INVALID_ARG
	InvalidTagSize Code = C.TLV_ERR_INVALID_TAG_SIZE
	Syntax         Code = C.TLV_ERR_SYNTAX
	Overflow       Code = C.TLV_ERR_OVERFLOW
	InvalidValue   Code = C.TLV_ERR_INVALID_VALUE
	Unsupported    Code = C.TLV_ERR_UNSUPPORTED
	InvalidSchema  Code = C.TLV_ERR_INVALID_SCHEMA
	NativeSize     Code = C.TLV_ERR_NATIVE_SIZE
	NeedMoreData   Code = C.TLV_NEED_MORE_DATA
	InvalidState   Code = C.TLV_ERR_INVALID_STATE
	Callback       Code = C.TLV_ERR_CALLBACK
)

// String copies the native status description into Go storage.
func (code Code) String() string { return C.GoString(C.tlv_result_string(C.tlv_result_t(code))) }

// Kind identifies a native format. Availability follows the linked C configuration.
type Kind int

const (
	Fixed         Kind = C.GO_FORMAT_FIXED
	BER           Kind = C.GO_FORMAT_BER
	BERIndefinite Kind = C.GO_FORMAT_BER_INDEFINITE
	DER           Kind = C.GO_FORMAT_DER
	CER           Kind = C.GO_FORMAT_CER
	EMV           Kind = C.GO_FORMAT_EMV
	LLDP          Kind = C.GO_FORMAT_LLDP
	BluetoothLTV  Kind = C.GO_FORMAT_BLUETOOTH_LTV
	DHCPv4        Kind = C.GO_FORMAT_DHCPV4
	NFCType2      Kind = C.GO_FORMAT_NFC_TYPE2
)

// FixedConfig mirrors generic fixed-field configuration, without C storage.
// ByteOrder: 1 big endian, 2 little endian. Order: 0 TLV, 1 LTV.
// LengthScope: 0 Value, 1 Tag and Value. Validation belongs to the C engine.
type FixedConfig struct {
	TagSize, LengthSize           int
	ByteOrder, Order, LengthScope int
}

// Format is immutable Go configuration. It owns no native resource and needs no Close.
// Its zero value is invalid; construct it with NewFixed or Builtin.
type Format struct{ config C.go_format }

// NewFixed validates configuration through tlv_fixed_format_init.
func NewFixed(config FixedConfig) (Format, Code) {
	if config.TagSize < 0 || config.LengthSize < 0 {
		return Format{}, InvalidArg
	}
	f := Format{config: C.go_format{kind: C.int(Fixed), tag_size: C.size_t(config.TagSize), length_size: C.size_t(config.LengthSize), byte_order: C.int(config.ByteOrder), element_order: C.int(config.Order), length_scope: C.int(config.LengthScope)}}
	// Reject narrowing before calling C (notably on 64-bit Go with C int enums).
	if int(f.config.byte_order) != config.ByteOrder || int(f.config.element_order) != config.Order || int(f.config.length_scope) != config.LengthScope {
		return Format{}, InvalidArg
	}
	return f, Code(C.go_format_check(f.config))
}

// Builtin selects a compiled-in preset; unavailable or unknown kinds are unsupported.
func Builtin(kind Kind) (Format, Code) {
	if kind < BER || kind > NFCType2 {
		return Format{}, Unsupported
	}
	f := Format{config: C.go_format{kind: C.int(kind)}}
	return f, Code(C.go_format_check(f.config))
}

// Range describes an optional wire region relative to the start of an element.
type Range struct {
	Offset, Size int
	Present      bool
}

// Source preserves raw framing as a borrowed Go slice and native-produced ranges.
// It contains no native descriptor; exact preservation must reconstruct that descriptor.
type Source struct {
	Bytes                               []byte
	Header, Tag, Length, Value, Trailer Range
	FormatTag                           bool
}

// Element borrows input Tag/Value. Format-supplied Tags are copied into Go storage.
// Keep borrowed input unchanged while any result is used; slices retain Go ownership.
type Element struct {
	Tag, Value []byte
	Source     Source
}

// OptionalSize preserves unset versus zero native diagnostic fields.
type OptionalSize struct {
	Value   uint64
	Present bool
}

// DiagnosticContext copies native context strings.
type DiagnosticContext struct{ Layer, Key, Value string }

// Diagnostic is a Go projection of native reader/writer detail, with no C pointers.
// Tag and RawLength are owned snapshots; text is copied from native storage.
// Location is the native evidence domain, anchor and half-open coordinates.
type Location struct {
	Domain, Kind int
	Begin, End   uint64
}

type Diagnostic struct {
	Location                                                  Location
	Code                                                      Code
	Severity, Operation                                       int
	Offset                                                    OptionalSize
	Expected, Actual                                          string
	Tag, RawLength                                            []byte
	HasTag, HasRawLength                                      bool
	TagOffset, LengthOffset, ValueOffset                      OptionalSize
	DeclaredLength, Available, EnclosingEnd, Required, Length OptionalSize
	Contexts                                                  []DiagnosticContext
	Path                                                      [][]byte
	HasPath                                                   bool
	PathOmitted                                               uint64
}

func bytePointer(data []byte) *C.uint8_t {
	if data == nil {
		return nil
	}
	return (*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data)))
}

func nativeBytes(data *C.uint8_t, size C.size_t) []byte {
	if data == nil {
		return nil
	}
	// Native tag/range sizes must fit Go's signed slice index.
	if uint64(size) > uint64(^uint(0)>>1) {
		panic("capi: native byte view exceeds Go int")
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(data)), int(size))
}

func wireRange(r C.tlv_range_t) Range                   { return Range{int(r.offset), int(r.size), r.present != 0} }
func optional(value uint64, present C.int) OptionalSize { return OptionalSize{value, present != 0} }
func diagnostic(d C.tlv_diagnostic_t, operation C.int) Diagnostic {
	result := Diagnostic{Location: Location{int(d.location.domain), int(d.location.kind), uint64(d.location.begin), uint64(d.location.end)}, Code: Code(d.code), Severity: int(d.severity), Operation: int(operation), Offset: optional(uint64(d.location.begin), C.int(d.location.kind)), Expected: C.GoString(d.expected), Actual: C.GoString(d.actual)}
	for c := d.contexts; c != nil; c = c.next {
		result.Contexts = append(result.Contexts, DiagnosticContext{C.GoString(c.layer), C.GoString(c.key), C.GoString(c.value)})
	}
	if d.has_path != 0 {
		result.HasPath = true
		result.PathOmitted = uint64(d.path.omitted)
		for i := 0; i < int(d.path.length); i++ {
			t := d.path.tags[i]
			result.Path = append(result.Path, bytes.Clone(nativeBytes(t.data, t.size)))
		}
	}
	return result
}

// Read decodes the first element through the canonical Reader, without copying input.
// final=false preserves NEED_MORE_DATA; consumed is zero on all non-success outcomes.
// No input, output, cursor or configuration pointer is retained in C after return.
func (f Format) Read(input []byte, final bool) (Element, int, Code, Diagnostic) {
	var finalInput C.int
	if final {
		finalInput = 1
	}
	r := C.go_read(f.config, bytePointer(input), C.size_t(len(input)), finalInput)
	defer runtime.KeepAlive(input)
	code := Code(r.code)
	if code != OK {
		d := readerDiagnostic(r.diagnostic, code)
		return Element{}, 0, code, d
	}
	s := Source{Bytes: input[:int(r.source.size)], Header: wireRange(r.source.header), Tag: wireRange(r.source.tag), Length: wireRange(r.source.length), Value: wireRange(r.source.value), Trailer: wireRange(r.source.trailer), FormatTag: r.source.tag_binding == C.TLV_TAG_BINDING_FORMAT}
	e := Element{Source: s, Value: input[s.Value.Offset : s.Value.Offset+s.Value.Size]}
	if s.FormatTag {
		e.Tag = bytes.Clone(nativeBytes(r.element.tag.data, r.element.tag.size))
	} else if s.Tag.Present {
		e.Tag = input[s.Tag.Offset : s.Tag.Offset+s.Tag.Size]
	}
	return e, int(r.consumed), OK, Diagnostic{}
}

func overlaps(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	ap, bp := uintptr(unsafe.Pointer(unsafe.SliceData(a))), uintptr(unsafe.Pointer(unsafe.SliceData(b)))
	if ap <= bp {
		return bp-ap < uintptr(len(a))
	}
	return ap-bp < uintptr(len(b))
}

// Measure delegates complete-element sizing to the C engine.
func (f Format) Measure(tag, value []byte) (int, Code, Diagnostic) {
	return f.write(nil, tag, value, true)
}

// Write borrows caller storage during encoding. Output must not overlap tag/value.
// On short capacity size is the native required size; other failures may modify output
// according to the C encoder contract. No input or output pointer is retained.
func (f Format) Write(output, tag, value []byte) (int, Code, Diagnostic) {
	if overlaps(output, tag) || overlaps(output, value) {
		return 0, InvalidArg, Diagnostic{Code: InvalidArg}
	}
	return f.write(output, tag, value, false)
}

func (f Format) write(output, tag, value []byte, measure bool) (int, Code, Diagnostic) {
	var sizing C.int
	if measure {
		sizing = 1
	}
	r := C.go_write(f.config, bytePointer(output), C.size_t(len(output)), bytePointer(tag), C.size_t(len(tag)), bytePointer(value), C.size_t(len(value)), sizing)
	defer runtime.KeepAlive(output)
	defer runtime.KeepAlive(tag)
	defer runtime.KeepAlive(value)
	code := Code(r.code)
	d := Diagnostic{}
	if code != OK {
		d = writerDiagnostic(r, tag)
	}
	if uint64(r.size) > uint64(^uint(0)>>1) {
		return 0, NativeSize, Diagnostic{Code: NativeSize}
	}
	return int(r.size), code, d
}

func writerDiagnostic(r C.go_write_result, tag []byte) Diagnostic {
	d := diagnostic(r.diagnostic.diagnostic, C.int(r.diagnostic.operation))
	d.Code = Code(r.code)
	d.HasTag = r.diagnostic.has_tag != 0
	if d.HasTag {
		d.Tag = bytes.Clone(tag)
	}
	d.Length = optional(uint64(r.diagnostic.length), r.diagnostic.has_length)
	d.Available = optional(uint64(r.diagnostic.available), r.diagnostic.has_available)
	d.Required = optional(uint64(r.diagnostic.required), r.diagnostic.has_required)
	return d
}

func readerDiagnostic(native C.tlv_reader_diagnostic_t, code Code) Diagnostic {
	return readerDiagnosticParts(native.diagnostic, native.detail, code)
}

func readerDiagnosticParts(common C.tlv_diagnostic_t, native C.tlv_reader_detail_t, code Code) Diagnostic {
	d := diagnostic(common, C.int(native.operation))
	d.Code = code
	d.HasTag = native.has_tag != 0
	if d.HasTag {
		d.Tag = bytes.Clone(nativeBytes(native.tag.data, native.tag.size))
	}
	d.HasRawLength = native.has_raw_length != 0
	if d.HasRawLength {
		d.RawLength = bytes.Clone(nativeBytes(native.raw_length.data, native.raw_length.size))
	}
	d.TagOffset = optional(uint64(native.tag_offset), native.has_tag_offset)
	d.LengthOffset = optional(uint64(native.length_offset), native.has_length_offset)
	d.ValueOffset = optional(uint64(native.value_offset), native.has_value_offset)
	d.DeclaredLength = optional(uint64(native.declared_length), native.has_declared_length)
	d.Available = optional(uint64(native.available), native.has_available)
	d.EnclosingEnd = optional(uint64(native.enclosing_end), native.has_enclosing_end)
	d.Required = optional(uint64(native.required), native.has_required)
	return d
}
