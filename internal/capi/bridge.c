// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
#include "bridge.h"
#include <stdlib.h>
#include <string.h>
#if OPENTLV_FORMAT_BER
#include <tlv/builtins/asn1/query.h>
#endif
#if OPENTLV_DOCUMENT
#include <tlv/document/document.h>
#endif
#if OPENTLV_FORMAT_BER
#include <tlv/builtins/asn1/ber.h>
#endif
#if OPENTLV_FORMAT_DER
#include <tlv/builtins/asn1/der.h>
#endif
#if OPENTLV_FORMAT_CER
#include <tlv/builtins/asn1/cer.h>
#endif
#if OPENTLV_EMV
#include <tlv/builtins/emv/format.h>
#endif
#if OPENTLV_LLDP
#include <tlv/builtins/lldp/lldp.h>
#endif
#if OPENTLV_BLUETOOTH
#include <tlv/builtins/bluetooth/bluetooth_ltv.h>
#endif
#if OPENTLV_DHCP
#include <tlv/builtins/dhcp/dhcpv4.h>
#endif
#if OPENTLV_NFC
#include <tlv/builtins/nfc/type2.h>
#endif

/* Fixed contexts are call-local; owned compositions retain C-only storage. */
static tlv_result_t resolve(go_format config, tlv_format_t* format, tlv_fixed_format_t* fixed) {
    switch (config.kind) {
        case GO_FORMAT_OWNED:
            if (!config.owned) return TLV_ERR_INVALID_ARG;
            *format = go_owned_format_descriptor(config.owned);
            return TLV_OK;
        case GO_FORMAT_FIXED:
            fixed->identifier.size = config.tag_size;
            fixed->length.size = config.length_size;
            fixed->length.byte_order = (tlv_byte_order_t)config.byte_order;
            fixed->element_order = (tlv_element_order_t)config.element_order;
            fixed->length_scope = (tlv_length_scope_t)config.length_scope;
            return tlv_fixed_format_init(format, fixed);
#if OPENTLV_FORMAT_BER
        case GO_FORMAT_BER: *format = tlv_format_ber; return TLV_OK;
        case GO_FORMAT_BER_INDEFINITE: *format = tlv_format_ber_indefinite; return TLV_OK;
#endif
#if OPENTLV_FORMAT_DER
        case GO_FORMAT_DER: *format = tlv_format_der; return TLV_OK;
#endif
#if OPENTLV_FORMAT_CER
        case GO_FORMAT_CER: *format = tlv_format_cer; return TLV_OK;
#endif
#if OPENTLV_EMV
        case GO_FORMAT_EMV: *format = tlv_format_emv; return TLV_OK;
#endif
#if OPENTLV_LLDP
        case GO_FORMAT_LLDP: *format = tlv_format_lldp; return TLV_OK;
#endif
#if OPENTLV_BLUETOOTH
        case GO_FORMAT_BLUETOOTH_LTV: *format = tlv_format_bluetooth_ltv; return TLV_OK;
#endif
#if OPENTLV_DHCP
        case GO_FORMAT_DHCPV4: *format = tlv_format_dhcpv4; return TLV_OK;
#endif
#if OPENTLV_NFC
        case GO_FORMAT_NFC_TYPE2: *format = tlv_format_nfc_type2; return TLV_OK;
#endif
        default: return TLV_ERR_UNSUPPORTED;
    }
}

/* Persistent descriptors and their context are entirely C-owned. */
struct go_document {
    go_owned_format* owned;
    tlv_format_t       format;
    tlv_fixed_format_t fixed;
#if OPENTLV_DOCUMENT
    tlv_document_t* document;
#endif
};

go_document* go_document_parse(go_format config, const uint8_t* data, size_t size, size_t depth,
                               size_t elements, int defaults, int retain_source_locations,
                               int* code, tlv_reader_diagnostic_t* diagnostic) {
    tlv_reader_diagnostic_init(diagnostic);
#if OPENTLV_DOCUMENT
    go_document*           d = calloc(1, sizeof(*d));
    tlv_document_options_t options;
    if (!d) {
        *code = TLV_ERR_OUT_OF_MEMORY;
        return NULL;
    }
    d->owned = config.owned;
    go_owned_format_retain(d->owned);
    *code = resolve(config, &d->format, &d->fixed);
    if (*code == TLV_OK) *code = tlv_document_options_init(&options, &d->format);
    if (*code == TLV_OK) {
        options.retain_source_locations = retain_source_locations;
        if (!defaults) {
            options.max_depth = depth;
            options.max_elements = elements;
        }
        tlv_tree_reader_t       reader;
        tlv_document_builder_t* builder = NULL;
        size_t                  capacity = options.max_depth < size ? options.max_depth : size;
        tlv_tree_frame_t*       frames = NULL;
        if (capacity > SIZE_MAX / sizeof(*frames)) {
            *code = TLV_ERR_OVERFLOW;
        } else {
            if (capacity) frames = calloc(capacity, sizeof(*frames));
            *code = capacity && !frames
                        ? TLV_ERR_OUT_OF_MEMORY
                        : tlv_tree_reader_init(&reader, data, size, &d->format, frames, capacity,
                                               options.max_depth, options.max_elements);
            if (*code == TLV_OK)
                *code = tlv_document_builder_create(&options, &reader, NULL, &builder);
            if (*code == TLV_OK)
                *code = tlv_document_builder_consume(builder, &d->document, diagnostic);
            tlv_document_builder_free(builder);
            free(frames);
        }
    }
    if (*code != TLV_OK) {
        go_owned_format_free(d->owned);
        free(d);
        return NULL;
    }
    return d;
#else
    (void)config;
    (void)data;
    (void)size;
    (void)depth;
    (void)elements;
    (void)defaults;
    (void)retain_source_locations;
    *code = TLV_ERR_UNSUPPORTED;
    return NULL;
#endif
}

void go_document_free(go_document* d) {
#if OPENTLV_DOCUMENT
    if (d) tlv_document_free(d->document);
#endif
    if (d) go_owned_format_free(d->owned);
    free(d);
}

size_t go_document_count(go_document* d) {
#if OPENTLV_DOCUMENT
    return tlv_document_count(d->document);
#else
    (void)d;
    return 0;
#endif
}

