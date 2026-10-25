// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include "bridge.h"
#include <stdlib.h>
extern int goQueryCallback(uintptr_t handle, tlv_tree_event_t* event);
static tlv_visit_result_t query_callback(const tlv_tree_event_t* event, void* context) {
    return (tlv_visit_result_t)goQueryCallback((uintptr_t)context, (tlv_tree_event_t*)event);
}
static tlv_result_t query_visit_handle(go_query_execution* q, uintptr_t handle, tlv_query_diagnostic_t* d) {
    return go_query_visit(q, query_callback, (void*)handle, d);
}
static tlv_reader_detail_t query_reader(const tlv_query_diagnostic_t* d) { return d->detail.reader; }
static tlv_codec_detail_t query_codec(const tlv_query_diagnostic_t* d) { return d->detail.codec; }
static tlv_schema_detail_t query_schema(const tlv_query_diagnostic_t* d) { return d->detail.schema; }
*/
import "C"
import (
	"bytes"
	"runtime"
	"runtime/cgo"
	"sort"
	"strings"
	"unsafe"
)

// QueryErrorState is the native lifecycle and reentrancy diagnostic category.
const QueryErrorState = int(C.TLV_QUERY_ERROR_STATE)

// ProgramDiagnostic copies each Query field from exactly one C member. Common
// holds the common evidence, together with the Reader detail of a Reader cause.
type ProgramDiagnostic struct {
	Common      Diagnostic
	HasReader   bool
	Kind        int
	Expression  Location
	Configured  uint64
	Limit       string
	CodecDetail *CodecDetail
}

func programDiagnostic(d C.tlv_query_diagnostic_t, code Code) ProgramDiagnostic {
	hasReader := d.cause == C.TLV_QUERY_CAUSE_READER
	common := diagnostic(d.diagnostic, 0)
	if hasReader {
		common = readerDiagnosticParts(d.diagnostic, C.query_reader(&d), code)
	}
	return ProgramDiagnostic{Common: common, HasReader: hasReader, Kind: int(d.kind),
		Expression: Location{Domain: int(d.expression.domain), Kind: int(d.expression.kind),
			Begin: uint64(d.expression.begin), End: uint64(d.expression.end)},
		Configured: uint64(d.configured), Limit: C.GoString(d.limit), CodecDetail: queryCodecDetail(d)}
}

// ProgramOptions owns maps; C borrows only during bounded compilation.
type ProgramOptions struct {
	Variables                                                             map[string]int
	Names                                                                 map[string][]byte
	Providers                                                             map[int]QueryProvider
	Tags                                                                  *QueryTagAdapter
	Resolver                                                              QueryResolver
	Optimize                                                              bool
	MaxText, MaxTokens, MaxNesting, MaxStates, MaxPattern, MaxResolvedTag int
}
type QueryMetadata struct {
	Tag, Value    []byte
	Kind          int
	Depth, Offset uint64
}
type ProviderResult struct {
	Type    int
	Integer int64
	Text    string
}
type QueryProvider struct {
	ID       uint32
	Capacity int
	Decode   func([]byte, *QueryMetadata) (ProviderResult, int)
}

//export goQueryProviderRelease
func goQueryProviderRelease(handle C.uintptr_t) { cgo.Handle(handle).Delete() }

