// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv

import (
	"bytes"
	"github.com/OpenTLV/opentlv-go/internal/capi"
)

// StructureCodecOptions bounds traversal and the complete sequence size.
// Every zero is a literal limit. MaxBytes bounds binding copies, not memory used
// by application callbacks; a callback must bound its own allocations.
type StructureCodecOptions struct {
	Limits   SchemaLimits
	MaxBytes int
}

// DefaultStructureCodecOptions permits 64 levels, 65536 elements and 16 MiB.
func DefaultStructureCodecOptions() StructureCodecOptions {
	return StructureCodecOptions{DefaultSchemaLimits(), 16 << 20}
}

// StructureCodec maps a complete TLV sequence to a Go object. Core validates
// framing, nesting, limits and optional Schema. Callbacks supply object mapping.
// The zero value is unsupported. Copies share callbacks and Schema ownership;
// concurrent use requires concurrency-safe callbacks. No Close is needed here.
type StructureCodec[T any] struct {
	format  Format
	schema  *capi.StructureSchema
	options StructureCodecOptions
	decode  func([]byte) (T, error)
	encode  func(T) ([]byte, error)
}

// NewStructureCodec retains format and an optional prepared schema. Closing that
// schema invalidates subsequent conversions. Either callback may be nil to disable
// that operation. Callbacks run outside native locks, may reenter, and return
// application errors unchanged. They are called at most once per operation.
func NewStructureCodec[T any](format Format, schema *StructureSchema, options StructureCodecOptions,
	decode func([]byte) (T, error), encode func(T) ([]byte, error)) (StructureCodec[T], error) {
	var zero StructureCodec[T]
	if !format.Valid() || options.Limits.MaxDepth < 0 || options.Limits.MaxElements < 0 || options.MaxBytes < 0 {
		return zero, ErrInvalidArg
	}
	var native *capi.StructureSchema
	if schema != nil {
		native = schema.native
		if native == nil {
			return zero, ErrInvalidState
		}
		if code := capi.StructureReady(native); code != capi.OK {
			return zero, StatusError{code: code}
		}
	}
	return StructureCodec[T]{format, native, options, decode, encode}, nil
}

// Decode validates before invoking the callback with a private copy. The callback
// may retain that copy; it never aliases caller input. On error the result is zero.
func (c StructureCodec[T]) Decode(data []byte) (T, error) {
	var zero T
	if c.decode == nil {
		return zero, ErrUnsupported
	}
	if len(data) > c.options.MaxBytes {
		return zero, ErrLimit
	}
	owned := bytes.Clone(data)
	_, code, detail := capi.StructureConvert(c.schema, c.format.native, owned, c.options.Limits.MaxDepth, c.options.Limits.MaxElements, false)
	if code != capi.OK {
		return zero, codecError(int(code), detail)
	}
	result, err := c.decode(owned)
	if err != nil {
		return zero, err
	}
	return result, nil
}

// Encode invokes the callback once, then validates and copies its complete output
// through Core. Failure publishes no bytes. The callback must keep returned bytes
// stable until Encode returns. Output diagnostics use output coordinates.
func (c StructureCodec[T]) Encode(value T) ([]byte, error) {
	if c.encode == nil {
		return nil, ErrUnsupported
	}
	if code := capi.StructureReady(c.schema); code != capi.OK {
		return nil, codecError(int(code), nil)
	}
	data, err := c.encode(value)
	if err != nil {
		return nil, err
	}
	if len(data) > c.options.MaxBytes {
		return nil, ErrLimit
	}
	output, code, detail := capi.StructureConvert(c.schema, c.format.native, data, c.options.Limits.MaxDepth, c.options.Limits.MaxElements, true)
	if code != capi.OK {
		return nil, codecError(int(code), detail)
	}
	return output, nil
}