#if OPENTLV_DOCUMENT
typedef struct {
    void** nodes;
    size_t count;
} go_query_results;
static tlv_visit_result_t collect_query(tlv_node_t* node, void* context) {
    go_query_results* results = context;
    results->nodes[results->count++] = node;
    return TLV_VISIT_CONTINUE;
}
#endif

int go_document_query(go_document* d, const char* text, void*** nodes, size_t* count,
                      tlv_diagnostic_t* diagnostic) {
#if OPENTLV_DOCUMENT
    tlv_query_t      query;
    tlv_result_t     code = tlv_query_parse(text, &query, diagnostic);
    go_query_results results = {NULL, 0};
    size_t           capacity;
    if (code != TLV_OK) return code;
    capacity = tlv_document_count(d->document);
    if (capacity > SIZE_MAX / sizeof(void*)) return TLV_ERR_OVERFLOW;
    if (capacity) {
        results.nodes = malloc(capacity * sizeof(void*));
        if (!results.nodes) return TLV_ERR_OUT_OF_MEMORY;
    }
    tlv_query_diagnostic_t detail;
    code = tlv_document_query_visit(d->document, &query, collect_query, &results, &detail);
    if (code != TLV_OK) {
        free(results.nodes);
        *diagnostic = detail.diagnostic;
        return code;
    }
    *nodes = results.nodes;
    *count = results.count;
    return TLV_OK;
#else
    (void)d;
    (void)text;
    (void)nodes;
    (void)count;
    (void)diagnostic;
    return TLV_ERR_UNSUPPORTED;
#endif
}
void go_query_free(void** nodes) {
    free(nodes);
}

void* go_document_node(go_document* d, void* n, int operation) {
#if OPENTLV_DOCUMENT
    switch (operation) {
        case 0: return tlv_document_first(d->document);
        case 1: return tlv_node_first_child(n);
        case 2: return tlv_node_next(n);
        case 3: return tlv_node_parent(n);
        case 4: return tlv_node_next_same_tag(n);
    }
#else
    (void)d;
    (void)n;
    (void)operation;
#endif
    return NULL;
}

go_read_result go_document_read(void* n) {
    go_read_result r = {0};
#if OPENTLV_DOCUMENT
    r.element.tag = tlv_node_tag(n);
    r.element.value.data = tlv_node_value_data(n);
    r.element.value.size = tlv_node_value_size(n);
    r.consumed = tlv_node_is_constructed(n);
#else
    (void)n;
#endif
    return r;
}

int go_document_edit(go_document* d, void* n, void* before, const uint8_t* tag, size_t tag_size,
                     const uint8_t* value, size_t value_size, int operation, void** result) {
#if OPENTLV_DOCUMENT
    tlv_node_t*  inserted = NULL;
    tlv_result_t code;
    if (operation == 0) return tlv_node_set_value(n, value, value_size);
    if (operation == 1) {
        tlv_node_erase(n);
        return TLV_OK;
    }
    code = tlv_document_insert(d->document, n, before, tlv_tag(tag, tag_size), value, value_size,
                               &inserted);
    *result = inserted;
    return code;
#else
    (void)d;
    (void)n;
    (void)before;
    (void)tag;
    (void)tag_size;
    (void)value;
    (void)value_size;
    (void)operation;
    (void)result;
    return TLV_ERR_UNSUPPORTED;
#endif
}

go_write_result go_document_encode(go_document* d, go_format config, uint8_t* data, size_t capacity,
                                   int measure) {
    go_write_result r = {0};
#if OPENTLV_DOCUMENT
    tlv_format_t       format;
    tlv_fixed_format_t fixed;
    r.code = resolve(config, &format, &fixed);
    if (r.code == TLV_OK)
        r.code = measure ? tlv_document_encoded_size_as(d->document, &format, &r.size)
                         : tlv_document_encode_as(d->document, &format, data, capacity, &r.size);
#else
    (void)d;
    (void)config;
    (void)data;
    (void)capacity;
    (void)measure;
    r.code = TLV_ERR_UNSUPPORTED;
#endif
    return r;
}

tlv_result_t go_format_check(go_format config) {
    tlv_format_t       format;
    tlv_fixed_format_t fixed;
    return resolve(config, &format, &fixed);
}

void* go_document_find(go_document* d, void* parent, const uint8_t* tag, size_t size) {
#if OPENTLV_DOCUMENT
    return tlv_document_find(d->document, parent, tlv_tag(tag, size));
#else
    (void)d; (void)parent; (void)tag; (void)size;
    return NULL;
#endif
}

void go_node_location(void* node, size_t* offset, size_t* header, int* has_offset, int* has_header) {
#if OPENTLV_DOCUMENT
    tlv_document_source_location_t source = tlv_node_source_location(node);
    *offset = source.offset; *header = source.header_size;
    *has_offset = source.has_offset; *has_header = source.has_header_size;
#else
    (void)node; *offset = *header = 0; *has_offset = *has_header = 0;
#endif
}

go_write_result go_node_encode(void* node, go_format config, uint8_t* data, size_t capacity, int measure) {
    go_write_result r = {0};
#if OPENTLV_DOCUMENT
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    r.code = resolve(config, &format, &fixed);
    if (r.code == TLV_OK)
        r.code = measure ? tlv_node_encoded_size_as(node, &format, &r.size)
                         : tlv_node_encode_as(node, &format, data, capacity, &r.size);
#else
    (void)node; (void)config; (void)data; (void)capacity; (void)measure;
    r.code = TLV_ERR_UNSUPPORTED;
#endif
    return r;
}

/* Go callbacks run outside C and locks. These adapters carry only staged wire
 * bytes; Core owns complete structure validation and diagnostic coordinates. */
