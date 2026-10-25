// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include "bridge.h"
#include <stdlib.h>
static tlv_schema_detail_t structure_schema(tlv_schema_diagnostic_t d) { return d.detail.schema; }
static tlv_reader_detail_t structure_reader(tlv_schema_diagnostic_t d) { return d.detail.reader; }
static tlv_codec_detail_t structure_codec(tlv_schema_diagnostic_t d) { return d.detail.codec; }
*/
import "C"
import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

type StructureRule struct {
	Tag                                                               []byte
	Name                                                              string
	MinLength, MaxLength, Multiple, MinOccurs, MaxOccurs, Kind, Child int
	Endpoints                                                         bool
	Group                                                             uint32
}
type StructureGroup struct {
	ID                   uint32
	Name                 string
	MinOccurs, MaxOccurs int
}
type StructureTable struct {
	Rules        []StructureRule
	Groups       []StructureGroup
	AllowUnknown bool
	Order        int
}
type StructureDiagnostic struct {
	Diagnostic Diagnostic
	Cause      int
	Schema     *CodecSchemaDetail
	Reader     *Diagnostic
	Codec      *CodecDetail
	Table      OptionalSize
}

// All native pointers refer only to separately allocated C storage. A single
// owner frees the graph without recursion, including cycles and shared tables.
type StructureSchema struct {
	mutex       sync.Mutex
	allocations []unsafe.Pointer
	tables      []C.tlv_structure_schema_t
	checked     C.tlv_schema_checked_t
}

func schemaSize(n int) C.size_t {
	if n == -1 {
		return ^C.size_t(0)
	}
	return C.size_t(n)
}
func ValidateSchemaLength(min, max, multiple int, endpoints bool, length int) Code {
	if min < 0 || max < -1 || multiple < 0 || length < 0 {
		return InvalidArg
	}
	entry := C.tlv_schema_entry_t{min_length: C.size_t(min), max_length: schemaSize(max), length_multiple: C.size_t(multiple)}
	if endpoints {
		entry.flags = C.TLV_SCHEMA_LENGTH_ENDPOINTS
	}
	return Code(C.tlv_schema_validate_length(&entry, C.size_t(length)))
}

func (s *StructureSchema) allocate(count, width uintptr) unsafe.Pointer {
	if count == 0 {
		return nil
	}
	if count > uintptr(^uint(0)>>1)/width {
		return nil
	}
	p := C.calloc(C.size_t(count), C.size_t(width))
	if p != nil {
		s.allocations = append(s.allocations, p)
	}
	return p
}
func (s *StructureSchema) text(value string) *C.char {
	p := s.allocate(uintptr(len(value))+1, 1)
	if p != nil {
		copy(unsafe.Slice((*byte)(p), len(value)), value)
	}
	return (*C.char)(p)
}

