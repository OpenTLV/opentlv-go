// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include "bridge.h"
*/
import "C"

func ReaderOperationName(value int) string {
	return C.GoString(C.tlv_reader_operation_string(C.tlv_reader_operation_t(value)))
}
func WriterOperationName(value int) string {
	return C.GoString(C.tlv_writer_operation_string(C.tlv_writer_operation_t(value)))
}
func QueryErrorKindName(value int) string {
	return C.GoString(C.tlv_query_error_kind_string(C.tlv_query_error_kind_t(value)))
}
func CodecReasonName(value int) string {
	return C.GoString(C.tlv_codec_reason_string(C.tlv_codec_reason_t(value)))
}
func SchemaCauseName(value int) string {
	return C.GoString(C.tlv_schema_cause_string(C.tlv_schema_cause_t(value)))
}
func QueryCauseName(value int) string {
	return C.GoString(C.tlv_query_cause_string(C.tlv_query_cause_t(value)))
}
func SchemaIssueName(value int) string {
	return C.GoString(C.tlv_schema_issue_kind_string(C.tlv_schema_issue_kind_t(value)))
}
func SchemaDefinitionKindName(value int) string {
	return C.GoString(C.tlv_schema_definition_kind_string(C.tlv_schema_definition_kind_t(value)))
}
func CodecOperationName(value int) string {
	return C.GoString(C.tlv_codec_operation_string(C.tlv_codec_operation_t(value)))
}
func CodecCauseName(value int) string {
	return C.GoString(C.tlv_codec_cause_string(C.tlv_codec_cause_t(value)))
}
func CodecViolationName(value int) string {
	return C.GoString(C.tlv_codec_violation_string(C.tlv_codec_violation_t(value)))
}
func SeverityName(value int) string {
	return C.GoString(C.tlv_diagnostic_severity_string(C.tlv_diagnostic_severity_t(value)))
}

func LocationDomainName(value int) string {
	return C.GoString(C.tlv_location_domain_string(C.tlv_location_domain_t(value)))
}
func LocationKindName(value int) string {
	return C.GoString(C.tlv_location_kind_string(C.tlv_location_kind_t(value)))
}