static tlv_result_t structure_decode_bytes(const void* context, const tlv_format_t* format,
    const uint8_t* data, size_t size, void* value, size_t capacity,
    tlv_codec_diagnostic_t* diagnostic) {
    (void)context; (void)format; (void)data; (void)size;
    (void)value; (void)capacity; (void)diagnostic;
    return TLV_OK;
}
static tlv_result_t structure_encode_bytes(const void* context, const tlv_format_t* format,
    const void* value, size_t size, uint8_t* data, size_t capacity, size_t* written,
    tlv_codec_diagnostic_t* diagnostic) {
    (void)context; (void)format; (void)diagnostic;
    if (data && capacity < size) return TLV_ERR_BUFFER_TOO_SHORT;
    if (data && size) memcpy(data, value, size);
    *written = size;
    return TLV_OK;
}
tlv_result_t go_structure_convert(go_format config, const tlv_structure_schema_t* schema,
    size_t depth, size_t elements, const uint8_t* data, size_t size, uint8_t* output,
    int encode, tlv_codec_diagnostic_t* diagnostic) {
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    uint8_t dummy = 0;
    size_t written = 0;
    tlv_codec_diagnostic_init(diagnostic, encode ? TLV_CODEC_OP_ENCODE : TLV_CODEC_OP_DECODE);
    tlv_result_t code = resolve(config, &format, &fixed);
    if (code != TLV_OK) return code;
    tlv_structure_codec_t codec = {NULL, &format, schema, depth, elements,
                                  structure_decode_bytes, structure_encode_bytes};
    if (encode)
        return tlv_structure_encode(&codec, data ? data : &dummy, size,
                                     output ? output : &dummy, size, &written, diagnostic);
    return tlv_structure_decode(&codec, data, size, &dummy, sizeof(dummy), diagnostic);
}

tlv_result_t go_generate(go_format config, const tlv_generator_options_t* options,
    uint8_t* data, size_t capacity, uint8_t* workspace, size_t workspace_size, size_t* written) {
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    tlv_result_t code = resolve(config, &format, &fixed);
    if (code != TLV_OK) return code;
    return tlv_generate(&format, options, data, capacity, workspace, workspace_size, written);
}

tlv_result_t go_structure_validate(const tlv_schema_checked_t* checked, go_format config,
                                   const uint8_t* data, size_t size, size_t depth, size_t elements,
                                   int unknown, tlv_schema_diagnostic_report_t* report,
                                   tlv_schema_diagnostic_t* diagnostic) {
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    tlv_schema_diagnostic_init(diagnostic);
    tlv_result_t code = resolve(config, &format, &fixed);
    if (code != TLV_OK) return code;
    if (report)
        return tlv_schema_validate_all_checked(checked, data, size, &format, depth, elements,
                                               (tlv_schema_unknown_policy_t)unknown, report, diagnostic);
    return tlv_schema_validate_checked(checked, data, size, &format, depth, elements, diagnostic);
}

int go_format_inspect(go_format config, int operation, const uint8_t* tag, size_t size) {
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    if (resolve(config, &format, &fixed) != TLV_OK) return 0;
    if (operation == 0) return tlv_format_can_read(&format);
    if (operation == 1) return tlv_format_can_write(&format);
    tlv_tag_t identifier = tlv_tag(tag, size);
    return format.is_constructed ? format.is_constructed(format.context, &identifier) : 0;
}

tlv_result_t go_format_preserve(go_format config, const uint8_t* source, size_t source_size,
    const uint8_t* tag, size_t tag_size, const uint8_t* value, size_t value_size,
    uint8_t* output, size_t capacity) {
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    tlv_decoded_t decoded;
    tlv_result_t rc = resolve(config, &format, &fixed);
    if (rc != TLV_OK) return rc;
    rc = tlv_format_decode(&format, source, source_size, &decoded, NULL);
    if (rc != TLV_OK) return rc;
    if (decoded.source.size != source_size) return TLV_ERR_INVALID_ARG;
    tlv_element_t element = {tlv_tag(tag, tag_size), {value, value_size}};
    size_t written = 0;
    return tlv_source_preserve(&decoded.source, &element, output, capacity, &written);
}

/* Reconstruct a single staged parent for this call; all pointers are call-local. */
go_write_result go_tree(go_format config, uint8_t* data, size_t capacity, const uint8_t* tag,
                        size_t tag_size, const uint8_t* value, size_t value_size, uint8_t* scratch,
                        int close) {
    go_write_result         result = {0};
    tlv_format_t            format;
    tlv_fixed_format_t      fixed;
    tlv_tree_writer_t       writer;
    tlv_tree_writer_frame_t frame;
    tlv_writer_diagnostic_init(&result.diagnostic);
    result.code = resolve(config, &format, &fixed);
    if (result.code != TLV_OK) return result;
    result.code = tlv_tree_writer_init(&writer, data, capacity, &format, &frame, 1, scratch,
                                       value_size, SIZE_MAX, SIZE_MAX);
    if (result.code != TLV_OK) return result;
    result.code = tlv_tree_writer_begin_diag(&writer, tlv_tag(tag, tag_size), &result.diagnostic);
    if (result.code != TLV_OK || !close) return result;
    result.code = tlv_writer_copy_encoded(&writer.output, value, value_size);
    if (result.code != TLV_OK) return result;
    result.code = tlv_tree_writer_end_diag(&writer, &result.diagnostic);
    if (result.code == TLV_OK) result.size = tlv_tree_writer_size(&writer);
    return result;
}

go_read_result go_read(go_format config, const uint8_t* data, size_t size, int final_input) {
    go_read_result     result = {0};
    tlv_format_t       format;
    tlv_fixed_format_t fixed;
    tlv_reader_t       reader;
    tlv_reader_diagnostic_init(&result.diagnostic);
    result.code = resolve(config, &format, &fixed);
    if (result.code != TLV_OK) return result;
    result.code = final_input ? tlv_reader_init(&reader, data, size, &format)
                              : tlv_reader_init_incremental(&reader, data, size, &format);
    if (result.code != TLV_OK) return result;
    result.code =
        tlv_reader_next_source_diag(&reader, &result.element, &result.source, &result.diagnostic);
    if (result.code == TLV_OK) result.consumed = tlv_reader_consumed(&reader);
    /* The Go projection keeps ranges, never the temporary descriptor address. */
    result.source.format = NULL;
    return result;
}

go_write_result go_write(go_format config, uint8_t* data, size_t capacity, const uint8_t* tag,
                         size_t tag_size, const uint8_t* value, size_t value_size, int measure) {
    go_write_result    result = {0};
    tlv_format_t       format;
    tlv_fixed_format_t fixed;
    tlv_element_t      element;
    tlv_writer_diagnostic_init(&result.diagnostic);
    result.code = resolve(config, &format, &fixed);
    if (result.code != TLV_OK) return result;
    element.tag = tlv_tag(tag, tag_size);
    element.value.data = value;
    element.value.size = value_size;
    result.code =
        measure ? tlv_element_encoded_size_diag(&element, &format, &result.size, &result.diagnostic)
                : tlv_write_element_diag(data, capacity, &format, &element, &result.size,
                                         &result.diagnostic);
    return result;
}

