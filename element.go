// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import (
	"bytes"
	"github.com/marekcingel/OpenTLV/bindings/go/internal/capi"
)

// Range describes an optional wire region relative to the start of an element.
// Present distinguishes an absent region from a present region with zero size.
type Range struct {
	Offset, Size int
	Present      bool
}

// Source describes an element's wire representation. Bytes borrows the input.
// Offset is the element start in the enclosing input; all ranges are relative
// to Bytes. FormatTag means the semantic Tag comes from the Format rather than
// directly from the wire Tag range. Keep borrowed bytes unchanged while in use.
// A zero Source has no encoding information.
type Source struct {
	Bytes                               []byte
	Offset                              int
	Header, Tag, Length, Value, Trailer Range
	FormatTag                           bool
}

// Element is a format-independent Tag/Value view with separate source metadata.
// Accessors borrow storage: callers must keep bytes unchanged while views are in
// use and avoid concurrent mutation. Clone creates independent owned storage.
// The zero value is an empty logical element with no source information.
type Element struct {
	tag, value []byte
	source     Source
}

// NewElement borrows tag and value without interpreting or normalizing them.
// It has no source metadata; Writer validates it against its Format.
func NewElement(tag, value []byte) Element { return Element{tag: tag, value: value} }

// Tag returns the binary identifier, preserving byte identity and nil versus empty.
func (e Element) Tag() []byte { return e.tag }

// Value returns the uninterpreted Value bytes.
func (e Element) Value() []byte { return e.value }

// Source returns wire metadata. Its Bytes continues to borrow the original input.
func (e Element) Source() Source { return e.source }

// Offset returns the source offset; consult Source for encoding availability.
// Logical elements without source metadata return zero.
func (e Element) Offset() int { return e.source.Offset }

// Clone copies Tag, Value and raw encoding into Go-owned storage, preserving
// source ranges and offset. The returned slices are independent of the original.
func (e Element) Clone() Element {
	s := e.source
	s.Bytes = bytes.Clone(s.Bytes)
	return Element{tag: bytes.Clone(e.tag), value: bytes.Clone(e.value), source: s}
}

func elementFromNative(e capi.Element, offset int) Element {
	rangeFromNative := func(r capi.Range) Range { return Range{Offset: r.Offset, Size: r.Size, Present: r.Present} }
	s := e.Source
	return Element{tag: e.Tag, value: e.Value, source: Source{Bytes: s.Bytes, Offset: offset, Header: rangeFromNative(s.Header), Tag: rangeFromNative(s.Tag), Length: rangeFromNative(s.Length), Value: rangeFromNative(s.Value), Trailer: rangeFromNative(s.Trailer), FormatTag: s.FormatTag}}
}
