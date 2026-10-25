// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include "bridge.h"
*/
import "C"
import (
	"runtime"
	"unsafe"
)

// StructureReady permits nil (framing-only) and checks shared schema lifecycle.
func StructureReady(s *StructureSchema) Code {
	if s == nil {
		return OK
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.checked.schema == nil {
		return InvalidState
	}
	return OK
}

func StructureConvert(s *StructureSchema, f Format, data []byte, depth, elements int, encode bool) ([]byte, Code, *CodecDetail) {
	var schema *C.tlv_structure_schema_t
	if s != nil {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		defer runtime.KeepAlive(s)
		if s.checked.schema == nil {
			return nil, InvalidState, nil
		}
		schema = s.checked.schema
	}
	if depth < 0 || elements < 0 {
		return nil, InvalidArg, nil
	}
	defer runtime.KeepAlive(f)
	defer runtime.KeepAlive(data)
	// Failure evidence may borrow tags from input/output until projected below.
	var pinned runtime.Pinner
	if len(data) > 0 {
		pinned.Pin(unsafe.SliceData(data))
	}
	defer pinned.Unpin()
	var output []byte
	mode := 0
	if encode {
		mode = 1
		output = make([]byte, len(data))
		if len(output) > 0 {
			pinned.Pin(unsafe.SliceData(output))
		}
	}
	var d C.tlv_codec_diagnostic_t
	code := Code(C.go_structure_convert(f.config, schema, C.size_t(depth), C.size_t(elements), bytePointer(data), C.size_t(len(data)), bytePointer(output), C.int(mode), &d))
	if code != OK {
		return nil, code, codecDetail(d.diagnostic, d.codec)
	}
	return output, OK, nil
}