func PrepareStructure(definitions []StructureTable) (result *StructureSchema, code Code, detail StructureDiagnostic) {
	if len(definitions) == 0 || len(definitions) > 256 {
		return nil, InvalidArg, detail
	}
	s := &StructureSchema{}
	defer func() {
		if result == nil {
			s.Close()
		}
	}()
	p := s.allocate(uintptr(len(definitions)), C.sizeof_tlv_structure_schema_t)
	if p == nil {
		return nil, OutOfMemory, detail
	}
	s.tables = unsafe.Slice((*C.tlv_structure_schema_t)(p), len(definitions))
	for i, table := range definitions {
		t := &s.tables[i]
		if int(C.tlv_schema_order_t(table.Order)) != table.Order {
			return nil, InvalidArg, detail
		}
		t.order = C.tlv_schema_order_t(table.Order)
		if table.AllowUnknown {
			t.allow_unknown = 1
		}
		rules := s.allocate(uintptr(len(table.Rules)), C.sizeof_tlv_structure_rule_t)
		entries := s.allocate(uintptr(len(table.Rules)), C.sizeof_tlv_schema_entry_t)
		groups := s.allocate(uintptr(len(table.Groups)), C.sizeof_tlv_structure_group_t)
		if (len(table.Rules) > 0 && (rules == nil || entries == nil)) || (len(table.Groups) > 0 && groups == nil) {
			return nil, OutOfMemory, detail
		}
		t.rules = (*C.tlv_structure_rule_t)(rules)
		t.count = C.size_t(len(table.Rules))
		t.groups = (*C.tlv_structure_group_t)(groups)
		t.group_count = C.size_t(len(table.Groups))
		for j, rule := range table.Rules {
			if rule.MinLength < 0 || rule.MaxLength < -1 || rule.Multiple < 0 || rule.MinOccurs < 0 || rule.MaxOccurs < -1 || strings.IndexByte(rule.Name, 0) >= 0 || rule.Child < -1 || rule.Child >= len(definitions) || int(C.tlv_schema_kind_t(rule.Kind)) != rule.Kind {
				return nil, InvalidArg, detail
			}
			e := &unsafe.Slice((*C.tlv_schema_entry_t)(entries), len(table.Rules))[j]
			tag := s.allocate(uintptr(len(rule.Tag)), 1)
			if len(rule.Tag) > 0 && tag == nil {
				return nil, OutOfMemory, detail
			}
			copy(unsafe.Slice((*byte)(tag), len(rule.Tag)), rule.Tag)
			e.tag = C.tlv_tag_t{data: (*C.uint8_t)(tag), size: C.size_t(len(rule.Tag))}
			e.name = s.text(rule.Name)
			if e.name == nil {
				return nil, OutOfMemory, detail
			}
			e.min_length = C.size_t(rule.MinLength)
			e.max_length = schemaSize(rule.MaxLength)
			e.length_multiple = C.size_t(rule.Multiple)
			if rule.Endpoints {
				e.flags = C.TLV_SCHEMA_LENGTH_ENDPOINTS
			}
			r := &unsafe.Slice((*C.tlv_structure_rule_t)(rules), len(table.Rules))[j]
			r.entry = e
			r.min_occurs = C.size_t(rule.MinOccurs)
			r.max_occurs = schemaSize(rule.MaxOccurs)
			r.kind = C.tlv_schema_kind_t(rule.Kind)
			r.group = C.uint32_t(rule.Group)
			if rule.Child >= 0 {
				r.children = &s.tables[rule.Child]
			}
		}
		for j, group := range table.Groups {
			if group.MinOccurs < 0 || group.MaxOccurs < -1 || strings.IndexByte(group.Name, 0) >= 0 {
				return nil, InvalidArg, detail
			}
			g := &unsafe.Slice((*C.tlv_structure_group_t)(groups), len(table.Groups))[j]
			g.id = C.uint32_t(group.ID)
			g.min_occurs = C.size_t(group.MinOccurs)
			g.max_occurs = schemaSize(group.MaxOccurs)
			g.name = s.text(group.Name)
			if g.name == nil {
				return nil, OutOfMemory, detail
			}
		}
	}
	var d C.tlv_schema_diagnostic_t
	code = Code(C.tlv_schema_prepare(&s.checked, &s.tables[0], &d))
	detail = s.diagnostic(d, code)
	if code != OK {
		return nil, code, detail
	}
	runtime.SetFinalizer(s, (*StructureSchema).Close)
	return s, OK, detail
}

func (s *StructureSchema) Close() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.checked.schema = nil
	s.tables = nil
	for _, p := range s.allocations {
		C.free(p)
	}
	s.allocations = nil
	runtime.SetFinalizer(s, nil)
}

