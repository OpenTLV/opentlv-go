// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
#ifndef OPENTLV_GO_BRIDGE_H
#define OPENTLV_GO_BRIDGE_H
#include <tlv/config.h>
#if !OPENTLV_READER || !OPENTLV_WRITER || !OPENTLV_QUERY || !OPENTLV_SCHEMA || !OPENTLV_CODEC
#error The Go binding requires Reader, Writer, Query, Schema and Codec
#endif
#include <tlv/formats/fixed.h>
#include "fields.h"
#include <tlv/reader/reader.h>
#include <tlv/writer/writer.h>
#include <tlv/writer/tree.h>
#include <tlv/query/adapters.h>
#include <tlv/schema/query.h>
#include <tlv/schema/schema.h>
#include <tlv/codec/structure.h>
#include <tlv/generator.h>


typedef struct go_document        go_document;
typedef struct go_query_program   go_query_program;
typedef struct go_query_execution go_query_execution;
typedef struct {
    const char*    name;
    const uint8_t* tag;
    size_t         size;
} go_query_name;
typedef struct {
    uint32_t  id;
    int       function;
    size_t    capacity;
    uintptr_t handle;
} go_query_provider;
typedef struct {
    uint32_t  id;
    uintptr_t handle;
    int       has_class, has_number;
} go_query_tags;

enum go_format_kind {
    GO_FORMAT_FIXED,
    GO_FORMAT_BER,
    GO_FORMAT_BER_INDEFINITE,
    GO_FORMAT_DER,
    GO_FORMAT_CER,
    GO_FORMAT_EMV,
    GO_FORMAT_LLDP,
    GO_FORMAT_BLUETOOTH_LTV,
    GO_FORMAT_DHCPV4,
    GO_FORMAT_NFC_TYPE2,
    GO_FORMAT_OWNED
};

typedef struct {
    int    kind;
    size_t tag_size, length_size;
    int    byte_order, element_order, length_scope;
    go_owned_format* owned;
} go_format;
tlv_result_t go_structure_convert(go_format, const tlv_structure_schema_t*, size_t, size_t,
                                  const uint8_t*, size_t, uint8_t*, int,
                                  tlv_codec_diagnostic_t*);
tlv_result_t go_generate(go_format, const tlv_generator_options_t*, uint8_t*, size_t,
                         uint8_t*, size_t, size_t*);
go_query_program* go_query_compile(go_format config, const uint8_t* text, size_t size,
                                   const tlv_query_compile_options_t* options,
                                   const go_query_name* names, size_t name_count,
                                   const go_query_provider* providers, size_t provider_count,
                                   const go_query_tags* tags, uintptr_t resolver, int image,
                                   tlv_result_t* code, tlv_query_diagnostic_t* diagnostic);
void              go_query_program_free(go_query_program* program);
void              go_query_program_retain(go_query_program*);
const tlv_query_environment_t* go_query_environment(go_query_program*);
tlv_result_t go_query_schema_buffer(go_format, const uint8_t*, size_t,
                                    const tlv_schema_query_rule_t*, size_t, size_t, size_t, size_t,
                                    tlv_schema_query_workspace_t*, tlv_schema_query_diagnostic_t*);
tlv_result_t go_query_schema_document(go_document*, const tlv_schema_query_rule_t*, size_t, size_t,
                                      size_t, size_t, tlv_schema_query_workspace_t*, uint8_t*,
                                      size_t, tlv_tree_writer_workspace_t*,
                                      tlv_schema_query_diagnostic_t*);
const tlv_query_program_t*      go_query_native(const go_query_program* program);
const tlv_query_program_info_t* go_query_info(const go_query_program* program);
go_query_execution* go_query_execution_create(go_query_program* program, size_t depth, size_t nodes,
                                              size_t work, int retained, tlv_result_t* code);
void                go_query_execution_free(go_query_execution* execution);
tlv_query_exec_t*   go_query_exec(go_query_execution* execution);
tlv_result_t        go_query_execution_reset(go_query_execution* execution);
tlv_result_t        go_query_input(go_query_execution* execution, const uint8_t* data, size_t size,
                                   size_t discard, int final_input);
tlv_result_t go_query_feed(go_query_execution*, int, const uint8_t*, size_t, const uint8_t*, size_t,
                           size_t, size_t, int, tlv_tree_event_t*, int*, tlv_query_diagnostic_t*);
tlv_result_t go_query_finish(go_query_execution*, tlv_query_diagnostic_t*);
tlv_result_t go_query_feed_encoded(go_query_execution*, const uint8_t*, size_t, size_t, size_t,
                                   tlv_tree_event_t*, int*, tlv_query_diagnostic_t*);
