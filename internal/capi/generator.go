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

const GeneratorVersion = C.TLV_GENERATOR_VERSION

type GeneratorCandidate struct {
	Tag      []byte
	Min, Max int
}
type GeneratorConfig struct {
	Seed                                                               uint64
	MaxElements, MaxDepth, MaxValueSize, MaxCaseSize, MaxWorkspaceSize int
	Candidates                                                         []GeneratorCandidate
}

func (c *GeneratorConfig) native(owner *StructureSchema) (C.tlv_generator_options_t, int, Code) {
	var n C.tlv_generator_options_t
	if c.MaxElements < 0 || c.MaxDepth < 0 || c.MaxValueSize < 0 || c.MaxCaseSize < 0 || c.MaxWorkspaceSize <= 0 {
		return n, 0, InvalidArg
	}
	for _, v := range c.Candidates {
		if v.Min < 0 || v.Max < 0 {
			return n, 0, InvalidArg
		}
	}
	n = C.tlv_generator_options_t{seed: C.uint64_t(c.Seed), max_elements: C.size_t(c.MaxElements), max_depth: C.size_t(c.MaxDepth), max_value_size: C.size_t(c.MaxValueSize), max_case_size: C.size_t(c.MaxCaseSize), candidate_count: C.size_t(len(c.Candidates))}
	if len(c.Candidates) > 0 {
		p := owner.allocate(uintptr(len(c.Candidates)), C.sizeof_tlv_generator_candidate_t)
		if p == nil {
			return n, 0, OutOfMemory
		}
		n.candidates = (*C.tlv_generator_candidate_t)(p)
		for i, v := range c.Candidates {
			var tag unsafe.Pointer
			if v.Tag != nil {
				size := len(v.Tag)
				if size == 0 {
					size = 1
				}
				tag = owner.allocate(uintptr(size), 1)
				if tag == nil {
					return n, 0, OutOfMemory
				}
				copy(unsafe.Slice((*byte)(tag), len(v.Tag)), v.Tag)
			}
			unsafe.Slice(n.candidates, len(c.Candidates))[i] = C.tlv_generator_candidate_t{tag: C.tlv_tag_t{data: (*C.uint8_t)(tag), size: C.size_t(len(v.Tag))}, min_value_size: C.size_t(v.Min), max_value_size: C.size_t(v.Max)}
		}
	}
	var size C.size_t
	code := Code(C.tlv_generator_workspace_size(&n, &size))
	if code != OK {
		return n, 0, code
	}
	if uint64(size) > uint64(^uint(0)>>1) {
		return n, 0, NativeSize
	}
	if uint64(size) > uint64(c.MaxWorkspaceSize) {
		return n, 0, Limit
	}
	return n, int(size), OK
}

func (c *GeneratorConfig) WorkspaceSize() (int, Code) {
	owner := &StructureSchema{}
	defer owner.Close()
	_, size, code := c.native(owner)
	return size, code
}

func (c *GeneratorConfig) Generate(f Format, index uint64) ([]byte, Code) {
	owner := &StructureSchema{}
	defer owner.Close()
	n, size, code := c.native(owner)
	if code != OK {
		return nil, code
	}
	n.case_index = C.uint64_t(index)
	output := make([]byte, c.MaxCaseSize)
	workspace := make([]byte, size)
	var written C.size_t
	code = Code(C.go_generate(f.config, &n, bytePointer(output), C.size_t(len(output)), bytePointer(workspace), C.size_t(len(workspace)), &written))
	runtime.KeepAlive(f)
	runtime.KeepAlive(output)
	runtime.KeepAlive(workspace)
	if code != OK {
		return nil, code
	}
	if uint64(written) > uint64(len(output)) {
		return nil, Callback
	}
	return output[:int(written):int(written)], OK
}