//export goQueryProviderDecode
func goQueryProviderDecode(handle C.uintptr_t, event *C.tlv_tree_event_t, data *C.uint8_t, size C.size_t,
	scratch unsafe.Pointer, capacity C.size_t, output *C.tlv_query_result_t) (code C.int) {
	code = C.TLV_ERR_CALLBACK
	defer func() {
		if recover() != nil {
			code = C.TLV_ERR_CALLBACK
		}
	}()
	provider := cgo.Handle(handle).Value().(QueryProvider)
	if uint64(size) > uint64(^uint(0)>>1) {
		return C.TLV_ERR_CALLBACK
	}
	input := bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(data)), int(size)))
	var metadata *QueryMetadata
	if event != nil {
		item := match(event)
		metadata = &QueryMetadata{Tag: item.Tag, Value: item.Value, Kind: int(event.kind), Depth: uint64(event.depth), Offset: uint64(event.offset)}
	}
	value, status := provider.Decode(input, metadata)
	if status != 0 {
		return C.int(status)
	}
	*output = C.tlv_query_result_t{}
	switch value.Type {
	case 2:
		output.kind = 2
		output.integer = C.int64_t(value.Integer)
	case 4:
		if uint64(len(value.Text)) > uint64(capacity) {
			return C.TLV_ERR_BUFFER_TOO_SHORT
		}
		copy(unsafe.Slice((*byte)(scratch), len(value.Text)), value.Text)
		output.kind = 4
		output.data = (*C.uint8_t)(scratch)
		output.size = C.size_t(len(value.Text))
	default:
		return C.TLV_ERR_CALLBACK
	}
	return 0
}

// Program owns a reference-counted C image. Existing executions survive Close.
type Program struct{ ptr *C.go_query_program }

func (p *Program) Retain() *Program { C.go_query_program_retain(p.ptr); return &Program{ptr: p.ptr} }

type QueryRule struct {
	Context, Assertion *Program
	Name               string
}
type QuerySchemaLimits struct{ Depth, Nodes, Work, Contexts, ValueCapacity int }
type QuerySchemaDiagnostic struct {
	Rule      int
	HasSchema bool
	Field     string
	Kind      int
	Tag       []byte
	Query     ProgramDiagnostic
}

