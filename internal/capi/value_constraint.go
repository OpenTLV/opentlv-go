// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include <tlv/schema/constraint.h>
#include <stdlib.h>
*/
import "C"
import (
	"strings"
	"unsafe"
)

type ValueConstraintConfig struct {
	Kind     int
	Min, Max int64
	Values   []int64
	Names    []string
}

func (c ValueConstraintConfig) Call(operation int, value int64) (Code, string, bool) {
	if int(C.tlv_value_constraint_kind_t(c.Kind)) != c.Kind || (c.Names != nil && len(c.Names) != len(c.Values)) {
		return InvalidArg, "", false
	}
	for _, name := range c.Names {
		if strings.IndexByte(name, 0) >= 0 {
			return InvalidArg, "", false
		}
	}
	// Reuse the checked C-allocation owner; no schema is prepared or retained.
	owner := &StructureSchema{}
	defer owner.Close()
	n := C.tlv_value_constraint_t{kind: C.tlv_value_constraint_kind_t(c.Kind), min_value: C.int64_t(c.Min), max_value: C.int64_t(c.Max), allowed_values_count: C.size_t(len(c.Values))}
	if len(c.Values) > 0 {
		p := owner.allocate(uintptr(len(c.Values)), C.sizeof_int64_t)
		if p == nil {
			return OutOfMemory, "", false
		}
		n.allowed_values = (*C.int64_t)(p)
		for i, v := range c.Values {
			unsafe.Slice(n.allowed_values, len(c.Values))[i] = C.int64_t(v)
		}
	}
	if len(c.Names) > 0 && operation == 2 {
		p := owner.allocate(uintptr(len(c.Names)), unsafe.Sizeof((*C.char)(nil)))
		if p == nil {
			return OutOfMemory, "", false
		}
		n.allowed_value_names = (**C.char)(p)
		for i, name := range c.Names {
			ptr := owner.text(name)
			if ptr == nil {
				return OutOfMemory, "", false
			}
			unsafe.Slice(n.allowed_value_names, len(c.Names))[i] = ptr
		}
	}
	switch operation {
	case 0:
		return Code(C.tlv_value_constraint_check(&n)), "", false
	case 1:
		return Code(C.tlv_value_constraint_validate(&n, C.int64_t(value))), "", false
	default:
		name := C.tlv_value_constraint_name(&n, C.int64_t(value))
		return OK, C.GoString(name), name != nil
	}
}