func (s *StructureSchema) diagnostic(d C.tlv_schema_diagnostic_t, code Code) StructureDiagnostic {
	result := StructureDiagnostic{Diagnostic: diagnostic(d.diagnostic, 0), Cause: int(d.cause)}
	result.Diagnostic.Code = code
	switch d.cause {
	case C.TLV_SCHEMA_CAUSE_SCHEMA:
		v := C.structure_schema(d)
		result.Schema = &CodecSchemaDetail{Kind: int(v.kind), DefinitionKind: int(v.definition.kind), DefinitionIndex: uint64(v.definition.index), Tag: bytes.Clone(nativeBytes(v.tag.data, v.tag.size)), Field: C.GoString(v.field), IsGroup: v.is_group != 0, HasOccurs: v.has_occurs != 0, HasLength: v.has_length != 0, HasForm: v.has_form != 0, MinOccurs: uint64(v.min_occurs), MaxOccurs: uint64(v.max_occurs), Occurs: uint64(v.occurs), MinLength: uint64(v.min_length), MaxLength: uint64(v.max_length), ActualLength: uint64(v.actual_length), ExpectedForm: int(v.expected_form), ActualConstructed: v.actual_constructed != 0, LengthMultiple: uint64(v.length_multiple), LengthFlags: uint32(v.length_flags)}
		for i := range s.tables {
			if unsafe.Pointer(&s.tables[i]) == v.definition.owner {
				result.Table = OptionalSize{uint64(i), true}
				break
			}
		}
	case C.TLV_SCHEMA_CAUSE_READER:
		v := readerDiagnosticParts(d.diagnostic, C.structure_reader(d), code)
		result.Reader = &v
	case C.TLV_SCHEMA_CAUSE_CODEC:
		result.Codec = codecDetail(d.diagnostic, C.structure_codec(d))
	}
	return result
}

func (s *StructureSchema) Validate(data []byte, f Format, depth, elements, unknown, capacity int) ([]StructureDiagnostic, uint64, Code, StructureDiagnostic) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	defer runtime.KeepAlive(s)
	defer runtime.KeepAlive(f)
	defer runtime.KeepAlive(data)
	if s.checked.schema == nil {
		return nil, 0, InvalidState, StructureDiagnostic{}
	}
	if depth < 0 || elements < 0 || capacity < -1 || unknown < 0 || unknown > 2 {
		return nil, 0, InvalidArg, StructureDiagnostic{}
	}
	// C-owned report entries may borrow input after validation returns, while
	// diagnostic projection makes further cgo calls. Keep those bytes pinned
	// until the owned snapshots are complete and the native report is freed.
	var input runtime.Pinner
	if len(data) > 0 {
		input.Pin(unsafe.SliceData(data))
	}
	defer input.Unpin()
	var report C.tlv_schema_diagnostic_report_t
	var target *C.tlv_schema_diagnostic_report_t
	if capacity >= 0 {
		target = &report
		if uint64(capacity) > uint64(^uint(0)>>1)/C.sizeof_tlv_schema_diagnostic_t {
			return nil, 0, Overflow, StructureDiagnostic{}
		}
		if capacity > 0 {
			p := C.calloc(C.size_t(capacity), C.sizeof_tlv_schema_diagnostic_t)
			if p == nil {
				return nil, 0, OutOfMemory, StructureDiagnostic{}
			}
			defer C.free(p)
			report.diagnostics = (*C.tlv_schema_diagnostic_t)(p)
		}
		report.capacity = C.size_t(capacity)
	}
	var d C.tlv_schema_diagnostic_t
	code := Code(C.go_structure_validate(&s.checked, f.config, bytePointer(data), C.size_t(len(data)), C.size_t(depth), C.size_t(elements), C.int(unknown), target, &d))
	detail := s.diagnostic(d, code)
	if code != OK && code != Schema {
		return nil, 0, code, detail
	}
	var findings []StructureDiagnostic
	count := report.count
	stored := count
	if stored > report.capacity {
		stored = report.capacity
	}
	for _, finding := range unsafe.Slice(report.diagnostics, int(stored)) {
		findings = append(findings, s.diagnostic(finding, Schema))
	}
	return findings, uint64(count), code, detail
}