// ValidateQuerySchema owns storage adaptation only; canonical C evaluates every rule.
func ValidateQuerySchema(rules []QueryRule, data []byte, document *Document, format Format,
	limits QuerySchemaLimits) (Code, QuerySchemaDiagnostic) {
	defer runtime.KeepAlive(format)
	if limits.Depth < 0 || limits.Nodes < 0 || limits.Work < 0 || limits.Contexts < 0 || limits.ValueCapacity < -1 {
		return InvalidArg, QuerySchemaDiagnostic{}
	}
	var allocations []unsafe.Pointer
	defer func() {
		for _, pointer := range allocations {
			C.free(pointer)
		}
	}()
	allocate := func(count int, element uintptr, aligned bool) unsafe.Pointer {
		maximum := ^uintptr(0)
		if count < 0 || uint64(count) > uint64((maximum-15)/element) {
			return nil
		}
		pointer := C.calloc(1, C.size_t(uintptr(count)*element+15))
		if pointer == nil {
			return nil
		}
		allocations = append(allocations, pointer)
		if aligned {
			return unsafe.Add(pointer, (16-uintptr(pointer)%16)%16)
		}
		return pointer
	}
	recordsPointer := allocate(len(rules), unsafe.Sizeof(C.tlv_schema_query_rule_t{}), false)
	if recordsPointer == nil {
		return OutOfMemory, QuerySchemaDiagnostic{}
	}
	records := unsafe.Slice((*C.tlv_schema_query_rule_t)(recordsPointer), len(rules))
	for index, rule := range rules {
		if strings.IndexByte(rule.Name, 0) >= 0 {
			return InvalidArg, QuerySchemaDiagnostic{}
		}
		name := C.CString(rule.Name)
		if name == nil {
			return OutOfMemory, QuerySchemaDiagnostic{}
		}
		allocations = append(allocations, unsafe.Pointer(name))
		records[index].context = C.go_query_native(rule.Context.ptr)
		records[index].assertion = C.go_query_native(rule.Assertion.ptr)
		records[index].environment = C.go_query_environment(rule.Assertion.ptr)
		records[index].name = name
	}
	var selector, assertion, alignment C.size_t
	code := Code(C.tlv_schema_query_size((*C.tlv_schema_query_rule_t)(recordsPointer), C.size_t(len(rules)),
		C.size_t(limits.Depth), C.size_t(limits.Nodes), &selector, &assertion, &alignment))
	if code != OK {
		return code, QuerySchemaDiagnostic{}
	}
	if uint64(selector) > uint64(^uint(0)>>1) || uint64(assertion) > uint64(^uint(0)>>1) || limits.Depth == int(^uint(0)>>1) {
		return Overflow, QuerySchemaDiagnostic{}
	}
	workspace := C.tlv_schema_query_workspace_t{}
	workspace.selector = allocate(int(selector), 1, true)
	workspace.selector_size = selector
	workspace.assertion = allocate(int(assertion), 1, true)
	workspace.assertion_size = assertion
	workspace.contexts = (*C.tlv_schema_query_context_t)(allocate(limits.Contexts, unsafe.Sizeof(C.tlv_schema_query_context_t{}), false))
	workspace.context_capacity = C.size_t(limits.Contexts)
	workspace.frames = (*C.tlv_tree_frame_t)(allocate(limits.Depth+1, unsafe.Sizeof(C.tlv_tree_frame_t{}), false))
	workspace.frame_capacity = C.size_t(limits.Depth + 1)
	if workspace.selector == nil || workspace.assertion == nil || workspace.contexts == nil || workspace.frames == nil {
		return OutOfMemory, QuerySchemaDiagnostic{}
	}
	var nativeDetail C.tlv_schema_query_diagnostic_t
	if document == nil {
		input := allocate(len(data), 1, false)
		if input == nil {
			return OutOfMemory, QuerySchemaDiagnostic{}
		}
		copy(unsafe.Slice((*byte)(input), len(data)), data)
		code = Code(C.go_query_schema_buffer(format.config, (*C.uint8_t)(input), C.size_t(len(data)),
			(*C.tlv_schema_query_rule_t)(recordsPointer), C.size_t(len(rules)), C.size_t(limits.Depth),
			C.size_t(limits.Nodes), C.size_t(limits.Work), &workspace, &nativeDetail))
	} else {
		capacity := limits.ValueCapacity
		if capacity < 0 {
			capacity, code = document.Encode(format, nil, true)
			if code != OK {
				return code, QuerySchemaDiagnostic{}
			}
		}
		staging := C.tlv_tree_writer_workspace_t{}
		staging.frames = (*C.tlv_tree_writer_frame_t)(allocate(limits.Depth+1, unsafe.Sizeof(C.tlv_tree_writer_frame_t{}), false))
		staging.frame_capacity = C.size_t(limits.Depth + 1)
		staging.data = (*C.uint8_t)(allocate(capacity, 1, false))
		staging.data_capacity = C.size_t(capacity)
		staging.scratch = (*C.uint8_t)(allocate(capacity, 1, false))
		staging.scratch_capacity = C.size_t(capacity)
		values := (*C.uint8_t)(allocate(capacity, 1, false))
		if staging.frames == nil || staging.data == nil || staging.scratch == nil || values == nil {
			return OutOfMemory, QuerySchemaDiagnostic{}
		}
		code = Code(C.go_query_schema_document(document.ptr, (*C.tlv_schema_query_rule_t)(recordsPointer),
			C.size_t(len(rules)), C.size_t(limits.Depth), C.size_t(limits.Nodes), C.size_t(limits.Work),
			&workspace, values, C.size_t(capacity), &staging, &nativeDetail))
	}
	detail := QuerySchemaDiagnostic{Rule: int(nativeDetail.rule), Query: programDiagnostic(nativeDetail.query, code)}
	if nativeDetail.query.cause == C.TLV_QUERY_CAUSE_SCHEMA {
		schema := C.query_schema(&nativeDetail.query)
		detail.HasSchema, detail.Field, detail.Kind = true, C.GoString(schema.field), int(schema.kind)
		detail.Tag = bytes.Clone(nativeBytes(schema.tag.data, schema.tag.size))
	}
	runtime.KeepAlive(rules)
	runtime.KeepAlive(document)
	return code, detail
}