/* The only persistent Reader data are scalar continuation fields and offsets.
 * Reattach borrowed pointers to a call-local cursor, never to Go-owned state. */
go_tree_read_result go_tree_read(go_format config, tlv_tree_reader_t* state,
    tlv_tree_frame_t* frames, size_t capacity, const uint8_t* old_data,
    const uint8_t* data, size_t size, size_t discard, int final_input,
    size_t max_depth, size_t max_elements, int operation) {
    go_tree_read_result result = {0};
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    tlv_tree_reader_t reader = *state;
    tlv_reader_diagnostic_init(&result.diagnostic);
    result.code = resolve(config, &format, &fixed);
    if (result.code != TLV_OK) return result;
    reader.input.format = &format;
    reader.input.data = old_data;
    reader.frames = frames;
    reader.capacity = capacity;
    if (operation == 0) {
        result.code = final_input
            ? tlv_tree_reader_init(&reader, data, size, &format, frames, capacity,
                                   max_depth, max_elements)
            : tlv_tree_reader_init_incremental(&reader, data, size, &format, frames, capacity,
                                               max_depth, max_elements);
    } else if (operation == 1) {
        result.code = tlv_tree_reader_next_event_diag(&reader, &result.event, &result.diagnostic);
    } else if (operation == 2) {
        tlv_tree_item_t item = {0};
        result.code = tlv_tree_reader_next_diag(&reader, &item, &result.diagnostic);
        if (result.code == TLV_OK) {
            result.event = (tlv_tree_event_t){item.constructed ? TLV_TREE_BEGIN : TLV_TREE_ELEMENT,
                                            item.element, item.source, item.depth, item.offset, 0};
        }
    } else if (operation == 3) {
        result.code = tlv_tree_reader_set_input(&reader, data, size, discard, final_input);
    } else if (operation == 4) {
        result.code = tlv_tree_reader_skip_subtree(&reader);
    } else {
        result.code = TLV_ERR_INVALID_ARG;
    }
    result.consumed = tlv_tree_reader_consumed(&reader);
    result.offset = tlv_tree_reader_offset(&reader);
    result.at_end = tlv_tree_reader_at_end(&reader);
    result.event.source.format = NULL;
    reader.input.format = NULL;
    reader.input.data = NULL;
    reader.frames = NULL;
    *state = reader;
    return result;
}

go_write_result go_writer_append(go_format config, uint8_t* output, size_t capacity,
    const uint8_t* source, size_t source_size, const uint8_t* tag, size_t tag_size,
    const uint8_t* value, size_t value_size, int preserve) {
    go_write_result result = {0};
    tlv_format_t format;
    tlv_fixed_format_t fixed;
    tlv_writer_t writer;
    tlv_decoded_t decoded;
    tlv_writer_diagnostic_init(&result.diagnostic);
    result.code = resolve(config, &format, &fixed);
    if (result.code != TLV_OK) return result;
    result.code = tlv_writer_init(&writer, output, capacity, &format);
    if (result.code != TLV_OK) return result;
    if (preserve) {
        result.code = tlv_format_decode(&format, source, source_size, &decoded, NULL);
        if (result.code != TLV_OK) return result;
        if (decoded.source.size != source_size) {
            result.code = TLV_ERR_INVALID_ARG;
            return result;
        }
        tlv_element_t element = {tlv_tag(tag, tag_size), {value, value_size}};
        result.code = tlv_writer_preserve_diag(&writer, &decoded.source, &element,
                                              &result.diagnostic);
    } else {
        result.code = tlv_writer_copy_encoded_diag(&writer, source, source_size, &result.diagnostic);
    }
    if (result.code == TLV_OK) result.size = tlv_writer_size(&writer);
    return result;
}

/* Owning language adapter. Core Query calls still receive explicit storage. */
struct go_query_program {
    go_owned_format* owned;
    size_t                     references;
    tlv_format_t               format;
    tlv_fixed_format_t         fixed;
    tlv_query_environment_t    environment;
    tlv_query_hook_t           hooks[4];
    uintptr_t                  provider_handles[4];
    size_t                     provider_count;
    uintptr_t                  tag_handle;
    tlv_query_tag_adapter_t    tags;
    void*                      image;
    const tlv_query_program_t* program;
    tlv_query_program_info_t   info;
};
extern int  goQueryProviderDecode(uintptr_t, tlv_tree_event_t*, uint8_t*, size_t, void*, size_t,
                                  tlv_query_result_t*);
