// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
#include "fields.h"
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>

static void bind_field(go_field* f) {
    f->identifier_policy.forbidden_leading = f->forbidden;
    f->variable_identifier.policy = f->has_policy ? &f->identifier_policy : NULL;
    f->variable_length.policy = f->has_policy ? &f->length_policy : NULL;
}
tlv_result_t go_field_read_tag(go_field f, const uint8_t* data, size_t size,
                                tlv_tag_t* tag, size_t* consumed) {
    bind_field(&f);
    if (f.kind == GO_FIELD_FIXED_IDENTIFIER)
        return tlv_fixed_identifier_read(&f.fixed_identifier, data, size, tag, consumed);
    if (f.kind == GO_FIELD_VARIABLE_IDENTIFIER)
        return tlv_variable_identifier_read(&f.variable_identifier, data, size, tag, consumed);
    return TLV_ERR_INVALID_ARG;
}
tlv_result_t go_field_write_tag(go_field f, const uint8_t* bytes, size_t size,
                                 uint8_t* data, size_t capacity, size_t* written) {
    bind_field(&f);
    tlv_tag_t tag = tlv_tag(bytes, size);
    if (f.kind == GO_FIELD_FIXED_IDENTIFIER)
        return tlv_fixed_identifier_write(&f.fixed_identifier, &tag, data, capacity, written);
    if (f.kind == GO_FIELD_VARIABLE_IDENTIFIER)
        return tlv_variable_identifier_write(&f.variable_identifier, &tag, data, capacity, written);
    return TLV_ERR_INVALID_ARG;
}
tlv_result_t go_field_read_length(go_field f, const uint8_t* data, size_t size,
                                   tlv_size_t* length, size_t* consumed) {
    bind_field(&f);
    switch (f.kind) {
        case GO_FIELD_FIXED_LENGTH:
            return tlv_fixed_length_read(&f.fixed_length, data, size, length, consumed);
        case GO_FIELD_VARIABLE_LENGTH:
            return tlv_variable_length_read(&f.variable_length, data, size, length, consumed);
        case GO_FIELD_ESCAPED_LENGTH:
            return tlv_escaped_length_read(&f.escaped_length, data, size, length, consumed);
        default: return TLV_ERR_INVALID_ARG;
    }
}
tlv_result_t go_field_write_length(go_field f, tlv_size_t length, uint8_t* data,
                                    size_t capacity, size_t* written) {
    bind_field(&f);
    switch (f.kind) {
        case GO_FIELD_FIXED_LENGTH:
            return tlv_fixed_length_write(&f.fixed_length, length, data, capacity, written);
        case GO_FIELD_VARIABLE_LENGTH:
            return tlv_variable_length_write(&f.variable_length, length, data, capacity, written);
        case GO_FIELD_ESCAPED_LENGTH:
            return tlv_escaped_length_write(&f.escaped_length, length, data, capacity, written);
        default: return TLV_ERR_INVALID_ARG;
    }
}
tlv_result_t go_field_check(go_field f) {
    size_t consumed = 0;
    tlv_tag_t tag;
    tlv_size_t length;
    tlv_result_t rc = f.kind <= GO_FIELD_VARIABLE_IDENTIFIER
        ? go_field_read_tag(f, NULL, 0, &tag, &consumed)
        : go_field_read_length(f, NULL, 0, &length, &consumed);
    /* The Core primitives validate configuration before examining input. */
    return rc == TLV_ERR_TRUNCATED ? TLV_OK : rc;
}

