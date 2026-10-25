// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
#ifndef OPENTLV_GO_FIELDS_H
#define OPENTLV_GO_FIELDS_H
#include <tlv/formats/variable.h>
#include <tlv/formats/escaped.h>
#include <tlv/formats/packed.h>

/* Pointer-free snapshots cross cgo. Pointers are bound only inside C storage. */
typedef struct {
    int kind, has_policy;
    tlv_fixed_identifier_t fixed_identifier;
    tlv_fixed_length_t fixed_length;
    tlv_variable_identifier_t variable_identifier;
    tlv_variable_length_t variable_length;
    tlv_escaped_length_t escaped_length;
    tlv_identifier_policy_t identifier_policy;
    tlv_length_policy_t length_policy;
    uint8_t forbidden[256];
} go_field;
enum { GO_FIELD_FIXED_IDENTIFIER = 1, GO_FIELD_VARIABLE_IDENTIFIER,
       GO_FIELD_FIXED_LENGTH, GO_FIELD_VARIABLE_LENGTH, GO_FIELD_ESCAPED_LENGTH };
tlv_result_t go_field_check(go_field);
tlv_result_t go_field_read_tag(go_field, const uint8_t*, size_t, tlv_tag_t*, size_t*);
tlv_result_t go_field_write_tag(go_field, const uint8_t*, size_t, uint8_t*, size_t, size_t*);
tlv_result_t go_field_read_length(go_field, const uint8_t*, size_t, tlv_size_t*, size_t*);
tlv_result_t go_field_write_length(go_field, tlv_size_t, uint8_t*, size_t, size_t*);

typedef struct go_owned_format go_owned_format;
enum { GO_COMPOSED = 1, GO_ESCAPED, GO_VARIABLE, GO_PACKED };
go_owned_format* go_owned_format_new(go_field, go_field, int, int, int,
    tlv_constructed_bit_t, int, tlv_packed_layout_t, size_t);
tlv_result_t go_owned_format_tag(go_owned_format*, size_t, const uint8_t*, size_t);
tlv_result_t go_owned_format_storage(go_owned_format*, const uint8_t*, size_t);
tlv_result_t go_owned_format_init(go_owned_format*);
void go_owned_format_retain(go_owned_format*);
void go_owned_format_free(go_owned_format*);
tlv_format_t go_owned_format_descriptor(go_owned_format*);
#endif