tlv_result_t go_query_visit(go_query_execution* execution, tlv_query_event_visitor_t visitor,
                            void* context, tlv_query_diagnostic_t* diagnostic);
tlv_result_t go_query_exists(go_query_execution* execution, int early_return, int* found,
                             tlv_query_diagnostic_t* diagnostic);
tlv_result_t go_query_bind(go_query_execution* execution, const char* name,
                           tlv_query_result_kind_t type, int64_t integer, const uint8_t* data,
                           size_t size, tlv_query_diagnostic_t* diagnostic);
tlv_result_t go_query_document(go_query_execution* execution, const go_document* document,
                               void* context, size_t capacity, tlv_query_diagnostic_t* diagnostic);
tlv_result_t go_query_document_next(go_query_execution* execution, void** node,
                                    tlv_query_diagnostic_t* diagnostic);
tlv_result_t go_query_edit(go_query_execution*, go_document*, int, const uint8_t*, size_t,
                           const uint8_t*, size_t, size_t, size_t*, tlv_query_diagnostic_t*);
uint64_t     go_query_node_identity(void* node);
typedef struct {
    tlv_result_t            code;
    size_t                  consumed;
    tlv_element_t           element;
    tlv_source_t            source;
    tlv_reader_diagnostic_t diagnostic;
} go_read_result;
typedef struct {
    tlv_result_t            code;
    size_t                  size;
    tlv_writer_diagnostic_t diagnostic;
} go_write_result;

typedef struct {
    tlv_result_t code;
    tlv_tree_event_t event;
    tlv_reader_diagnostic_t diagnostic;
    size_t consumed, offset;
    int at_end;
} go_tree_read_result;

/* state contains no pointers between calls; frames contain only offsets. */
go_tree_read_result go_tree_read(go_format, tlv_tree_reader_t*, tlv_tree_frame_t*, size_t,
                                 const uint8_t*, const uint8_t*, size_t, size_t, int,
                                 size_t, size_t, int);
go_write_result go_writer_append(go_format, uint8_t*, size_t, const uint8_t*, size_t,
                                  const uint8_t*, size_t, const uint8_t*, size_t, int);

tlv_result_t    go_format_check(go_format config);
int            go_format_inspect(go_format config, int operation, const uint8_t* tag, size_t size);
tlv_result_t go_format_preserve(go_format, const uint8_t*, size_t, const uint8_t*, size_t,
                                const uint8_t*, size_t, uint8_t*, size_t);
go_write_result go_tree(go_format config, uint8_t* data, size_t capacity, const uint8_t* tag,
                        size_t tag_size, const uint8_t* value, size_t value_size, uint8_t* scratch,
                        int close);
go_read_result  go_read(go_format config, const uint8_t* data, size_t size, int final_input);
go_write_result go_write(go_format config, uint8_t* data, size_t capacity, const uint8_t* tag,
                         size_t tag_size, const uint8_t* value, size_t value_size, int measure);
go_document*    go_document_parse(go_format config, const uint8_t* data, size_t size, size_t depth,
                                  size_t elements, int defaults, int retain_source_locations,
                                  int* code, tlv_reader_diagnostic_t* diagnostic);
void            go_document_free(go_document* document);
void*           go_document_node(go_document* document, void* node, int operation);
go_read_result  go_document_read(void* node);
int go_document_edit(go_document* document, void* node, void* before, const uint8_t* tag,
                     size_t tag_size, const uint8_t* value, size_t value_size, int operation,
                     void** result);
go_write_result go_document_encode(go_document* document, go_format config, uint8_t* data,
                                   size_t capacity, int measure);
size_t          go_document_count(go_document* document);
int  go_document_query(go_document* document, const char* text, void*** nodes, size_t* count,
                       tlv_diagnostic_t* diagnostic);
void go_query_free(void** nodes);
void* go_document_find(go_document*, void*, const uint8_t*, size_t);
void go_node_location(void*, size_t*, size_t*, int*, int*);
tlv_result_t go_query_document_visit(go_query_execution*, uintptr_t, tlv_query_diagnostic_t*);
go_write_result go_node_encode(void*, go_format, uint8_t*, size_t, int);
tlv_result_t go_structure_validate(const tlv_schema_checked_t*, go_format, const uint8_t*, size_t,
                                   size_t, size_t, int, tlv_schema_diagnostic_report_t*,
                                   tlv_schema_diagnostic_t*);
#endif
