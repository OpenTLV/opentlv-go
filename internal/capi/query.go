// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include "bridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"runtime"
	"strings"
	"unsafe"
)

// Query delegates parsing and document traversal to C and retains no Go pointers.
func (d *Document) Query(text string) ([]Node, Code, Diagnostic) {
	if i := strings.IndexByte(text, 0); i >= 0 {
		return nil, Syntax, Diagnostic{Code: Syntax, Location: Location{3, 2, uint64(i), uint64(i + 1)}, Offset: OptionalSize{Value: uint64(i), Present: true}}
	}
	s := C.CString(text)
	defer C.free(unsafe.Pointer(s))
	defer runtime.KeepAlive(d)
	var nodes *unsafe.Pointer
	var count C.size_t
	var nativeDetail C.tlv_diagnostic_t
	code := Code(C.go_document_query(d.ptr, s, &nodes, &count, &nativeDetail))
	defer C.go_query_free(nodes)
	detail := diagnostic(nativeDetail, 0)
	if code != OK {
		return nil, code, detail
	}
	if uint64(count) > uint64(^uint(0)>>1) {
		return nil, NativeSize, Diagnostic{}
	}
	result := make([]Node, int(count))
	for i, p := range unsafe.Slice(nodes, int(count)) {
		result[i] = Node{p}
	}
	return result, OK, Diagnostic{}
}
