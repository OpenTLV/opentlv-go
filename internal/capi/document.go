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

// Document owns persistent C resources; callers must serialize access and Close.
type Document struct{ ptr *C.go_document }

// Node is a private borrowed native reference, valid only under its owner's checks.
type Node struct{ ptr unsafe.Pointer }

// ParseDocument copies input into the canonical native Document.
func (f Format) ParseDocument(data []byte, depth, elements int, defaults, retainSourceLocations bool) (*Document, Code, Diagnostic) {
	if depth < 0 || elements < 0 {
		return nil, InvalidArg, Diagnostic{Code: InvalidArg}
	}
	var code C.int
	var diag C.tlv_reader_diagnostic_t
	var def C.int
	if defaults {
		def = 1
	}
	var retain C.int
	if retainSourceLocations {
		retain = 1
	}
	p := C.go_document_parse(f.config, bytePointer(data), C.size_t(len(data)), C.size_t(depth), C.size_t(elements), def, retain, &code, &diag)
	defer runtime.KeepAlive(data)
	if Code(code) != OK {
		return nil, Code(code), readerDiagnostic(diag, Code(code))
	}
	d := &Document{p}
	runtime.SetFinalizer(d, (*Document).Close)
	return d, OK, Diagnostic{}
}

// Close releases all native allocations, once.
func (d *Document) Close() {
	if d.ptr != nil {
		C.go_document_free(d.ptr)
		d.ptr = nil
	}
	runtime.SetFinalizer(d, nil)
}

// Count returns the native total element count.
func (d *Document) Count() int { defer runtime.KeepAlive(d); return int(C.go_document_count(d.ptr)) }

// Navigate delegates root, child, sibling and parent navigation to C.
func (d *Document) Navigate(n Node, operation int) Node {
	defer runtime.KeepAlive(d)
	return Node{C.go_document_node(d.ptr, n.ptr, C.int(operation))}
}

// Valid reports whether navigation found a node.
func (n Node) Valid() bool { return n.ptr != nil }

// Read returns independent Go snapshots of node content.
func (d *Document) Read(n Node) (Element, bool) {
	defer runtime.KeepAlive(d)
	r := C.go_document_read(n.ptr)
	// Owned Document values were allocated using size_t, so this conversion
	// cannot narrow a real native allocation, including on 32-bit targets.
	return Element{Tag: bytes.Clone(nativeBytes(r.element.tag.data, r.element.tag.size)), Value: bytes.Clone(nativeBytes(r.element.value.data, C.size_t(r.element.value.size)))}, r.consumed != 0
}

// Edit delegates replacement, erase and insertion to C.
func (d *Document) Edit(n, before Node, tag, value []byte, operation int) (Node, Code) {
	var result unsafe.Pointer
	code := C.go_document_edit(d.ptr, n.ptr, before.ptr, bytePointer(tag), C.size_t(len(tag)), bytePointer(value), C.size_t(len(value)), C.int(operation), &result)
	runtime.KeepAlive(tag)
	runtime.KeepAlive(value)
	runtime.KeepAlive(d)
	return Node{result}, Code(code)
}

// Encode delegates complete tree measurement or encoding to C Tree Writer.
func (d *Document) Encode(f Format, output []byte, measure bool) (int, Code) {
	var m C.int
	if measure {
		m = 1
	}
	r := C.go_document_encode(d.ptr, f.config, bytePointer(output), C.size_t(len(output)), m)
	runtime.KeepAlive(output)
	runtime.KeepAlive(d)
	if uint64(r.size) > uint64(^uint(0)>>1) {
		return 0, NativeSize
	}
	return int(r.size), Code(r.code)
}