struct go_owned_format {
    atomic_size_t references;
    int kind, has_constructed;
    go_field identifier, length;
    tlv_constructed_bit_t constructed;
    tlv_tagged_fields_composition_t tagged;
    tlv_escaped_format_t escaped;
    tlv_variable_format_t variable;
    tlv_packed_layout_t packed;
    tlv_tag_t* tags;
    size_t count;
    uint8_t* storage;
    tlv_format_t base, descriptor;
};
static tlv_result_t read_tag(const void* context, const uint8_t* data, size_t size,
                              tlv_tag_t* tag, size_t* consumed) {
    const go_owned_format* f = context;
    return go_field_read_tag(f->identifier, data, size, tag, consumed);
}
static tlv_result_t write_tag(const void* context, const tlv_tag_t* tag, uint8_t* data,
                               size_t size, size_t* written) {
    const go_owned_format* f = context;
    return go_field_write_tag(f->identifier, tag->data, tag->size, data, size, written);
}
static tlv_result_t read_length(const void* context, const uint8_t* data, size_t size,
                                 tlv_size_t* length, size_t* consumed) {
    const go_owned_format* f = context;
    return go_field_read_length(f->length, data, size, length, consumed);
}
static tlv_result_t write_length(const void* context, tlv_size_t length, uint8_t* data,
                                  size_t size, size_t* written) {
    const go_owned_format* f = context;
    return go_field_write_length(f->length, length, data, size, written);
}
/* Context adapters only; all wire operations and validation belong to Core. */
static tlv_result_t decode(const void* context, const uint8_t* data, size_t size,
                             tlv_decoded_t* result, tlv_format_error_t* error) {
    const go_owned_format* f = context;
    return f->base.decode(f->base.context, data, size, result, error);
}
static tlv_result_t measure(const void* context, const tlv_element_t* element,
                              tlv_encoding_t* result, tlv_format_error_t* error) {
    const go_owned_format* f = context;
    return f->base.measure(f->base.context, element, result, error);
}
static tlv_result_t encode(const void* context, const tlv_element_t* element,
                             uint8_t* data, size_t capacity, size_t* written,
                             tlv_format_error_t* error) {
    const go_owned_format* f = context;
    return f->base.encode(f->base.context, element, data, capacity, written, error);
}
static int constructed(const void* context, const tlv_tag_t* tag) {
    const go_owned_format* f = context;
    return tlv_constructed_bit_predicate(&f->constructed, tag);
}
go_owned_format* go_owned_format_new(go_field identifier, go_field length, int kind,
    int order, int scope, tlv_constructed_bit_t predicate, int has_predicate,
    tlv_packed_layout_t packed, size_t count) {
    if (count > SIZE_MAX / sizeof(tlv_tag_t)) return NULL;
    go_owned_format* f = calloc(1, sizeof *f);
    if (!f) return NULL;
    atomic_init(&f->references, 1);
    f->kind = kind;
    f->identifier = identifier;
    f->length = length;
    bind_field(&f->identifier);
    bind_field(&f->length);
    f->constructed = predicate;
    f->has_constructed = has_predicate;
    f->packed = packed;
    f->tagged.fields = (tlv_field_composition_t){f, read_tag, read_length, NULL,
        write_tag, write_length, (tlv_element_order_t)order, (tlv_length_scope_t)scope};
    if (count) {
        f->tags = calloc(count, sizeof *f->tags);
        if (!f->tags) { free(f); return NULL; }
    }
    f->count = count;
    return f;
}
tlv_result_t go_owned_format_tag(go_owned_format* f, size_t index, const uint8_t* data, size_t size) {
    if (index >= f->count || f->tags[index].data) return TLV_ERR_INVALID_ARG;
    uint8_t* copy = malloc(size ? size : 1);
    if (!copy) return TLV_ERR_OUT_OF_MEMORY;
    if (size) memcpy(copy, data, size);
    f->tags[index] = tlv_tag(copy, size);
    return TLV_OK;
}
tlv_result_t go_owned_format_storage(go_owned_format* f, const uint8_t* data, size_t size) {
    if (f->storage) return TLV_ERR_INVALID_ARG;
    f->storage = malloc(size ? size : 1);
    if (!f->storage) return TLV_ERR_OUT_OF_MEMORY;
    if (size) memcpy(f->storage, data, size);
    f->packed.tag_storage = f->storage;
    f->packed.tag_storage_size = size;
    return TLV_OK;
}
tlv_result_t go_owned_format_init(go_owned_format* f) {
    tlv_result_t rc;
    if (f->kind == GO_PACKED) {
        rc = tlv_packed_layout_validate(&f->packed);
        if (rc == TLV_OK)
            rc = tlv_format_init(&f->base, &f->packed, tlv_packed_decode,
                                  tlv_packed_measure, tlv_packed_encode);
    } else if (f->kind == GO_VARIABLE) {
        f->variable = (tlv_variable_format_t){f->identifier.variable_identifier,
            f->length.variable_length, f->tagged.fields.element_order,
            f->tagged.fields.length_scope, f->has_constructed ? &f->constructed : NULL};
        rc = tlv_variable_format_init(&f->base, &f->variable);
    } else if (f->kind == GO_ESCAPED) {
        f->escaped = (tlv_escaped_format_t){f->identifier.fixed_identifier,
            f->length.escaped_length, f->tagged.fields.element_order,
            f->tagged.fields.length_scope, f->tags, f->count};
        rc = tlv_escaped_format_init(&f->base, &f->escaped);
    } else {
        rc = go_field_check(f->identifier);
        if (rc == TLV_OK) rc = go_field_check(f->length);
        if (rc == TLV_OK) {
            f->tagged.tag_only = f->tags;
            f->tagged.count = f->count;
            rc = f->count ? tlv_tagged_fields_format_init(&f->base, &f->tagged)
                          : tlv_fields_format_init(&f->base, &f->tagged.fields);
        }
    }
    if (rc != TLV_OK) return rc;
    /* Use Core's predicate validation even for mixed and packed compositions. */
    if (f->has_constructed) {
        tlv_variable_format_t check = {0};
        tlv_field_composition_t fields;
        check.identifier = (tlv_variable_identifier_t){0x1f, 0x1f, 0x80, 0x7f, 8, NULL};
        check.length = (tlv_variable_length_t){0x80, 0x7f, TLV_BYTE_ORDER_BIG_ENDIAN, NULL};
        check.constructed = &f->constructed;
        rc = tlv_variable_fields_init(&fields, &check);
        if (rc != TLV_OK) return rc;
    }
    rc = tlv_format_init(&f->descriptor, f, decode, measure, encode);
    if (rc == TLV_OK && f->has_constructed) f->descriptor.is_constructed = constructed;
    return rc;
}
void go_owned_format_retain(go_owned_format* f) {
    if (f) atomic_fetch_add_explicit(&f->references, 1, memory_order_relaxed);
}
void go_owned_format_free(go_owned_format* f) {
    if (!f || atomic_fetch_sub_explicit(&f->references, 1, memory_order_acq_rel) != 1) return;
    for (size_t i = 0; i < f->count; ++i) free((void*)f->tags[i].data);
    free(f->tags);
    free(f->storage);
    free(f);
}
tlv_format_t go_owned_format_descriptor(go_owned_format* f) { return f->descriptor; }