// CompileProgram compiles bounded bytes or loads a same-release image copy.
func (f Format) CompileProgram(data []byte, options ProgramOptions, image bool) (*Program, Code, ProgramDiagnostic) {
	defer runtime.KeepAlive(f)
	var config C.tlv_query_compile_options_t
	C.tlv_query_compile_options_init(&config)
	values := []struct {
		input  int
		target *C.size_t
	}{
		{options.MaxText, &config.max_text}, {options.MaxTokens, &config.max_tokens},
		{options.MaxNesting, &config.max_nesting}, {options.MaxStates, &config.max_states},
		{options.MaxPattern, &config.max_pattern}, {options.MaxResolvedTag, &config.max_resolved_tag},
	}
	for _, value := range values {
		if value.input < 0 {
			return nil, InvalidArg, ProgramDiagnostic{}
		}
		if value.input > 0 {
			*value.target = C.size_t(value.input)
		}
	}
	if !options.Optimize {
		config.optimize = 0
	}
	keys := make([]string, 0, len(options.Variables))
	for key := range options.Variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var variables []C.tlv_query_variable_t
	if len(keys) > 0 {
		memory := C.calloc(C.size_t(len(keys)), C.size_t(C.sizeof_tlv_query_variable_t))
		if memory == nil {
			return nil, OutOfMemory, ProgramDiagnostic{}
		}
		defer C.free(memory)
		variables = unsafe.Slice((*C.tlv_query_variable_t)(memory), len(keys))
	}
	defer func() {
		for _, v := range variables {
			C.free(unsafe.Pointer(v.name))
		}
	}()
	for i, key := range keys {
		if strings.IndexByte(key, 0) >= 0 {
			return nil, InvalidArg, ProgramDiagnostic{}
		}
		variables[i].name = C.CString(key)
		variables[i]._type = C.tlv_query_result_kind_t(options.Variables[key])
	}
	if len(variables) > 0 {
		config.variables = &variables[0]
		config.variable_count = C.size_t(len(variables))
	}
	keys = keys[:0]
	for key := range options.Names {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var names []C.go_query_name
	if len(keys) > 0 {
		memory := C.calloc(C.size_t(len(keys)), C.size_t(C.sizeof_go_query_name))
		if memory == nil {
			return nil, OutOfMemory, ProgramDiagnostic{}
		}
		defer C.free(memory)
		names = unsafe.Slice((*C.go_query_name)(memory), len(keys))
	}
	defer func() {
		for _, n := range names {
			C.free(unsafe.Pointer(n.name))
			C.free(unsafe.Pointer(n.tag))
		}
	}()
	for i, key := range keys {
		if strings.IndexByte(key, 0) >= 0 {
			return nil, InvalidArg, ProgramDiagnostic{}
		}
		names[i].name = C.CString(key)
		names[i].tag = (*C.uint8_t)(C.CBytes(options.Names[key]))
		names[i].size = C.size_t(len(options.Names[key]))
	}
	var first *C.go_query_name
	if len(names) > 0 {
		first = &names[0]
	}
	var code C.tlv_result_t
	var diagnostic C.tlv_query_diagnostic_t
	var load C.int
	if image {
		load = 1
	}
	for selector, provider := range options.Providers {
		if selector < 0 || selector > 3 || provider.ID == 0 || provider.Capacity < 0 || provider.Decode == nil {
			return nil, InvalidArg, ProgramDiagnostic{}
		}
	}
	if options.Tags != nil && (options.Tags.ID == 0 || (options.Tags.Class == nil && options.Tags.Number == nil)) {
		return nil, InvalidArg, ProgramDiagnostic{}
	}
	var resolver C.uintptr_t
	if options.Resolver != nil {
		state := &resolverCall{resolve: options.Resolver}
		handle := cgo.NewHandle(state)
		resolver = C.uintptr_t(handle)
		defer func() { C.free(state.data); handle.Delete() }()
	}
	var providerFirst *C.go_query_provider
	if len(options.Providers) > 0 {
		memory := C.calloc(C.size_t(len(options.Providers)), C.size_t(unsafe.Sizeof(C.go_query_provider{})))
		if memory == nil {
			return nil, OutOfMemory, ProgramDiagnostic{}
		}
		defer C.free(memory)
		providers := unsafe.Slice((*C.go_query_provider)(memory), len(options.Providers))
		index := 0
		for selector, provider := range options.Providers {
			providers[index].id = C.uint32_t(provider.ID)
			providers[index].function = C.int(selector)
			providers[index].capacity = C.size_t(provider.Capacity)
			providers[index].handle = C.uintptr_t(cgo.NewHandle(provider))
			index++
		}
		providerFirst = &providers[0]
	}
	var tags *C.go_query_tags
	var nativeTags C.go_query_tags
	if options.Tags != nil {
		nativeTags.id = C.uint32_t(options.Tags.ID)
		nativeTags.handle = C.uintptr_t(cgo.NewHandle(*options.Tags))
		if options.Tags.Class != nil {
			nativeTags.has_class = 1
		}
		if options.Tags.Number != nil {
			nativeTags.has_number = 1
		}
		tags = &nativeTags
	}
	ptr := C.go_query_compile(f.config, bytePointer(data), C.size_t(len(data)), &config, first, C.size_t(len(names)), providerFirst, C.size_t(len(options.Providers)), tags, resolver, load, &code, &diagnostic)
	runtime.KeepAlive(data)
	runtime.KeepAlive(variables)
	runtime.KeepAlive(names)
	detail := programDiagnostic(diagnostic, Code(code))
	if Code(code) != OK {
		return nil, Code(code), detail
	}
	p := &Program{ptr}
	runtime.SetFinalizer(p, (*Program).Close)
	return p, OK, detail
}
func (p *Program) Close() {
	if p != nil && p.ptr != nil {
		C.go_query_program_free(p.ptr)
		p.ptr = nil
	}
	runtime.SetFinalizer(p, nil)
}

// Info returns copied compiler resource requirements.
func (p *Program) Info() map[string]uint64 {
	defer runtime.KeepAlive(p)
	i := C.go_query_info(p.ptr)
	return map[string]uint64{"program_size": uint64(i.program_size), "program_alignment": uint64(i.program_alignment),
		"scratch_size": uint64(i.scratch_size), "scratch_alignment": uint64(i.scratch_alignment), "states": uint64(i.states),
		"language_version": uint64(i.language_version), "level": uint64(i.level), "result_kind": uint64(i.result_kind),
		"expression_values": uint64(i.expression_values), "instructions": uint64(i.instructions), "variable_slots": uint64(i.variable_slots),
		"codec_scratch": uint64(i.codec_scratch), "pattern_bytes": uint64(i.pattern_bytes), "optimized_states": uint64(i.optimized_states),
		"expression_stack": uint64(i.expression_stack), "candidate_size": uint64(i.candidate_size), "candidate_alignment": uint64(i.candidate_alignment),
		"frame_states": uint64(i.frame_states), "decision_timing": uint64(i.decision_timing), "stable_input_required": uint64(i.stable_input_required),
		"constructed_values_required": uint64(i.constructed_values_required)}
}
func (p *Program) Render(explain bool) (string, Code) {
	defer runtime.KeepAlive(p)
	var size C.size_t
	raw := C.go_query_native(p.ptr)
	fn := func(output *C.char, capacity C.size_t) C.tlv_result_t {
		if explain {
			return C.tlv_query_program_explain(raw, output, capacity, &size)
		}
		return C.tlv_query_program_format(raw, output, capacity, &size)
	}
	if code := Code(fn(nil, 0)); code != OK {
		return "", code
	}
	output := C.malloc(size)
	if output == nil {
		return "", OutOfMemory
	}
	defer C.free(output)
	code := Code(fn((*C.char)(output), size))
	if code != OK {
		return "", code
	}
	return C.GoString((*C.char)(output)), OK
}
func (p *Program) Image() []byte {
	defer runtime.KeepAlive(p)
	i := C.go_query_info(p.ptr)
	return bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(C.go_query_native(p.ptr))), int(i.program_size)))
}
func (p *Program) Variables() (map[string]int, Code) {
	defer runtime.KeepAlive(p)
	raw := C.go_query_native(p.ptr)
	result := map[string]int{}
	for i, count := C.size_t(0), C.tlv_query_program_variable_count(raw); i < count; i++ {
		var info C.tlv_query_variable_info_t
		if code := Code(C.tlv_query_program_variable(raw, i, &info)); code != OK {
			return nil, code
		}
		result[string(unsafe.Slice((*byte)(unsafe.Pointer(info.name)), int(info.name_size)))] = int(info._type)
	}
	return result, OK
}