extern void goQueryProviderRelease(uintptr_t);
extern int  goQueryTagLookup(uintptr_t, int, uint8_t*, size_t, int64_t*);
extern int  goQueryResolve(uintptr_t, char*, size_t, char*, size_t, tlv_tag_t*);
static tlv_result_t query_tag_class(const void* context, const tlv_tag_t* tag, int64_t* value) {
    return (tlv_result_t)goQueryTagLookup((uintptr_t)context, 0, (uint8_t*)tag->data, tag->size,
                                          value);
}
static tlv_result_t query_tag_number(const void* context, const tlv_tag_t* tag, int64_t* value) {
    return (tlv_result_t)goQueryTagLookup((uintptr_t)context, 1, (uint8_t*)tag->data, tag->size,
                                          value);
}
static tlv_result_t query_dynamic_resolve(const void* context, const char* space, size_t space_size,
                                          const char* name, size_t name_size, tlv_tag_t* tag) {
    return (tlv_result_t)goQueryResolve((uintptr_t)context, (char*)space, space_size, (char*)name,
                                        name_size, tag);
}
static tlv_result_t query_provider_decode(const void* context, const tlv_tree_event_t* event,
                                          const uint8_t* data, size_t size, void* scratch,
                                          size_t capacity, tlv_query_result_t* result,
                                          tlv_codec_diagnostic_t* diagnostic) {
    tlv_codec_diagnostic_init(diagnostic, TLV_CODEC_OP_DECODE);
    return tlv_codec_diagnostic_result(
        diagnostic,
        (tlv_result_t)goQueryProviderDecode((uintptr_t)context, (tlv_tree_event_t*)event,
                                            (uint8_t*)data, size, scratch, capacity, result));
}
typedef struct query_buffer {
    struct query_buffer* next;
    uint8_t              data[1];
} query_buffer;
struct go_query_execution {
    go_query_program*       program;
    tlv_query_environment_t environment;
    void*                   storage;
    void*                   storage_allocation;
    size_t                  capacity, depth, nodes, work;
    int                     retained, has_reader, has_document, fed;
    tlv_query_exec_t*       exec;
    tlv_tree_reader_t       reader;
    tlv_tree_frame_t*       frames;
    query_buffer*           buffers;
    void*                   values;
};
typedef struct query_names {
    const go_query_name* names;
    size_t               count;
} query_names;
static tlv_result_t query_resolve(const void* context, const char* space, size_t space_size,
                                  const char* name, size_t name_size, tlv_tag_t* tag) {
    const query_names* names = context;
    for (size_t i = 0; i < names->count; ++i) {
        const go_query_name* entry = &names->names[i];
        size_t               size = strlen(entry->name);
        if (!space_size && size == name_size && !memcmp(entry->name, name, size)) {
            *tag = tlv_tag(entry->tag, entry->size);
            return TLV_OK;
        }
        if (space_size && size > space_size && size - space_size - 1 == name_size &&
            entry->name[space_size] == ':' && !memcmp(entry->name, space, space_size) &&
            !memcmp(entry->name + space_size + 1, name, name_size)) {
            *tag = tlv_tag(entry->tag, entry->size);
            return TLV_OK;
        }
    }
    return TLV_ERR_INVALID_TAG;
}
go_query_program* go_query_compile(go_format format, const uint8_t* text, size_t size,
                                   const tlv_query_compile_options_t* options,
                                   const go_query_name* names, size_t name_count,
                                   const go_query_provider* providers, size_t provider_count,
                                   const go_query_tags* tags, uintptr_t resolver, int image,
                                   tlv_result_t* code, tlv_query_diagnostic_t* diagnostic) {
    memset(diagnostic, 0, sizeof *diagnostic);
    if (!text && !size) text = (const uint8_t*)"";
    go_query_program* p = calloc(1, sizeof *p);
    if (!p) {
        for (size_t i = 0; i < provider_count; ++i) goQueryProviderRelease(providers[i].handle);
        if (tags) goQueryProviderRelease(tags->handle);
        *code = TLV_ERR_OUT_OF_MEMORY;
        return NULL;
    }
    p->references = 1;
    p->owned = format.owned;
    go_owned_format_retain(p->owned);
    if (tags) {
        p->tag_handle = tags->handle;
        p->tags = (tlv_query_tag_adapter_t){tags->id, (const void*)tags->handle,
                                            tags->has_class ? query_tag_class : NULL,
                                            tags->has_number ? query_tag_number : NULL};
    }
    *code = resolve(format, &p->format, &p->fixed);
    p->environment.format = &p->format;
    size_t                  count;
    const tlv_query_hook_t* hooks = tlv_query_builtin_hooks(&count);
    memcpy(p->hooks, hooks, count * sizeof *hooks);
#if OPENTLV_FORMAT_BER
    if (format.kind == GO_FORMAT_BER || format.kind == GO_FORMAT_BER_INDEFINITE ||
        format.kind == GO_FORMAT_DER || format.kind == GO_FORMAT_CER) {
        p->environment.tags = &tlv_asn1_query_tags;
        p->hooks[count++] = tlv_asn1_query_date;
    }
#endif
    if (tags) p->environment.tags = &p->tags;
    p->environment.hooks = p->hooks;
    p->environment.hook_count = count;
    p->provider_count = provider_count;
    for (size_t i = 0; i < provider_count; ++i) {
        const go_query_provider* provider = &providers[i];
        p->provider_handles[i] = provider->handle;
        size_t slot = 0;
        while (slot < p->environment.hook_count &&
               p->hooks[slot].function != (tlv_query_conversion_t)provider->function)
            ++slot;
        if (slot == p->environment.hook_count) ++p->environment.hook_count;
        p->hooks[slot] = (tlv_query_hook_t){provider->id,
                                            (tlv_query_conversion_t)provider->function,
                                            provider->capacity,
                                            1,
                                            (const void*)provider->handle,
                                            query_provider_decode};
    }
    tlv_query_compile_options_t config;
    tlv_query_compile_options_init(&config);
    if (options) config = *options;
    query_names lookup = {names, name_count};
    config.environment = &p->environment;
    config.resolve = query_resolve;
    config.resolve_context = &lookup;
    if (resolver) {
        config.resolve = query_dynamic_resolve;
        config.resolve_context = (const void*)resolver;
    }
    size_t bytes = 0, alignment = 0;
    p->info.struct_size = sizeof p->info;
    if (*code == TLV_OK)
        *code = image ? tlv_query_program_load_scratch(text, size, &config, &bytes, &alignment,
                                                       diagnostic)
                      : tlv_query_compile_prepare_size((const char*)text, size, &config, &bytes,
                                                       &alignment, diagnostic);
    void* scratch = *code == TLV_OK ? malloc(bytes ? bytes : 1) : NULL;
    if (*code == TLV_OK && !scratch) *code = TLV_ERR_OUT_OF_MEMORY;
    const tlv_query_program_t* prepared = NULL;
    if (*code == TLV_OK && !image)
        *code = tlv_query_compile_prepare((const char*)text, size, &config, scratch, bytes,
                                          &prepared, &p->info, diagnostic);
    if (*code == TLV_OK) {
        p->image = malloc(image ? (size ? size : 1) : p->info.program_size);
        if (!p->image) *code = TLV_ERR_OUT_OF_MEMORY;
    }
    if (*code == TLV_OK && image) {
        memcpy(p->image, text, size);
        *code = tlv_query_program_load(p->image, size, &config, scratch, bytes, &p->program,
                                       &p->info, diagnostic);
    } else if (*code == TLV_OK) {
        size_t validation_bytes = 0;
        *code = tlv_query_program_load_scratch(prepared, p->info.program_size, &config,
                                               &validation_bytes, &alignment, diagnostic);
        void* validation = *code == TLV_OK ? malloc(validation_bytes ? validation_bytes : 1) : NULL;
        if (*code == TLV_OK && !validation) *code = TLV_ERR_OUT_OF_MEMORY;
        if (*code == TLV_OK)
            *code = tlv_query_compile_commit(prepared, p->info.program_size, &config, validation,
                                             validation_bytes, p->image, p->info.program_size,
                                             &p->info, diagnostic);
        free(validation);
        p->program = p->image;
    }
    free(scratch);
    if (*code != TLV_OK) {
        go_query_program_free(p);
        return NULL;
    }
    return p;
}
void go_query_program_free(go_query_program* p) {
    if (p && --p->references == 0) {
        for (size_t i = 0; i < p->provider_count; ++i)
            goQueryProviderRelease(p->provider_handles[i]);
        if (p->tag_handle) goQueryProviderRelease(p->tag_handle);
        free(p->image);
        go_owned_format_free(p->owned);
        free(p);
    }
}
void go_query_program_retain(go_query_program* p) {
    ++p->references;
}
const tlv_query_environment_t* go_query_environment(go_query_program* p) {
    return &p->environment;
}
/* Go Format is an immutable value. For equal Fixed values, retain the native
 * operation's descriptor in a private environment; never mutate the program. */
