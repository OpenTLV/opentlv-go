// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include "bridge.h"
#include <stdlib.h>
#if OPENTLV_EMV
#include <tlv/builtins/emv/query.h>
#endif
static tlv_result_t query_emv_resolve(const char* ns, size_t ns_size, const char* name,
                                     size_t name_size, tlv_tag_t* tag) {
#if OPENTLV_EMV
    return tlv_emv_query_resolve(NULL, ns, ns_size, name, name_size, tag);
#else
    (void)ns; (void)ns_size; (void)name; (void)name_size; (void)tag;
    return TLV_ERR_UNSUPPORTED;
#endif
}
*/
import "C"
import (
	"bytes"
	"runtime/cgo"
	"strings"
	"unsafe"
)

type QueryTagAdapter struct {
	ID            uint32
	Class, Number func([]byte) (int64, Code)
}
type QueryResolver func(string, string) ([]byte, Code)
type resolverCall struct {
	resolve QueryResolver
	data    unsafe.Pointer
}

//export goQueryTagLookup
func goQueryTagLookup(handle C.uintptr_t, kind C.int, data *C.uint8_t, size C.size_t, value *C.int64_t) (code C.int) {
	code = C.TLV_ERR_INVALID_VALUE
	defer func() {
		if recover() != nil {
			code = C.TLV_ERR_INVALID_VALUE
		}
	}()
	provider := cgo.Handle(handle).Value().(QueryTagAdapter)
	callback := provider.Class
	if kind != 0 {
		callback = provider.Number
	}
	if callback == nil {
		return C.TLV_ERR_UNSUPPORTED
	}
	result, status := callback(bytes.Clone(nativeBytes(data, size)))
	if status == OK {
		*value = C.int64_t(result)
	}
	return C.int(status)
}

//export goQueryResolve
func goQueryResolve(handle C.uintptr_t, namespace *C.char, namespaceSize C.size_t, name *C.char, nameSize C.size_t, tag *C.tlv_tag_t) (code C.int) {
	code = C.TLV_ERR_INVALID_VALUE
	defer func() {
		if recover() != nil {
			code = C.TLV_ERR_INVALID_VALUE
		}
	}()
	state := cgo.Handle(handle).Value().(*resolverCall)
	ns := string(nativeBytes((*C.uint8_t)(unsafe.Pointer(namespace)), namespaceSize))
	label := string(nativeBytes((*C.uint8_t)(unsafe.Pointer(name)), nameSize))
	value, status := state.resolve(ns, label)
	if status != OK {
		return C.int(status)
	}
	C.free(state.data)
	state.data = C.CBytes(value)
	if len(value) != 0 && state.data == nil {
		return C.TLV_ERR_OUT_OF_MEMORY
	}
	tag.data = (*C.uint8_t)(state.data)
	tag.size = C.size_t(len(value))
	return C.TLV_OK
}

type QueryDefinition struct {
	Namespace, Name string
	Tag             []byte
}

func ResolveEMV(namespace, name string) ([]byte, Code) {
	ns, label := C.CString(namespace), C.CString(name)
	defer C.free(unsafe.Pointer(ns))
	defer C.free(unsafe.Pointer(label))
	var tag C.tlv_tag_t
	code := Code(C.query_emv_resolve(ns, C.size_t(len(namespace)), label, C.size_t(len(name)), &tag))
	if code != OK {
		return nil, code
	}
	return bytes.Clone(nativeBytes(tag.data, tag.size)), OK
}

// ResolveDefinitions adapts owned records to the canonical C ambiguity rules.
func ResolveDefinitions(definitions []QueryDefinition, namespace, name string) ([]byte, Code) {
	var allocations []unsafe.Pointer
	defer func() {
		for _, pointer := range allocations {
			C.free(pointer)
		}
	}()
	allocate := func(count int, size uintptr) unsafe.Pointer {
		pointer := C.calloc(C.size_t(count+1), C.size_t(size))
		allocations = append(allocations, pointer)
		return pointer
	}
	scopesPointer := allocate(len(definitions), unsafe.Sizeof(C.tlv_query_definition_scope_t{}))
	registriesPointer := allocate(len(definitions), unsafe.Sizeof(C.tlv_definition_registry_t{}))
	entriesPointer := allocate(len(definitions), unsafe.Sizeof(C.tlv_definition_t{}))
	if scopesPointer == nil || registriesPointer == nil || entriesPointer == nil {
		return nil, OutOfMemory
	}
	scopes := unsafe.Slice((*C.tlv_query_definition_scope_t)(scopesPointer), len(definitions))
	registries := unsafe.Slice((*C.tlv_definition_registry_t)(registriesPointer), len(definitions))
	entries := unsafe.Slice((*C.tlv_definition_t)(entriesPointer), len(definitions))
	for index, definition := range definitions {
		if strings.IndexByte(definition.Namespace, 0) >= 0 || strings.IndexByte(definition.Name, 0) >= 0 {
			return nil, InvalidArg
		}
		ns, label, tag := C.CString(definition.Namespace), C.CString(definition.Name), C.CBytes(definition.Tag)
		allocations = append(allocations, unsafe.Pointer(ns), unsafe.Pointer(label), tag)
		if ns == nil || label == nil || (len(definition.Tag) != 0 && tag == nil) {
			return nil, OutOfMemory
		}
		entries[index].tag = C.tlv_tag_t{data: (*C.uint8_t)(tag), size: C.size_t(len(definition.Tag))}
		entries[index].name = label
		registries[index].entries = &entries[index]
		registries[index].count = 1
		scopes[index].namespace_name = ns
		scopes[index].definitions = &registries[index]
	}
	resolver := C.tlv_query_definition_resolver_t{scopes: (*C.tlv_query_definition_scope_t)(scopesPointer), count: C.size_t(len(definitions))}
	ns, label := C.CString(namespace), C.CString(name)
	allocations = append(allocations, unsafe.Pointer(ns), unsafe.Pointer(label))
	if ns == nil || label == nil {
		return nil, OutOfMemory
	}
	var tag C.tlv_tag_t
	code := Code(C.tlv_query_definition_resolve(unsafe.Pointer(&resolver), ns, C.size_t(len(namespace)), label, C.size_t(len(name)), &tag))
	if code != OK {
		return nil, code
	}
	return bytes.Clone(nativeBytes(tag.data, tag.size)), OK
}