// ProgramExecution owns stable C descriptors, windows, bindings and bounded workspace.
type ProgramExecution struct{ ptr *C.go_query_execution }

func (p *Program) Execution(depth, nodes, work int, retained bool) (*ProgramExecution, Code) {
	if depth < 0 || nodes <= 0 || work <= 0 {
		return nil, InvalidArg
	}
	var flag C.int
	if retained {
		flag = 1
	}
	var code C.tlv_result_t
	ptr := C.go_query_execution_create(p.ptr, C.size_t(depth), C.size_t(nodes), C.size_t(work), flag, &code)
	runtime.KeepAlive(p)
	if Code(code) != OK {
		return nil, Code(code)
	}
	q := &ProgramExecution{ptr}
	runtime.SetFinalizer(q, (*ProgramExecution).Close)
	return q, OK
}
func (q *ProgramExecution) Close() {
	if q != nil && q.ptr != nil {
		C.go_query_execution_free(q.ptr)
		q.ptr = nil
	}
	runtime.SetFinalizer(q, nil)
}
func (q *ProgramExecution) Reset() Code {
	defer runtime.KeepAlive(q)
	return Code(C.go_query_execution_reset(q.ptr))
}
func (q *ProgramExecution) SetInput(data []byte, discard int, final bool) Code {
	if discard < 0 {
		return InvalidArg
	}
	var flag C.int
	if final {
		flag = 1
	}
	code := Code(C.go_query_input(q.ptr, bytePointer(data), C.size_t(len(data)), C.size_t(discard), flag))
	runtime.KeepAlive(data)
	runtime.KeepAlive(q)
	return code
}
func (q *ProgramExecution) Bind(name string, kind int, number int64, data []byte) (Code, ProgramDiagnostic) {
	if strings.IndexByte(name, 0) >= 0 {
		return InvalidArg, ProgramDiagnostic{}
	}
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	var d C.tlv_query_diagnostic_t
	code := Code(C.go_query_bind(q.ptr, n, C.tlv_query_result_kind_t(kind), C.int64_t(number), bytePointer(data), C.size_t(len(data)), &d))
	runtime.KeepAlive(data)
	runtime.KeepAlive(q)
	return code, programDiagnostic(d, code)
}