static void query_format_environment(const tlv_format_t*            format,
                                     const tlv_query_environment_t* source,
                                     tlv_query_environment_t*       output) {
    *output = *source;
    const tlv_format_t* original = source->format;
    if (!original || !format || !original->context || !format->context ||
        original->decode != tlv_binary_decode || format->decode != tlv_binary_decode ||
        original->measure != format->measure || original->encode != format->encode ||
        original->is_constructed != format->is_constructed)
        return;
    const tlv_fixed_format_t* a = original->context;
    const tlv_fixed_format_t* b = format->context;
    if (a->identifier.size == b->identifier.size && a->length.size == b->length.size &&
        a->length.byte_order == b->length.byte_order && a->element_order == b->element_order &&
        a->length_scope == b->length_scope)
        output->format = format;
}
static tlv_result_t query_schema_environments(const tlv_format_t*            format,
                                              const tlv_schema_query_rule_t* rules, size_t count,
                                              tlv_schema_query_rule_t** copies,
                                              tlv_query_environment_t** environments) {
    *copies = NULL;
    *environments = NULL;
    if (!count) return TLV_OK;
    if (count > SIZE_MAX / sizeof **copies || count > SIZE_MAX / sizeof **environments)
        return TLV_ERR_OVERFLOW;
    *copies = calloc(count, sizeof **copies);
    *environments = calloc(count, sizeof **environments);
    if (!*copies || !*environments) {
        free(*copies);
        free(*environments);
        *copies = NULL;
        *environments = NULL;
        return TLV_ERR_OUT_OF_MEMORY;
    }
    for (size_t i = 0; i < count; ++i) {
        (*copies)[i] = rules[i];
        if (rules[i].environment) {
            query_format_environment(format, rules[i].environment, &(*environments)[i]);
            (*copies)[i].environment = &(*environments)[i];
        }
    }
    return TLV_OK;
}
tlv_result_t go_query_schema_buffer(go_format config, const uint8_t* data, size_t size,
                                    const tlv_schema_query_rule_t* rules, size_t count,
                                    size_t depth, size_t nodes, size_t work,
                                    tlv_schema_query_workspace_t*  workspace,
                                    tlv_schema_query_diagnostic_t* diagnostic) {
    tlv_format_t             format;
    tlv_fixed_format_t       fixed;
    tlv_result_t             rc = resolve(config, &format, &fixed);
    tlv_schema_query_rule_t* copies = NULL;
    tlv_query_environment_t* environments = NULL;
    if (rc == TLV_OK) rc = query_schema_environments(&format, rules, count, &copies, &environments);
    if (rc == TLV_OK)
        rc = tlv_schema_query_validate_buffer(data, size, &format, copies, count, depth, nodes,
                                              work, workspace, diagnostic);
    free(copies);
    free(environments);
    return rc;
}
tlv_result_t go_query_schema_document(go_document* document, const tlv_schema_query_rule_t* rules,
                                      size_t count, size_t depth, size_t nodes, size_t work,
                                      tlv_schema_query_workspace_t* workspace, uint8_t* values,
                                      size_t capacity, tlv_tree_writer_workspace_t* staging,
                                      tlv_schema_query_diagnostic_t* diagnostic) {
#if OPENTLV_DOCUMENT
    tlv_schema_query_rule_t* copies = NULL;
    tlv_query_environment_t* environments = NULL;
    tlv_result_t             rc =
        query_schema_environments(&document->format, rules, count, &copies, &environments);
    if (rc == TLV_OK)
        rc = tlv_schema_query_validate_document(document->document, copies, count, depth, nodes,
                                                work, workspace, values, capacity, staging,
                                                diagnostic);
    free(copies);
    free(environments);
    return rc;
#else
    (void)document;
    (void)rules;
    (void)count;
    (void)depth;
    (void)nodes;
    (void)work;
    (void)workspace;
    (void)values;
    (void)capacity;
    (void)staging;
    (void)diagnostic;
    return TLV_ERR_UNSUPPORTED;
#endif
}
const tlv_query_program_t* go_query_native(const go_query_program* p) {
    return p->program;
}
const tlv_query_program_info_t* go_query_info(const go_query_program* p) {
    return &p->info;
}
tlv_query_exec_t* go_query_exec(go_query_execution* q) {
    return q->exec;
}
static void query_buffers_free(go_query_execution* q) {
    while (q->buffers) {
        query_buffer* next = q->buffers->next;
        free(q->buffers);
        q->buffers = next;
    }
}
static query_buffer* query_buffer_new(const uint8_t* data, size_t size) {
    if (size > SIZE_MAX - sizeof(query_buffer)) return NULL;
    query_buffer* buffer = malloc(sizeof(query_buffer) + size);
    if (!buffer) return NULL;
    if (size) memcpy(buffer->data, data, size);
    buffer->next = NULL;
    return buffer;
}
tlv_result_t go_query_execution_reset(go_query_execution* q) {
    q->environment = q->program->environment;
    tlv_result_t rc = q->retained
                          ? tlv_query_eval_init(q->program->program, &q->environment, q->storage,
                                                q->capacity, q->depth, q->nodes, q->work, &q->exec)
                          : tlv_query_exec_init(q->program->program, q->storage, q->capacity,
                                                q->depth, q->nodes, q->work, &q->exec);
    if (rc == TLV_OK) {
        query_buffers_free(q);
        free(q->values);
        q->values = NULL;
        q->has_reader = q->has_document = q->fed = 0;
    }
    return rc;
}
go_query_execution* go_query_execution_create(go_query_program* p, size_t depth, size_t nodes,
                                              size_t work, int retained, tlv_result_t* code) {
    size_t bytes, alignment;
    *code = retained ? tlv_query_eval_size(p->program, depth, nodes, &bytes, &alignment)
                     : tlv_query_exec_size(p->program, depth, &bytes, &alignment);
    if (*code != TLV_OK) return NULL;
    if (depth > SIZE_MAX / sizeof(tlv_tree_frame_t)) {
        *code = TLV_ERR_OVERFLOW;
        return NULL;
    }
    go_query_execution* q = calloc(1, sizeof *q);
    if (!q) {
        *code = TLV_ERR_OUT_OF_MEMORY;
        return NULL;
    }
    q->program = p;
    ++p->references;
    q->capacity = bytes;
    q->depth = depth;
    q->nodes = nodes;
    q->work = work;
    q->retained = retained;
    if (bytes > SIZE_MAX - (alignment - 1)) {
        *code = TLV_ERR_OVERFLOW;
        go_query_execution_free(q);
        return NULL;
    }
    q->storage_allocation = malloc(bytes + alignment - 1);
    if (q->storage_allocation)
        q->storage = (void*)(((uintptr_t)q->storage_allocation + alignment - 1) &
                             ~(uintptr_t)(alignment - 1));
    q->frames = calloc(depth ? depth : 1, sizeof *q->frames);
    if (!q->storage || !q->frames)
        *code = TLV_ERR_OUT_OF_MEMORY;
    else
        *code = go_query_execution_reset(q);
    if (*code != TLV_OK) {
        go_query_execution_free(q);
        return NULL;
    }
    return q;
}
void go_query_execution_free(go_query_execution* q) {
    if (!q) return;
    query_buffers_free(q);
    free(q->storage_allocation);
    free(q->frames);
    free(q->values);
    go_query_program_free(q->program);
    free(q);
}
tlv_result_t go_query_input(go_query_execution* q, const uint8_t* data, size_t size, size_t discard,
                            int final_input) {
    if (q->has_document || q->fed) return TLV_ERR_INVALID_STATE;
    if (!q->has_reader && discard) return TLV_ERR_INVALID_ARG;
    query_buffer* buffer = query_buffer_new(data, size);
    if (!buffer) return TLV_ERR_OUT_OF_MEMORY;
    tlv_result_t rc;
    if (q->has_reader)
        rc = tlv_tree_reader_set_input(&q->reader, buffer->data, size, discard, final_input);
    else {
        rc = final_input ? tlv_tree_reader_init(&q->reader, buffer->data, size, &q->program->format,
                                                q->frames, q->depth, q->depth, q->nodes)
                         : tlv_tree_reader_init_incremental(&q->reader, buffer->data, size,
                                                            &q->program->format, q->frames,
                                                            q->depth, q->depth, q->nodes);
    }
    if (rc != TLV_OK) {
        free(buffer);
        return rc;
    }
    buffer->next = q->buffers;
    q->buffers = buffer;
    q->has_reader = 1;
    return TLV_OK;
}
tlv_result_t go_query_feed(go_query_execution* q, int kind, const uint8_t* tag, size_t tag_size,
                           const uint8_t* value, size_t value_size, size_t depth, size_t offset,
                           int skipped, tlv_tree_event_t* event, int* matched,
                           tlv_query_diagnostic_t* diagnostic) {
    if (q->has_reader || q->has_document) return TLV_ERR_INVALID_STATE;
    query_buffer* tags = query_buffer_new(tag, tag_size);
    query_buffer* values = query_buffer_new(value, value_size);
    if (!tags || !values) {
        free(tags);
        free(values);
        return TLV_ERR_OUT_OF_MEMORY;
    }
    tags->next = q->buffers;
    values->next = tags;
    q->buffers = values;
    memset(event, 0, sizeof *event);
    event->kind = (tlv_tree_event_kind_t)kind;
    event->element.tag = tlv_tag(tags->data, tag_size);
    event->element.value.data = values->data;
    event->element.value.size = value_size;
    event->depth = depth;
    event->offset = offset;
    event->skipped = skipped;
    q->fed = 1;
    tlv_result_t rc = tlv_query_exec_feed(q->exec, event, matched, diagnostic);
    if (rc == TLV_OK && *matched && q->program->info.level == TLV_QUERY_S1)
        rc = tlv_query_exec_selected(q->exec, event);
    return rc;
}
static tlv_visit_result_t query_drain(const tlv_tree_event_t* event, void* context) {
    (void)event;
    (void)context;
    return TLV_VISIT_CONTINUE;
}
tlv_result_t go_query_feed_encoded(go_query_execution* q, const uint8_t* data, size_t size,
                                   size_t depth, size_t offset, tlv_tree_event_t* event,
                                   int* matched, tlv_query_diagnostic_t* diagnostic) {
    if (q->has_reader || q->has_document) return TLV_ERR_INVALID_STATE;
    query_buffer* buffer = query_buffer_new(data, size);
    if (!buffer) return TLV_ERR_OUT_OF_MEMORY;
    tlv_reader_t            reader;
    tlv_reader_diagnostic_t original = {0};
    tlv_result_t            rc = tlv_reader_init(&reader, buffer->data, size, &q->program->format);
    memset(event, 0, sizeof *event);
    if (rc == TLV_OK)
        rc = tlv_reader_next_source_diag(&reader, &event->element, &event->source, &original);
    if (rc == TLV_OK && reader.pos != size) rc = TLV_ERR_INVALID_ARG;
    if (rc != TLV_OK) {
        /* Preserve diagnostic spans until the owning Go projection completes. */
        buffer->next = q->buffers;
        q->buffers = buffer;
        memset(diagnostic, 0, sizeof *diagnostic);
        diagnostic->kind = TLV_QUERY_ERROR_READER;
        diagnostic->cause = TLV_QUERY_CAUSE_READER;
        diagnostic->detail.reader = original.detail;
        diagnostic->diagnostic = original.diagnostic;
        diagnostic->diagnostic.code = rc;
        return rc;
    }
    event->kind =
        q->program->format.is_constructed &&
                q->program->format.is_constructed(q->program->format.context, &event->element.tag)
            ? TLV_TREE_BEGIN
            : TLV_TREE_ELEMENT;
    event->depth = depth;
    event->offset = offset;
    buffer->next = q->buffers;
    q->buffers = buffer;
    q->fed = 1;
    rc = tlv_query_exec_feed(q->exec, event, matched, diagnostic);
    if (rc == TLV_OK && *matched && q->program->info.level == TLV_QUERY_S1)
        rc = tlv_query_exec_selected(q->exec, event);
    return rc;
}
tlv_result_t go_query_finish(go_query_execution* q, tlv_query_diagnostic_t* diagnostic) {
    return q->has_reader
               ? tlv_query_program_visit(&q->reader, q->exec, query_drain, NULL, diagnostic)
               : tlv_query_exec_finish(q->exec, diagnostic);
}
tlv_result_t go_query_visit(go_query_execution* q, tlv_query_event_visitor_t visitor, void* context,
                            tlv_query_diagnostic_t* diagnostic) {
    if (!q->has_reader || q->has_document) return TLV_ERR_INVALID_STATE;
    return tlv_query_program_visit(&q->reader, q->exec, visitor, context, diagnostic);
}
tlv_result_t go_query_exists(go_query_execution* q, int early_return, int* found,
                             tlv_query_diagnostic_t* diagnostic) {
    if (!q->has_reader || q->has_document) return TLV_ERR_INVALID_STATE;
    return tlv_query_program_exists(&q->reader, q->exec, early_return, found, diagnostic);
}
tlv_result_t go_query_bind(go_query_execution* q, const char* name, tlv_query_result_kind_t type,
                           int64_t integer, const uint8_t* data, size_t size,
                           tlv_query_diagnostic_t* diagnostic) {
    query_buffer* buffer = query_buffer_new(data, size);
    if (!buffer) return TLV_ERR_OUT_OF_MEMORY;
    tlv_result_t rc =
        tlv_query_exec_bind(q->exec, name, type, integer, buffer->data, size, diagnostic);
    if (rc != TLV_OK) {
        free(buffer);
        return rc;
    }
    buffer->next = q->buffers;
    q->buffers = buffer;
    return TLV_OK;
}
tlv_result_t go_query_document(go_query_execution* q, const go_document* document, void* context,
                               size_t capacity, tlv_query_diagnostic_t* diagnostic) {
#if OPENTLV_DOCUMENT
    if (q->has_reader || q->has_document) return TLV_ERR_INVALID_STATE;
    if (!q->retained) return TLV_ERR_INVALID_ARG;
    query_format_environment(&document->format, &q->program->environment, &q->environment);
    if (q->depth == SIZE_MAX || q->depth + 1 > SIZE_MAX / sizeof(tlv_tree_writer_frame_t))
        return TLV_ERR_OVERFLOW;
    tlv_tree_writer_workspace_t staging = {0};
    if (q->program->info.constructed_values_required) {
        staging.frames = calloc(q->depth + 1, sizeof *staging.frames);
        staging.frame_capacity = q->depth + 1;
        staging.data = malloc(capacity ? capacity : 1);
        staging.data_capacity = capacity;
        staging.scratch = malloc(capacity ? capacity : 1);
        staging.scratch_capacity = capacity;
        q->values = malloc(capacity ? capacity : 1);
        if (!staging.frames || !staging.data || !staging.scratch || !q->values) {
            free(staging.frames);
            free(staging.data);
            free(staging.scratch);
            free(q->values);
            q->values = NULL;
            return TLV_ERR_OUT_OF_MEMORY;
        }
    }
    q->has_document = 1;
    tlv_result_t rc = tlv_document_query_evaluate(
        document->document, q->exec, context, q->values,
        q->program->info.constructed_values_required ? capacity : 0,
        q->program->info.constructed_values_required ? &staging : NULL, diagnostic);
    free(staging.frames);
    free(staging.data);
    free(staging.scratch);
    return rc;
#else
    (void)q;
    (void)document;
    (void)context;
    (void)capacity;
    (void)diagnostic;
    return TLV_ERR_UNSUPPORTED;
#endif
}

