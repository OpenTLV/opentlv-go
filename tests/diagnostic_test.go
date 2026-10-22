// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"errors"
	"fmt"
	tlv "github.com/OpenTLV/opentlv-go"
	"testing"
)

func TestCanonicalDiagnosticNames(t *testing.T) {
	cases := []struct {
		value fmt.Stringer
		name  string
	}{
		{tlv.ReaderOperationHeader, "header"}, {tlv.WriterOperationEnd, "end"},
		{tlv.QueryErrorKindState, "state"}, {tlv.SchemaIssueMissing, "missing"},
		{tlv.SchemaDefinitionKindComponent, "component"}, {tlv.CodecOperationMeasure, "measure"},
		{tlv.CodecCauseReader, "reader"}, {tlv.CodecViolationUtf8, "utf8"},
		{tlv.SeverityWarning, "warning"}, {tlv.QueryErrorKind(9876), "unknown"},
	}
	for _, c := range cases {
		if got := c.value.String(); got != c.name {
			t.Fatalf("got %q, want %q", got, c.name)
		}
	}
}

func TestDiagnosticPhasesAreTyped(t *testing.T) {
	// The shared raw value is typed by the layer that produced the evidence.
	reader := tlv.Diagnostic{Operation: int(tlv.ReaderOperationHeader)}
	writer := tlv.Diagnostic{Operation: int(tlv.WriterOperationEnd)}
	if reader.ReaderPhase() != tlv.ReaderOperationHeader || writer.WriterPhase() != tlv.WriterOperationEnd {
		t.Fatalf("phases: %v %v", reader.ReaderPhase(), writer.WriterPhase())
	}
}

func TestQueryCategoryAndCommonEvidenceWithoutReader(t *testing.T) {
	format, err := tlv.Builtin(tlv.BER)
	if err != nil {
		t.Skip("BER disabled")
	}
	_, err = tlv.CompileQuery("/", tlv.ProgramOptions{Format: format})
	var failure *tlv.ProgramError
	if !errors.As(err, &failure) {
		t.Fatal(err)
	}
	if failure.Kind != tlv.QueryErrorKindSyntax || failure.HasReader ||
		failure.Location.Domain != tlv.LocationExpression || !failure.HasOffset ||
		failure.Expression.Kind != tlv.LocationUnknown {
		t.Fatalf("lost Query evidence: %#v", failure)
	}
	if _, ok := failure.SourceOffset(); ok {
		t.Fatal("an expression span is not an input offset")
	}
}