// ProgramMatch owns copied node bytes, independent of future windows/reset.
type ProgramMatch struct {
	Tag, Value    []byte
	Depth, Offset uint64
	Constructed   bool
}

func (q *ProgramExecution) Feed(kind int, tag, value []byte, depth, offset int, skipped bool) (*ProgramMatch, Code, ProgramDiagnostic) {
	var event C.tlv_tree_event_t
	var matched C.int
	var diagnostic C.tlv_query_diagnostic_t
	var omit C.int
	if skipped {
		omit = 1
	}
	code := Code(C.go_query_feed(q.ptr, C.int(kind), bytePointer(tag), C.size_t(len(tag)),
		bytePointer(value), C.size_t(len(value)), C.size_t(depth), C.size_t(offset), omit, &event, &matched, &diagnostic))
	runtime.KeepAlive(tag)
	runtime.KeepAlive(value)
	runtime.KeepAlive(q)
	if code != OK || matched == 0 {
		return nil, code, programDiagnostic(diagnostic, code)
	}
	item := match(&event)
	return &item, code, programDiagnostic(diagnostic, code)
}
func (q *ProgramExecution) Finish() (Code, ProgramDiagnostic) {
	var diagnostic C.tlv_query_diagnostic_t
	code := Code(C.go_query_finish(q.ptr, &diagnostic))
	runtime.KeepAlive(q)
	return code, programDiagnostic(diagnostic, code)
}
func (q *ProgramExecution) NextResult() (ProgramMatch, Code) {
	item, _, code := q.NextResultWithOrdinal()
	return item, code
}
func (q *ProgramExecution) NextResultWithOrdinal() (ProgramMatch, uint64, Code) {
	var event C.tlv_tree_event_t
	var ordinal C.size_t
	code := Code(C.tlv_query_result_next_ordinal(C.go_query_exec(q.ptr), &event, &ordinal))
	defer runtime.KeepAlive(q)
	if code != OK {
		return ProgramMatch{}, 0, code
	}
	return match(&event), uint64(ordinal), code
}
func (q *ProgramExecution) FeedEncoded(data []byte, depth, offset int) (*ProgramMatch, Code, ProgramDiagnostic) {
	var event C.tlv_tree_event_t
	var matched C.int
	var diagnostic C.tlv_query_diagnostic_t
	code := Code(C.go_query_feed_encoded(q.ptr, bytePointer(data), C.size_t(len(data)),
		C.size_t(depth), C.size_t(offset), &event, &matched, &diagnostic))
	runtime.KeepAlive(data)
	runtime.KeepAlive(q)
	if code != OK || matched == 0 {
		return nil, code, programDiagnostic(diagnostic, code)
	}
	item := match(&event)
	return &item, code, programDiagnostic(diagnostic, code)
}