#if OPENTLV_DOCUMENT
extern int goDocumentVisitHandle(uintptr_t, void*);
static tlv_visit_result_t visit_document_handle(tlv_node_t* node, void* context) {
    return (tlv_visit_result_t)goDocumentVisitHandle(*(uintptr_t*)context, node);
}
#endif

tlv_result_t go_query_document_visit(go_query_execution* q, uintptr_t handle,
                                     tlv_query_diagnostic_t* diagnostic) {
#if OPENTLV_DOCUMENT
    return tlv_document_query_program_visit(q->exec, visit_document_handle, &handle, diagnostic);
#else
    (void)q; (void)handle; (void)diagnostic;
    return TLV_ERR_UNSUPPORTED;
#endif
}
tlv_result_t go_query_edit(go_query_execution* q, go_document* document, int kind,
                           const uint8_t* tag, size_t tag_size, const uint8_t* value,
                           size_t value_size, size_t capacity, size_t* applied,
                           tlv_query_diagnostic_t* diagnostic) {
    *applied = 0;
#if OPENTLV_DOCUMENT
    if (!q->has_document || !document) return TLV_ERR_INVALID_ARG;
    if (capacity > SIZE_MAX / sizeof(tlv_node_t*)) return TLV_ERR_OVERFLOW;
    tlv_node_t** targets = capacity ? calloc(capacity, sizeof *targets) : NULL;
    if (capacity && !targets) return TLV_ERR_OUT_OF_MEMORY;
    tlv_result_t rc = tlv_document_query_edit(
        document->document, q->exec, (tlv_document_query_edit_kind_t)kind, tlv_tag(tag, tag_size),
        value, value_size, targets, capacity, applied, diagnostic);
    free(targets);
    return rc;
#else
    (void)q;
    (void)document;
    (void)kind;
    (void)tag;
    (void)tag_size;
    (void)value;
    (void)value_size;
    (void)capacity;
    (void)diagnostic;
    return TLV_ERR_UNSUPPORTED;
#endif
}
tlv_result_t go_query_document_next(go_query_execution* q, void** node,
                                    tlv_query_diagnostic_t* diagnostic) {
#if OPENTLV_DOCUMENT
    tlv_node_t*  result;
    tlv_result_t rc = tlv_document_query_next(q->exec, &result, diagnostic);
    if (rc == TLV_OK) *node = result;
    return rc;
#else
    (void)q;
    (void)node;
    (void)diagnostic;
    return TLV_ERR_UNSUPPORTED;
#endif
}
uint64_t go_query_node_identity(void* node) {
#if OPENTLV_DOCUMENT
    return tlv_node_identity(node);
#else
    (void)node;
    return 0;
#endif
}
