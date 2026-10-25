// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include "bridge.h"
*/
import "C"
import (
	"runtime"
	"runtime/cgo"
	"unsafe"
)

func (d *Document) Find(parent Node, tag []byte) Node {
	defer runtime.KeepAlive(d)
	defer runtime.KeepAlive(tag)
	return Node{C.go_document_find(d.ptr, parent.ptr, bytePointer(tag), C.size_t(len(tag)))}
}

func (d *Document) SourceLocation(n Node) (OptionalSize, OptionalSize) {
	defer runtime.KeepAlive(d)
	var offset, header C.size_t
	var hasOffset, hasHeader C.int
	C.go_node_location(n.ptr, &offset, &header, &hasOffset, &hasHeader)
	return OptionalSize{uint64(offset), hasOffset != 0}, OptionalSize{uint64(header), hasHeader != 0}
}

func (d *Document) EncodeNode(n Node, f Format, output []byte, measure bool) (int, Code) {
	defer runtime.KeepAlive(d)
	defer runtime.KeepAlive(f)
	defer runtime.KeepAlive(output)
	var m C.int
	if measure {
		m = 1
	}
	r := C.go_node_encode(n.ptr, f.config, bytePointer(output), C.size_t(len(output)), m)
	if uint64(r.size) > uint64(^uint(0)>>1) {
		return 0, NativeSize
	}
	return int(r.size), Code(r.code)
}

type documentVisitor struct {
	callback   func(Node) int
	panicValue any
}

//export goDocumentVisitHandle
func goDocumentVisitHandle(handle C.uintptr_t, node unsafe.Pointer) (action C.int) {
	state := cgo.Handle(handle).Value().(*documentVisitor)
	action = C.int(C.TLV_VISIT_ERROR)
	defer func() {
		if value := recover(); value != nil {
			state.panicValue = value
		}
	}()
	value := state.callback(Node{node})
	if value < int(C.TLV_VISIT_CONTINUE) || value > int(C.TLV_VISIT_ERROR) {
		return -1
	}
	return C.int(value)
}

func (q *ProgramExecution) VisitDocument(callback func(Node) int) (Code, ProgramDiagnostic) {
	state := &documentVisitor{callback: callback}
	handle := cgo.NewHandle(state)
	defer handle.Delete()
	defer runtime.KeepAlive(q)
	var d C.tlv_query_diagnostic_t
	code := Code(C.go_query_document_visit(q.ptr, C.uintptr_t(handle), &d))
	if state.panicValue != nil {
		panic(state.panicValue)
	}
	return code, programDiagnostic(d, code)
}