func match(event *C.tlv_tree_event_t) ProgramMatch {
	return ProgramMatch{Tag: bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(event.element.tag.data)), int(event.element.tag.size))),
		Value: bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(event.element.value.data)), int(event.element.value.size))), Depth: uint64(event.depth), Offset: uint64(event.offset), Constructed: event.kind == C.TLV_TREE_BEGIN}
}

type queryVisitor struct {
	callback func(ProgramMatch) int
	panic    any
}

//export goQueryCallback
func goQueryCallback(handle C.uintptr_t, event *C.tlv_tree_event_t) (action C.int) {
	state := cgo.Handle(handle).Value().(*queryVisitor)
	defer func() {
		if value := recover(); value != nil {
			state.panic = value
			action = 2
		}
	}()
	return C.int(state.callback(match(event)))
}
func (q *ProgramExecution) Visit(callback func(ProgramMatch) int) (Code, ProgramDiagnostic) {
	state := &queryVisitor{callback: callback}
	handle := cgo.NewHandle(state)
	defer handle.Delete()
	var d C.tlv_query_diagnostic_t
	code := Code(C.query_visit_handle(q.ptr, C.uintptr_t(handle), &d))
	runtime.KeepAlive(q)
	if state.panic != nil {
		panic(state.panic)
	}
	return code, programDiagnostic(d, code)
}
func (q *ProgramExecution) Exists(early bool) (bool, Code, ProgramDiagnostic) {
	var found, flag C.int
	if early {
		flag = 1
	}
	var d C.tlv_query_diagnostic_t
	code := Code(C.go_query_exists(q.ptr, flag, &found, &d))
	runtime.KeepAlive(q)
	return found != 0, code, programDiagnostic(d, code)
}
func (q *ProgramExecution) Info() (map[string]uint64, Code) {
	defer runtime.KeepAlive(q)
	var i C.tlv_query_exec_info_t
	i.struct_size = C.size_t(C.sizeof_tlv_query_exec_info_t)
	code := Code(C.tlv_query_exec_info(C.go_query_exec(q.ptr), &i))
	return map[string]uint64{"elements": uint64(i.elements), "work": uint64(i.work), "skipped_subtrees": uint64(i.skipped_subtrees), "finished": uint64(i.finished), "full_validation": uint64(i.full_validation), "invalid": uint64(i.invalid)}, code
}
func (q *ProgramExecution) Control(ordinal int, pruning bool) Code {
	defer runtime.KeepAlive(q)
	if ordinal < 0 {
		flag := C.int(0)
		if pruning {
			flag = 1
		}
		return Code(C.tlv_query_exec_pruning(C.go_query_exec(q.ptr), flag))
	}
	return Code(C.tlv_query_exec_context(C.go_query_exec(q.ptr), C.size_t(ordinal)))
}
func (q *ProgramExecution) Result() (kind int, number int64, data []byte, code Code) {
	defer runtime.KeepAlive(q)
	var result C.tlv_query_result_t
	code = Code(C.tlv_query_exec_result(C.go_query_exec(q.ptr), &result))
	if code != OK {
		return
	}
	kind = int(result.kind)
	switch kind {
	case 1:
		number = int64(result.boolean)
	case 2:
		number = int64(result.integer)
	case 3, 4:
		if uint64(result.size) > uint64(^uint(0)>>1) {
			code = NativeSize
			return
		}
		data = bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(result.data)), int(result.size)))
	}
	return
}
func (q *ProgramExecution) Document(document *Document, context Node, capacity int) (Code, ProgramDiagnostic) {
	if capacity < 0 {
		return InvalidArg, ProgramDiagnostic{}
	}
	var d C.tlv_query_diagnostic_t
	code := Code(C.go_query_document(q.ptr, document.ptr, context.ptr, C.size_t(capacity), &d))
	runtime.KeepAlive(q)
	runtime.KeepAlive(document)
	return code, programDiagnostic(d, code)
}
func (q *ProgramExecution) NextDocument() (Node, Code, ProgramDiagnostic) {
	defer runtime.KeepAlive(q)
	var node unsafe.Pointer
	var d C.tlv_query_diagnostic_t
	code := Code(C.go_query_document_next(q.ptr, &node, &d))
	if code == End {
		return Node{}, code, ProgramDiagnostic{}
	}
	return Node{node}, code, programDiagnostic(d, code)
}
func (q *ProgramExecution) EditDocument(document *Document, kind int, tag, value []byte, capacity int) (int, Code, ProgramDiagnostic) {
	var applied C.size_t
	var d C.tlv_query_diagnostic_t
	code := Code(C.go_query_edit(q.ptr, document.ptr, C.int(kind), bytePointer(tag), C.size_t(len(tag)),
		bytePointer(value), C.size_t(len(value)), C.size_t(capacity), &applied, &d))
	runtime.KeepAlive(q)
	runtime.KeepAlive(document)
	runtime.KeepAlive(tag)
	runtime.KeepAlive(value)
	return int(applied), code, programDiagnostic(d, code)
}
func (n Node) Identity() uint64 { return uint64(C.go_query_node_identity(n.ptr)) }

func queryCodecDetail(d C.tlv_query_diagnostic_t) *CodecDetail {
	if d.cause != C.TLV_QUERY_CAUSE_CODEC {
		return nil
	}
	return codecDetail(d.diagnostic, C.query_codec(&d))
}
