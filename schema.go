// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// SchemaKind constrains the primitive/constructed classification.
type SchemaKind int

const (
	SchemaAny SchemaKind = iota
	SchemaPrimitive
	SchemaConstructed
)

// SchemaOrder constrains matched siblings; unknown tags are not ordered.
type SchemaOrder int

const (
	SchemaOrderAny SchemaOrder = iota
	SchemaOrderSequence
)

// SchemaUnknownPolicy overrides each scope's AllowUnknown during ValidateAll.
type SchemaUnknownPolicy int

const (
	SchemaUnknownBySchema SchemaUnknownPolicy = iota
	SchemaUnknownAllow
	SchemaUnknownReject
)

// SchemaEntry describes inclusive byte-length bounds for a tag. MaxLength -1
// means unrestricted; zero is literal. Endpoints permits only the two bounds.
// A positive LengthMultiple additionally requires divisibility.
type SchemaEntry struct {
	Tag                                  []byte
	Name                                 string
	MinLength, MaxLength, LengthMultiple int
	Endpoints                            bool
}

// ValidateLength checks only this entry's bounds through Core, without parsing.
// Tag and Name are not consulted. Definition errors match ErrInvalidSchema.
func (e SchemaEntry) ValidateLength(length int) error {
	code := capi.ValidateSchemaLength(e.MinLength, e.MaxLength, e.LengthMultiple, e.Endpoints, length)
	if code != capi.OK {
		return StatusError{code: code}
	}
	return nil
}

// StructureRule applies within one parent. MaxOccurs -1 is unrestricted; zero
// prohibits occurrences. Children requires SchemaConstructed. A nonzero Group
// selects a group in this scope and requires MinOccurs == 0.
type StructureRule struct {
	Entry                SchemaEntry
	MinOccurs, MaxOccurs int
	Kind                 SchemaKind
	Children             *StructureDefinition
	Group                uint32
}

// StructureGroup describes the combined occurrence bounds of alternative tags.
type StructureGroup struct {
	ID                   uint32
	Name                 string
	MinOccurs, MaxOccurs int
}

// StructureDefinition is a mutable construction graph. Shared and recursive
// Children pointers are supported. PrepareStructure copies the entire graph;
// subsequent changes to slices, tags, names or child definitions do not affect it.
type StructureDefinition struct {
	Rules        []StructureRule
	Groups       []StructureGroup
	AllowUnknown bool
	Order        SchemaOrder
}

// SchemaLimits bounds input traversal; zero limits are literal, not defaults.
type SchemaLimits struct{ MaxDepth, MaxElements int }

// DefaultSchemaLimits permits 64 nested levels and 65536 total elements.
func DefaultSchemaLimits() SchemaLimits { return SchemaLimits{64, 65536} }

// SchemaDetail is the shared owned Schema rule evidence also used by codecs.
type SchemaDetail = CodecSchemaDetail

// SchemaError preserves status, common metadata and the native discriminated cause.
// DefinitionTable identifies the copied table in breadth-first discovery order
// (root zero), paired with Detail.DefinitionIndex; no native address escapes.
type SchemaError struct {
	Diagnostic
	Cause           SchemaCause
	Detail          *SchemaDetail
	Reader          *Diagnostic
	Codec           *CodecDetail
	DefinitionTable OptionalSize
	status          StatusError
}

func (e *SchemaError) Error() string { return e.status.Error() }
func (e *SchemaError) Unwrap() error { return e.status }

func schemaError(code capi.Code, d capi.StructureDiagnostic) *SchemaError {
	d.Diagnostic.Code = code
	result := &SchemaError{Diagnostic: publicDiagnostic(d.Diagnostic), Cause: SchemaCause(d.Cause),
		DefinitionTable: OptionalSize{d.Table.Value, d.Table.Present}, status: StatusError{code: code}}
	if d.Schema != nil {
		result.Detail = publicCodecDetail(&capi.CodecDetail{Schema: d.Schema}).Schema
	}
	if d.Reader != nil {
		v := publicDiagnostic(*d.Reader)
		result.Reader = &v
	}
	result.Codec = publicCodecDetail(d.Codec)
	return result
}

// StructureSchema owns a prepared immutable native graph. Copies share ownership;
// Close is idempotent and synchronized with validation. The zero value is closed.
type StructureSchema struct{ native *capi.StructureSchema }

