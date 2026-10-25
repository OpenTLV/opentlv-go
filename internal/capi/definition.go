// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include "tlv/definition.h"

static tlv_definition_registry_t* go_definitions_new(size_t count) {
    if (count > SIZE_MAX / sizeof(tlv_definition_t)) return NULL;
    tlv_definition_registry_t* registry = calloc(1, sizeof(*registry));
    if (!registry) return NULL;
    if (count) {
        registry->entries = calloc(count, sizeof(tlv_definition_t));
        if (!registry->entries) { free(registry); return NULL; }
    }
    registry->count = count;
    return registry;
}
static void go_definitions_free(tlv_definition_registry_t* registry) {
    if (!registry) return;
    for (size_t i = 0; i < registry->count; ++i)
        free((void*)registry->entries[i].tag.data);
    free((void*)registry->entries);
    free(registry);
}
static int go_definitions_set(tlv_definition_registry_t* registry, size_t index,
                              const uint8_t* data, size_t size) {
    tlv_definition_t* entry = (tlv_definition_t*)&registry->entries[index];
    if (size) {
        uint8_t* owned = malloc(size);
        if (!owned) return 0;
        memcpy(owned, data, size);
        entry->tag.data = owned;
    }
    entry->tag.size = size;
    return 1;
}
static size_t go_definitions_find(const tlv_definition_registry_t* registry,
                                  const uint8_t* data, size_t size) {
    tlv_tag_t tag = {data, size};
    const tlv_definition_t* entry = tlv_definition_find(registry, &tag);
    return entry ? (size_t)(entry - registry->entries) : registry->count;
}
*/
import "C"

import "runtime"

type definitionOwner struct{ ptr *C.tlv_definition_registry_t }
type DefinitionRegistry struct{ owner *definitionOwner }

func NewDefinitionRegistry(tags [][]byte) (DefinitionRegistry, Code) {
	ptr := C.go_definitions_new(C.size_t(len(tags)))
	if ptr == nil {
		return DefinitionRegistry{}, OutOfMemory
	}
	for i, tag := range tags {
		ok := C.go_definitions_set(ptr, C.size_t(i), bytePointer(tag), C.size_t(len(tag)))
		runtime.KeepAlive(tag)
		if ok == 0 {
			C.go_definitions_free(ptr)
			return DefinitionRegistry{}, OutOfMemory
		}
	}
	owner := &definitionOwner{ptr: ptr}
	runtime.SetFinalizer(owner, func(o *definitionOwner) { C.go_definitions_free(o.ptr) })
	return DefinitionRegistry{owner: owner}, OK
}

func (r DefinitionRegistry) Find(tag []byte) int {
	if r.owner == nil {
		return -1
	}
	index := C.go_definitions_find(r.owner.ptr, bytePointer(tag), C.size_t(len(tag)))
	count := r.owner.ptr.count
	runtime.KeepAlive(tag)
	runtime.KeepAlive(r.owner)
	if index == count {
		return -1
	}
	return int(index)
}
