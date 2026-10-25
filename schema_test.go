// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv_test

import (
	"bytes"
	"errors"
	"runtime"
	"sync"
	"testing"

	tlv "github.com/OpenTLV/opentlv-go"
)

func structureFormat(t *testing.T) tlv.Format {
	t.Helper()
	id, err := tlv.NewFixedIdentifier(1)
	if err != nil {
		t.Fatal(err)
	}
	length, err := tlv.NewFixedLength(1, tlv.BigEndian)
	if err != nil {
		t.Fatal(err)
	}
	f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: id, Length: length, Constructed: &tlv.ConstructedBit{Mask: 0x80, Value: 0x80}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func prepareStructure(t *testing.T, d *tlv.StructureDefinition) *tlv.StructureSchema {
	t.Helper()
	s, err := tlv.PrepareStructure(d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestStructureOwnedGraphAndDiagnosticSnapshots(t *testing.T) {
	f := structureFormat(t)
	child := &tlv.StructureDefinition{Rules: []tlv.StructureRule{{Entry: tlv.SchemaEntry{Tag: []byte{1}, Name: "payload", MinLength: 2, MaxLength: 4, LengthMultiple: 2}, MinOccurs: 1, MaxOccurs: 1, Kind: tlv.SchemaPrimitive}}}
	definition := &tlv.StructureDefinition{Rules: []tlv.StructureRule{{Entry: tlv.SchemaEntry{Tag: []byte{0x80}, MaxLength: -1}, MinOccurs: 1, MaxOccurs: 1, Kind: tlv.SchemaConstructed, Children: child}}}
	s := prepareStructure(t, definition)
	child.Rules[0].Entry.Tag[0] = 9
	child.Rules[0].Entry.Name = "changed"
	child.Rules = nil
	definition.Rules = nil
	runtime.GC()
	if err := s.Validate([]byte{0x80, 4, 1, 2, 7, 8}, f, tlv.DefaultSchemaLimits()); err != nil {
		t.Fatal(err)
	}
	input := []byte{0x80, 3, 1, 1, 7}
	err := s.Validate(input, f, tlv.DefaultSchemaLimits())
	var detail *tlv.SchemaError
	if !errors.Is(err, tlv.ErrSchema) || !errors.As(err, &detail) {
		t.Fatal(err)
	}
	if detail.Cause != tlv.SchemaCauseSchema || detail.Detail == nil || detail.Detail.Kind != tlv.SchemaIssueLength || detail.Detail.Field != "payload" || !detail.Detail.HasLength || detail.Detail.ActualLength != 1 || detail.Detail.LengthMultiple != 2 || !detail.HasOffset || detail.Offset != 2 || len(detail.Path) != 1 || !bytes.Equal(detail.Path[0], []byte{0x80}) {
		t.Fatalf("%+v / %+v", detail, detail.Detail)
	}
	input[0] = 0
	input[2] = 0
	s.Close()
	runtime.GC()
	if !bytes.Equal(detail.Detail.Tag, []byte{1}) || !bytes.Equal(detail.Path[0], []byte{0x80}) || detail.Detail.Field != "payload" {
		t.Fatal("borrowed diagnostic")
	}
	if err := s.Validate(nil, f, tlv.DefaultSchemaLimits()); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
}

func TestStructureBoundedReportsPoliciesAndWireErrors(t *testing.T) {
	f := structureFormat(t)
	s := prepareStructure(t, &tlv.StructureDefinition{Rules: []tlv.StructureRule{
		{Entry: tlv.SchemaEntry{Tag: []byte{1}, MaxLength: -1}, MinOccurs: 1, MaxOccurs: 1},
		{Entry: tlv.SchemaEntry{Tag: []byte{2}, MaxLength: -1}, MinOccurs: 1, MaxOccurs: 1},
	}})
	for _, capacity := range []int{0, 1, 4} {
		report, err := s.ValidateAll(nil, f, tlv.DefaultSchemaLimits(), tlv.SchemaUnknownBySchema, capacity)
		if !errors.Is(err, tlv.ErrSchema) || report.Count != 2 || len(report.Findings) != min(capacity, 2) {
			t.Fatal(report, err)
		}
		for _, finding := range report.Findings {
			if finding.Detail.Kind != tlv.SchemaIssueMissing || finding.Location.Kind != tlv.LocationScopeEnd || finding.Location.Begin != 0 {
				t.Fatal(finding)
			}
		}
	}
	input := []byte{1, 0, 2, 0, 3, 0}
	if report, err := s.ValidateAll(input, f, tlv.DefaultSchemaLimits(), tlv.SchemaUnknownAllow, 1); err != nil || report.Count != 0 {
		t.Fatal(report, err)
	}
	report, err := s.ValidateAll(input, f, tlv.DefaultSchemaLimits(), tlv.SchemaUnknownReject, 1)
	if !errors.Is(err, tlv.ErrSchema) || report.Count != 1 || report.Findings[0].Detail.Kind != tlv.SchemaIssueUnexpected {
		t.Fatal(report, err)
	}
	input[4] = 7
	if report.Findings[0].Detail.Tag[0] != 3 {
		t.Fatal("report aliases input")
	}
	report, err = s.ValidateAll([]byte{1, 5}, f, tlv.DefaultSchemaLimits(), tlv.SchemaUnknownBySchema, 3)
	var failure *tlv.SchemaError
	if !errors.As(err, &failure) || failure.Cause != tlv.SchemaCauseReader || failure.Reader == nil || !failure.Reader.HasOffset || report.Count != 0 || len(report.Findings) != 0 {
		t.Fatal(report, err)
	}
	if err := s.Validate([]byte{1, 0}, f, tlv.SchemaLimits{}); !errors.Is(err, tlv.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := s.ValidateAll(nil, f, tlv.DefaultSchemaLimits(), tlv.SchemaUnknownBySchema, -1); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if _, err := s.ValidateAll(nil, f, tlv.DefaultSchemaLimits(), tlv.SchemaUnknownPolicy(-1), 0); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if err := s.Validate(nil, f, tlv.SchemaLimits{-1, 1}); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}

func TestStructureRecursiveSharedDefinitionsAndErrors(t *testing.T) {
	f := structureFormat(t)
	recursive := &tlv.StructureDefinition{}
	recursive.Rules = []tlv.StructureRule{{Entry: tlv.SchemaEntry{Tag: []byte{0x80}, MaxLength: -1}, MaxOccurs: -1, Kind: tlv.SchemaConstructed, Children: recursive}}
	s := prepareStructure(t, recursive)
	if err := s.Validate([]byte{0x80, 2, 0x80, 0}, f, tlv.DefaultSchemaLimits()); err != nil {
		t.Fatal(err)
	}
	bad := &tlv.StructureDefinition{Rules: []tlv.StructureRule{{Entry: tlv.SchemaEntry{Tag: []byte{1}, Name: "bad", MinLength: 2, MaxLength: 1}}}}
	root := &tlv.StructureDefinition{Rules: []tlv.StructureRule{
		{Entry: tlv.SchemaEntry{Tag: []byte{0x80}, MaxLength: -1}, Kind: tlv.SchemaConstructed, Children: bad},
		{Entry: tlv.SchemaEntry{Tag: []byte{0x81}, MaxLength: -1}, Kind: tlv.SchemaConstructed, Children: bad},
	}}
	_, err := tlv.PrepareStructure(root)
	var detail *tlv.SchemaError
	if !errors.Is(err, tlv.ErrInvalidSchema) || !errors.As(err, &detail) || detail.Detail == nil || detail.Detail.Kind != tlv.SchemaIssueDefinition || !detail.DefinitionTable.Present || detail.DefinitionTable.Value != 1 || detail.Detail.DefinitionIndex != 0 || detail.HasOffset {
		t.Fatalf("%+v", err)
	}
	bad.Rules[0].Entry.Name = "gone"
	bad.Rules[0].Entry.Tag[0] = 3
	runtime.GC()
	// Core supplies definition identity only, without an input tag or field name.
	if detail.DefinitionTable.Value != 1 || detail.Detail.DefinitionIndex != 0 || detail.Detail.Field != "" || len(detail.Detail.Tag) != 0 {
		t.Fatal("definition evidence changed")
	}
	chain := &tlv.StructureDefinition{}
	cursor := chain
	for i := 0; i < 256; i++ {
		child := &tlv.StructureDefinition{}
		cursor.Rules = []tlv.StructureRule{{Children: child}}
		cursor = child
	}
	if _, err := tlv.PrepareStructure(chain); !errors.Is(err, tlv.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := tlv.PrepareStructure(&tlv.StructureDefinition{Groups: []tlv.StructureGroup{{Name: "x\x00y"}}}); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}

func TestStructureGroupsSequenceAndLengthProjection(t *testing.T) {
	f := structureFormat(t)
	s := prepareStructure(t, &tlv.StructureDefinition{Order: tlv.SchemaOrderSequence,
		Groups: []tlv.StructureGroup{{ID: 7, Name: "choice", MinOccurs: 1, MaxOccurs: 1}},
		Rules: []tlv.StructureRule{
			{Entry: tlv.SchemaEntry{Tag: []byte{1}}, MaxOccurs: 1},
			{Entry: tlv.SchemaEntry{Tag: []byte{2}}, MaxOccurs: 1, Group: 7},
			{Entry: tlv.SchemaEntry{Tag: []byte{3}}, MaxOccurs: 1, Group: 7},
		}})
	if err := s.Validate([]byte{1, 0, 2, 0}, f, tlv.DefaultSchemaLimits()); err != nil {
		t.Fatal(err)
	}
	var detail *tlv.SchemaError
	err := s.Validate([]byte{1, 0}, f, tlv.DefaultSchemaLimits())
	if !errors.As(err, &detail) || !detail.Detail.IsGroup || detail.Detail.Field != "choice" || !detail.Detail.HasOccurs {
		t.Fatal(err)
	}
	err = s.Validate([]byte{2, 0, 1, 0}, f, tlv.DefaultSchemaLimits())
	if !errors.As(err, &detail) || detail.Detail.Kind != tlv.SchemaIssueOrder {
		t.Fatal(err)
	}
	e := tlv.SchemaEntry{MinLength: 2, MaxLength: 6, LengthMultiple: 2, Endpoints: true}
	if e.ValidateLength(2) != nil || e.ValidateLength(6) != nil || !errors.Is(e.ValidateLength(4), tlv.ErrSchema) || !errors.Is(e.ValidateLength(-1), tlv.ErrInvalidArg) {
		t.Fatal("length projection")
	}
	e.MaxLength = 1
	if !errors.Is(e.ValidateLength(2), tlv.ErrInvalidSchema) {
		t.Fatal("invalid bounds")
	}
}

func TestStructureCopiesConcurrentCloseAndZeroValue(t *testing.T) {
	f := structureFormat(t)
	s := prepareStructure(t, &tlv.StructureDefinition{AllowUnknown: true})
	alias := *s
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 30; j++ {
				if err := alias.Validate([]byte{1, 0}, f, tlv.DefaultSchemaLimits()); err != nil && !errors.Is(err, tlv.ErrInvalidState) {
					t.Error(err)
				}
			}
		}()
	}
	s.Close()
	workers.Wait()
	alias.Close()
	var zero tlv.StructureSchema
	zero.Close()
	if err := zero.Validate(nil, f, tlv.DefaultSchemaLimits()); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if _, err := tlv.PrepareStructure(nil); !errors.Is(err, tlv.ErrNullArg) {
		t.Fatal(err)
	}
}
