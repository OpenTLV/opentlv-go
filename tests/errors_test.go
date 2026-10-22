// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"bytes"
	"errors"
	"fmt"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"testing"
)

func TestCallbackAndUnsupportedStatusIdentity(t *testing.T) {
	if opentlv.ErrUnsupported.Error() != "unsupported capability" {
		t.Fatal("unsupported capability mapping differs from the native enum")
	}
	if errors.Is(opentlv.ErrCallback, opentlv.ErrInvalidValue) || opentlv.ErrCallback.Error() != "callback contract violated" {
		t.Fatal("provider defects must remain distinct from invalid data")
	}
}

func TestParseErrorSnapshotAndAbsoluteOffsets(t *testing.T) {
	f, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	// First publish and discard one element, then publish another before failing.
	r := opentlv.NewIncrementalReader([]byte{1, 0}, f)
	if !r.Next() {
		t.Fatal(r.Err())
	}
	input := []byte{2, 0, 3, 2, 42}
	if err := r.SetInput(input, 2, true); err != nil {
		t.Fatal(err)
	}
	if !r.Next() || r.Next() {
		t.Fatal("unexpected reader state")
	}
	var parsed *opentlv.ParseError
	var status opentlv.StatusError
	err := fmt.Errorf("application: %w", r.Err())
	if !errors.As(err, &parsed) || !errors.As(err, &status) || !errors.Is(err, opentlv.ErrTruncated) {
		t.Fatalf("error chain: %v", err)
	}
	if !parsed.HasOffset || parsed.Offset != 6 || !parsed.TagOffset.Present || parsed.TagOffset.Value != 4 || parsed.LengthOffset.Value != 5 || parsed.ValueOffset.Value != 6 {
		t.Fatalf("absolute diagnostic: %+v", parsed)
	}
	if !parsed.HasTag || !bytes.Equal(parsed.Tag, []byte{3}) || !parsed.HasRawLength || !bytes.Equal(parsed.RawLength, []byte{2}) || parsed.DeclaredLength.Value != 2 || parsed.Available.Value != 1 || parsed.Message == "" {
		t.Fatalf("native detail: %+v", parsed)
	}
	input[2], input[3] = 99, 99
	if parsed.Tag[0] != 3 || parsed.RawLength[0] != 2 {
		t.Fatal("diagnostic borrowed input")
	}
	if r.Next() || r.Err() != parsed {
		t.Fatal("terminal error changed")
	}
}

func TestWriteErrorAndCapacityCompatibility(t *testing.T) {
	f, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	w := opentlv.NewWriter(f)
	tag := []byte{1, 2}
	err := w.WriteElement(tag, nil)
	var written *opentlv.WriteError
	var status opentlv.StatusError
	if !errors.As(err, &written) || !errors.As(err, &status) || !errors.Is(err, opentlv.ErrInvalidTagSize) || written.Message == "" {
		t.Fatalf("writer chain: %v", err)
	}
	if !written.HasTag || !bytes.Equal(written.Tag, tag) {
		t.Fatalf("missing native writer tag: %+v", written)
	}
	tag[0] = 99
	if written.Tag[0] != 1 {
		t.Fatal("writer diagnostic borrowed tag")
	}
	if len(w.Bytes()) != 0 {
		t.Fatal("failed write published output")
	}
	w = opentlv.NewWriterBuffer(f, make([]byte, 1))
	err = w.WriteElement([]byte{1}, nil)
	var capacity opentlv.CapacityError
	if !errors.As(err, &capacity) || !errors.As(err, &status) || !errors.Is(err, opentlv.ErrBufferTooShort) || capacity.Required != 2 || capacity.Available != 1 {
		t.Fatalf("capacity chain: %v", err)
	}
	_, err = opentlv.Builtin(opentlv.FormatKind(-1))
	if !errors.Is(err, opentlv.ErrUnsupported) || errors.Is(err, opentlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}

func TestNestedDocumentFailureAndLimits(t *testing.T) {
	f, err := opentlv.Builtin(opentlv.BER)
	if errors.Is(err, opentlv.ErrUnsupported) {
		t.Skip("BER disabled")
	}
	if err != nil {
		t.Fatal(err)
	}
	d, err := opentlv.Parse([]byte{0x30, 3, 2, 2, 42}, f)
	if errors.Is(err, opentlv.ErrUnsupported) {
		t.Skip("Document disabled")
	}
	var parsed *opentlv.ParseError
	if d != nil || !errors.As(err, &parsed) || !errors.Is(err, opentlv.ErrTruncated) || !parsed.HasOffset || parsed.Offset != 4 || parsed.TagOffset.Value != 2 || !bytes.Equal(parsed.Tag, []byte{2}) {
		t.Fatalf("nested diagnostic: %v %+v", err, parsed)
	}
	d, err = opentlv.ParseWithOptions([]byte{0x30, 2, 2, 0}, f, opentlv.DocumentOptions{MaxDepth: 0, MaxElements: 10})
	if d != nil || !errors.As(err, &parsed) || !errors.Is(err, opentlv.ErrLimit) || parsed.HasOffset {
		t.Fatalf("depth limit diagnostic: %v %+v", err, parsed)
	}
	d, err = opentlv.ParseWithOptions([]byte{2, 0}, f, opentlv.DocumentOptions{MaxDepth: 0, MaxElements: 0})
	if d != nil || !errors.As(err, &parsed) || !errors.Is(err, opentlv.ErrLimit) || parsed.HasOffset {
		t.Fatalf("element limit diagnostic: %v %+v", err, parsed)
	}
}

func TestDocumentParseDiagnostics(t *testing.T) {
	f, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	input := []byte{1, 0, 2, 2, 42}
	d, err := opentlv.Parse(input, f)
	if errors.Is(err, opentlv.ErrUnsupported) {
		t.Skip("Document disabled")
	}
	var parsed *opentlv.ParseError
	if d != nil || !errors.As(err, &parsed) || !errors.Is(err, opentlv.ErrTruncated) || !parsed.HasOffset || parsed.Offset != 4 || parsed.TagOffset.Value != 2 {
		t.Fatalf("document diagnostic: %v %+v", err, parsed)
	}
	input[2], input[3] = 99, 99
	if !bytes.Equal(parsed.Tag, []byte{2}) || !bytes.Equal(parsed.RawLength, []byte{2}) {
		t.Fatal("document diagnostic borrowed input")
	}
}

func TestReaderFlowControlIsNotError(t *testing.T) {
	f, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	r := opentlv.NewIncrementalReader([]byte{1}, f)
	if r.Next() || r.Err() != nil || !r.NeedsMoreData() {
		t.Fatal("pause became error")
	}
	r = opentlv.NewReader(nil, f)
	if r.Next() || r.Err() != nil {
		t.Fatal("EOF became error")
	}
}