// PrepareStructure copies and checks up to 256 distinct tables using Core. It
// rejects embedded NUL names and unrepresentable Go bounds before entering C.
// The caller must not mutate the definition graph during this call.
func PrepareStructure(root *StructureDefinition) (*StructureSchema, error) {
	if root == nil {
		return nil, schemaError(capi.NullArg, capi.StructureDiagnostic{})
	}
	tables := []*StructureDefinition{root}
	indices := map[*StructureDefinition]int{root: 0}
	var definitions []capi.StructureTable
	for i := 0; i < len(tables); i++ {
		t := tables[i]
		out := capi.StructureTable{AllowUnknown: t.AllowUnknown, Order: int(t.Order)}
		for _, r := range t.Rules {
			child := -1
			if r.Children != nil {
				var ok bool
				child, ok = indices[r.Children]
				if !ok {
					if len(tables) == 256 {
						return nil, schemaError(capi.Unsupported, capi.StructureDiagnostic{})
					}
					child = len(tables)
					indices[r.Children] = child
					tables = append(tables, r.Children)
				}
			}
			out.Rules = append(out.Rules, capi.StructureRule{Tag: r.Entry.Tag, Name: r.Entry.Name,
				MinLength: r.Entry.MinLength, MaxLength: r.Entry.MaxLength, Multiple: r.Entry.LengthMultiple,
				Endpoints: r.Entry.Endpoints, MinOccurs: r.MinOccurs, MaxOccurs: r.MaxOccurs,
				Kind: int(r.Kind), Child: child, Group: r.Group})
		}
		for _, g := range t.Groups {
			out.Groups = append(out.Groups, capi.StructureGroup{ID: g.ID, Name: g.Name, MinOccurs: g.MinOccurs, MaxOccurs: g.MaxOccurs})
		}
		definitions = append(definitions, out)
	}
	native, code, diag := capi.PrepareStructure(definitions)
	if code != capi.OK {
		return nil, schemaError(code, diag)
	}
	return &StructureSchema{native}, nil
}

// Close releases the graph once, including when invoked through a copied handle.
func (s *StructureSchema) Close() {
	if s != nil && s.native != nil {
		s.native.Close()
	}
}

// Validate returns the first structural, wire or resource failure. Input is
// borrowed only for the call. Returned diagnostic tags, names and paths are owned.
func (s *StructureSchema) Validate(data []byte, format Format, limits SchemaLimits) error {
	if s == nil || s.native == nil {
		return schemaError(capi.InvalidState, capi.StructureDiagnostic{})
	}
	if !format.Valid() {
		return schemaError(capi.InvalidArg, capi.StructureDiagnostic{})
	}
	_, _, code, diag := s.native.Validate(data, format.native, limits.MaxDepth, limits.MaxElements, 0, -1)
	if code != capi.OK {
		return schemaError(code, diag)
	}
	return nil
}

// SchemaReport contains at most the requested capacity of findings and the total
// count, which may exceed len(Findings). Capacity zero performs a count-only pass.
type SchemaReport struct {
	Findings []*SchemaError
	Count    uint64
}

// ValidateAll collects bounded findings. Schema violations return ErrSchema
// alongside the report; fatal wire/definition/resource errors return an empty
// report and an owned SchemaError. Unknown policy applies in every scope.
func (s *StructureSchema) ValidateAll(data []byte, format Format, limits SchemaLimits, unknown SchemaUnknownPolicy, capacity int) (SchemaReport, error) {
	if s == nil || s.native == nil {
		return SchemaReport{}, schemaError(capi.InvalidState, capi.StructureDiagnostic{})
	}
	if !format.Valid() || capacity < 0 || unknown < SchemaUnknownBySchema || unknown > SchemaUnknownReject {
		return SchemaReport{}, schemaError(capi.InvalidArg, capi.StructureDiagnostic{})
	}
	findings, count, code, diag := s.native.Validate(data, format.native, limits.MaxDepth, limits.MaxElements, int(unknown), capacity)
	report := SchemaReport{Count: count}
	for _, d := range findings {
		report.Findings = append(report.Findings, schemaError(capi.Schema, d))
	}
	if code == capi.Schema {
		return report, StatusError{code: code}
	}
	if code != capi.OK {
		return SchemaReport{}, schemaError(code, diag)
	}
	return report, nil
}
