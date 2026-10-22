// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

/*
#include "bridge.h"
static tlv_reader_detail_t codec_reader(tlv_codec_detail_t d) { return d.detail.reader; }
static tlv_schema_detail_t codec_schema(tlv_codec_detail_t d) { return d.detail.schema; }
*/
import "C"
import "bytes"

// CodecDetail owns conversion evidence before borrowed C buffers expire.
type CodecDetail struct {
	Diagnostic                                    Diagnostic
	Operation, Reported, Violation, Reason, Cause int
	Representation                                string
	Reader                                        *Diagnostic
	Schema                                        *CodecSchemaDetail
}

// CodecSchemaDetail retains delegated Schema rule evidence.
type CodecSchemaDetail struct {
	Kind, DefinitionKind                                             int
	DefinitionIndex                                                  uint64
	Tag                                                              []byte
	Field                                                            string
	IsGroup, HasOccurs, HasLength, HasForm                           bool
	MinOccurs, MaxOccurs, Occurs, MinLength, MaxLength, ActualLength uint64
	ExpectedForm                                                     int
	ActualConstructed                                                bool
	LengthMultiple                                                   uint64
	LengthFlags                                                      uint32
}

func codecDetail(common C.tlv_diagnostic_t, d C.tlv_codec_detail_t) *CodecDetail {
	result := &CodecDetail{Diagnostic: diagnostic(common, 0), Operation: int(d.operation), Cause: int(d.cause), Reported: int(d.reported), Violation: int(d.violation), Reason: int(d.reason), Representation: C.GoString(d.representation)}
	if d.cause == C.TLV_CODEC_CAUSE_READER {
		reader := readerDiagnosticParts(common, C.codec_reader(d), Code(common.code))
		result.Reader = &reader
	}
	if d.cause == C.TLV_CODEC_CAUSE_SCHEMA {
		v := C.codec_schema(d)
		result.Schema = &CodecSchemaDetail{Kind: int(v.kind), DefinitionKind: int(v.definition.kind), DefinitionIndex: uint64(v.definition.index), Tag: bytes.Clone(nativeBytes(v.tag.data, v.tag.size)), Field: C.GoString(v.field), IsGroup: v.is_group != 0, HasOccurs: v.has_occurs != 0, HasLength: v.has_length != 0, HasForm: v.has_form != 0, MinOccurs: uint64(v.min_occurs), MaxOccurs: uint64(v.max_occurs), Occurs: uint64(v.occurs), MinLength: uint64(v.min_length), MaxLength: uint64(v.max_length), ActualLength: uint64(v.actual_length), ExpectedForm: int(v.expected_form), ActualConstructed: v.actual_constructed != 0, LengthMultiple: uint64(v.length_multiple), LengthFlags: uint32(v.length_flags)}
	}
	return result
}
