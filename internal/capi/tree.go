// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include "bridge.h"
*/
import "C"

import "runtime"

// Begin validates an opening parent using the canonical Tree Writer.
func (f Format) Begin(tag []byte) (Code, Diagnostic) {
	r := C.go_tree(f.config, nil, 0, bytePointer(tag), C.size_t(len(tag)), nil, 0, nil, 0)
	runtime.KeepAlive(tag)
	return Code(r.code), writerDiagnostic(r, tag)
}

// End closes a staged parent through the canonical Tree Writer.
func (f Format) End(output, tag, value []byte) (int, Code, Diagnostic) {
	if overlaps(output, tag) || overlaps(output, value) {
		return 0, InvalidArg, Diagnostic{Code: InvalidArg}
	}
	scratch := make([]byte, len(value))
	r := C.go_tree(f.config, bytePointer(output), C.size_t(len(output)), bytePointer(tag),
		C.size_t(len(tag)), bytePointer(value), C.size_t(len(value)), bytePointer(scratch), 1)
	runtime.KeepAlive(output)
	runtime.KeepAlive(tag)
	runtime.KeepAlive(value)
	runtime.KeepAlive(scratch)
	if uint64(r.size) > uint64(^uint(0)>>1) {
		return 0, NativeSize, Diagnostic{Code: NativeSize}
	}
	return int(r.size), Code(r.code), writerDiagnostic(r, tag)
}
