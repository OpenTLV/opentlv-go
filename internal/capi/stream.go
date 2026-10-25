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
)

// TreeCursor stores a pointer-free Core continuation and Go-owned frames/input.
// C only borrows these during a call; the immutable Format owns its configuration.
type TreeCursor struct {
	state            C.tlv_tree_reader_t
	frames           []C.tlv_tree_frame_t
	format           Format
	input            []byte
	consumed, offset int
	atEnd            bool
}

type TreeEvent struct {
	Kind, Depth, Offset int
	Element             Element
	Skipped             bool
}

const (
	TreeBegin   = int(C.TLV_TREE_BEGIN)
	TreeElement = int(C.TLV_TREE_ELEMENT)
	TreeEnd     = int(C.TLV_TREE_END)
)

func NewTreeCursor(format Format, input []byte, final bool, capacity, depth, elements int) (*TreeCursor, Code) {
	t := &TreeCursor{format: format, frames: make([]C.tlv_tree_frame_t, capacity)}
	r := t.call(0, input, 0, final, depth, elements)
	if Code(r.code) != OK {
		return nil, Code(r.code)
	}
	t.input = input
	return t, OK
}

func (t *TreeCursor) call(operation int, input []byte, discard int, final bool, depth, elements int) C.go_tree_read_result {
	var frames *C.tlv_tree_frame_t
	if len(t.frames) > 0 {
		frames = &t.frames[0]
	}
	var eof C.int
	if final {
		eof = 1
	}
	r := C.go_tree_read(t.format.config, &t.state, frames, C.size_t(len(t.frames)),
		bytePointer(t.input), bytePointer(input), C.size_t(len(input)), C.size_t(discard), eof,
		C.size_t(depth), C.size_t(elements), C.int(operation))
	runtime.KeepAlive(t)
	runtime.KeepAlive(input)
	t.consumed, t.offset, t.atEnd = int(r.consumed), int(r.offset), r.at_end != 0
	return r
}

func (t *TreeCursor) Next(events bool) (TreeEvent, Code, Diagnostic) {
	operation := 2
	if events {
		operation = 1
	}
	r := t.call(operation, nil, 0, false, 0, 0)
	defer runtime.KeepAlive(t)
	code := Code(r.code)
	if code != OK {
		return TreeEvent{}, code, readerDiagnostic(r.diagnostic, code)
	}
	e := r.event
	result := TreeEvent{Kind: int(e.kind), Depth: int(e.depth), Offset: int(e.offset), Skipped: e.skipped != 0}
	if result.Kind == TreeEnd {
		return result, OK, Diagnostic{}
	}
	start := int(e.offset - t.state.input.base_offset)
	input := t.input[start : start+int(e.source.size)]
	s := Source{Bytes: input, Header: wireRange(e.source.header), Tag: wireRange(e.source.tag),
		Length: wireRange(e.source.length), Value: wireRange(e.source.value), Trailer: wireRange(e.source.trailer),
		FormatTag: e.source.tag_binding == C.TLV_TAG_BINDING_FORMAT}
	result.Element = Element{Source: s, Value: input[s.Value.Offset : s.Value.Offset+s.Value.Size]}
	if s.FormatTag {
		result.Element.Tag = bytes.Clone(nativeBytes(e.element.tag.data, e.element.tag.size))
	} else if s.Tag.Present {
		result.Element.Tag = input[s.Tag.Offset : s.Tag.Offset+s.Tag.Size]
	}
	return result, OK, Diagnostic{}
}

func (t *TreeCursor) SetInput(input []byte, discard int, final bool) Code {
	r := t.call(3, input, discard, final, 0, 0)
	if Code(r.code) == OK {
		t.input = input
	}
	return Code(r.code)
}

func (t *TreeCursor) Skip() Code     { return Code(t.call(4, nil, 0, false, 0, 0).code) }
func (t *TreeCursor) Consumed() int  { return t.consumed }
func (t *TreeCursor) Offset() int    { return t.offset }
func (t *TreeCursor) AtEnd() bool    { return t.atEnd }
func (t *TreeCursor) InputSize() int { return len(t.input) }
func (t *TreeCursor) Base() int      { return int(t.state.input.base_offset) }
func (t *TreeCursor) Capacity() int  { return len(t.frames) }
func (t *TreeCursor) Grow(capacity int) {
	frames := make([]C.tlv_tree_frame_t, capacity)
	copy(frames, t.frames)
	t.frames = frames
}

func (f Format) AppendEncoded(output, source, tag, value []byte, preserve bool) (Code, Diagnostic) {
	defer runtime.KeepAlive(f)
	var mode C.int
	if preserve {
		mode = 1
	}
	r := C.go_writer_append(f.config, bytePointer(output), C.size_t(len(output)),
		bytePointer(source), C.size_t(len(source)), bytePointer(tag), C.size_t(len(tag)),
		bytePointer(value), C.size_t(len(value)), mode)
	defer runtime.KeepAlive(output)
	defer runtime.KeepAlive(source)
	defer runtime.KeepAlive(tag)
	defer runtime.KeepAlive(value)
	return Code(r.code), writerDiagnostic(r, tag)
}
