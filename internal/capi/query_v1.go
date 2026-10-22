// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include <tlv/query/query.h>
#include <stdlib.h>
static int path_matcher_feed(tlv_query_matcher_t* matcher, const uint8_t* data,
                              size_t size, size_t depth) {
    tlv_tag_t tag = {data, size};
    return tlv_query_matcher_visit(matcher, &tag, depth);
}
*/
import "C"
import (
	"bytes"
	"runtime"
	"sync"
	"unsafe"
)

// PathQuery contains self-contained native storage, never C pointers into Go.
type PathQuery struct{ raw C.tlv_query_t }

func ParsePath(text string) (*PathQuery, Code, Diagnostic) {
	query := &PathQuery{}
	input := []byte(text)
	// C rejects empty input; a nonnull pointer keeps its status SYNTAX.
	if len(input) == 0 {
		input = []byte{0}
	}
	var detail C.tlv_diagnostic_t
	code := Code(C.tlv_query_parse_n((*C.char)(unsafe.Pointer(bytePointer(input))), C.size_t(len(text)), &query.raw, &detail))
	runtime.KeepAlive(input)
	if code != OK {
		return nil, code, diagnostic(detail, 0)
	}
	return query, OK, Diagnostic{}
}

func (q *PathQuery) Count() int {
	if q == nil {
		return 0
	}
	return int(C.tlv_query_count(&q.raw))
}
func (q *PathQuery) Step(index int) []byte {
	if q == nil || index < 0 || index >= q.Count() {
		return nil
	}
	tag := C.tlv_query_step(&q.raw, C.size_t(index))
	value := bytes.Clone(nativeBytes(tag.data, tag.size))
	runtime.KeepAlive(q)
	return value
}
func (q *PathQuery) Format() (string, Code) {
	if q == nil {
		return "", InvalidArg
	}
	var required C.size_t
	code := Code(C.tlv_query_format(&q.raw, nil, 0, &required))
	if code != OK {
		return "", code
	}
	output := make([]byte, int(required)) // Native V1 storage bounds this to 1089 bytes.
	code = Code(C.tlv_query_format(&q.raw, (*C.char)(unsafe.Pointer(bytePointer(output))), required, &required))
	if code != OK {
		return "", code
	}
	return string(output[:len(output)-1]), OK
}

// PathMatcher serializes access to the stable C query and continuation storage.
type PathMatcher struct {
	mu      sync.Mutex
	query   *C.tlv_query_t
	matcher *C.tlv_query_matcher_t
}

func (q *PathQuery) Matcher() (*PathMatcher, Code) {
	if q == nil {
		return nil, InvalidArg
	}
	m := &PathMatcher{query: (*C.tlv_query_t)(C.malloc(C.sizeof_tlv_query_t)), matcher: (*C.tlv_query_matcher_t)(C.malloc(C.sizeof_tlv_query_matcher_t))}
	if m.query == nil || m.matcher == nil {
		m.Close()
		return nil, OutOfMemory
	}
	*m.query = q.raw
	code := Code(C.tlv_query_matcher_init(m.matcher, m.query))
	if code != OK {
		m.Close()
		return nil, code
	}
	runtime.SetFinalizer(m, (*PathMatcher).Close)
	return m, OK
}
func (m *PathMatcher) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	C.free(unsafe.Pointer(m.matcher))
	C.free(unsafe.Pointer(m.query))
	m.matcher, m.query = nil, nil
	runtime.SetFinalizer(m, nil)
}
func (m *PathMatcher) Feed(tag []byte, depth int) (bool, Code) {
	if m == nil || depth < 0 {
		return false, InvalidArg
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.matcher == nil {
		return false, InvalidArg
	}
	matched := C.path_matcher_feed(m.matcher, bytePointer(tag), C.size_t(len(tag)), C.size_t(depth)) != 0
	runtime.KeepAlive(tag)
	runtime.KeepAlive(m)
	return matched, OK
}
func (m *PathMatcher) Reset() Code {
	if m == nil {
		return InvalidArg
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.matcher == nil {
		return InvalidArg
	}
	code := Code(C.tlv_query_matcher_init(m.matcher, m.query))
	runtime.KeepAlive(m)
	return code
}
func (m *PathMatcher) Rebind(query *PathQuery) Code {
	if m == nil || query == nil {
		return InvalidArg
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.matcher == nil {
		return InvalidArg
	}
	copy := (*C.tlv_query_t)(C.malloc(C.sizeof_tlv_query_t))
	if copy == nil {
		return OutOfMemory
	}
	*copy = query.raw
	code := Code(C.tlv_query_matcher_rebind(m.matcher, copy))
	if code == OK {
		C.free(unsafe.Pointer(m.query))
		m.query = copy
	} else {
		C.free(unsafe.Pointer(copy))
	}
	runtime.KeepAlive(m)
	return code
}
