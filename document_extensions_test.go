// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv_test

import (
	"bytes"
	"errors"
	"runtime"
	"testing"

	tlv "github.com/OpenTLV/opentlv-go"
)

func TestNodeSubtreeEncodingNavigationAndSource(t *testing.T) {
	f := structureFormat(t)
	wire := []byte{0x80, 6, 1, 1, 7, 1, 1, 8, 2, 0}
	d, err := tlv.ParseWithOptions(wire, f, tlv.DocumentOptions{MaxDepth: 8, MaxElements: 20, RetainSourceLocations: true})
	if errors.Is(err, tlv.ErrUnsupported) {
		t.Skip("Document disabled")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	parent, err := d.Find(tlv.Node{}, []byte{0x80})
	if err != nil || !parent.Valid() {
		t.Fatal(parent, err)
	}
	first, err := d.Find(parent, []byte{1})
	if err != nil || !first.Valid() {
		t.Fatal(first, err)
	}
	second := first.NextSameTag()
	if !second.Valid() || second.Value()[0] != 8 || second.NextSameTag().Valid() || second.Next().Valid() || parent.Next().Tag()[0] != 2 {
		t.Fatal("navigation")
	}
	missing, err := d.Find(parent, []byte{9})
	if err != nil || missing.Valid() {
		t.Fatal(missing, err)
	}
	location, err := first.SourceLocation()
	if err != nil || location.Offset != (tlv.OptionalSize{Value: 2, Present: true}) || location.HeaderSize != (tlv.OptionalSize{Value: 2, Present: true}) {
		t.Fatal(location, err)
	}
	encoded, err := parent.Encode()
	if err != nil || !bytes.Equal(encoded, wire[:8]) {
		t.Fatal(encoded, err)
	}
	size, err := parent.EncodedSize()
	if err != nil || size != len(encoded) {
		t.Fatal(size, err)
	}
	if size, err := d.EncodedSize(); err != nil || size != len(wire) {
		t.Fatal(size, err)
	}
	id, _ := tlv.NewFixedIdentifier(1)
	length, _ := tlv.NewFixedLength(2, tlv.LittleEndian)
	destination, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: id, Length: length, Constructed: &tlv.ConstructedBit{Mask: 0x80, Value: 0x80}})
	if err != nil {
		t.Fatal(err)
	}
	converted, err := parent.EncodeAs(destination)
	if err != nil || !bytes.Equal(converted, []byte{0x80, 8, 0, 1, 1, 0, 7, 1, 1, 0, 8}) {
		t.Fatal(converted, err)
	}
	if size, err := parent.EncodedSizeAs(destination); err != nil || size != len(converted) {
		t.Fatal(size, err)
	}
	if size, err := d.EncodedSizeAs(destination); err != nil || size != len(converted)+3 {
		t.Fatal(size, err)
	}
	other := document(t, []byte{1, 0}, f)
	foreign, _ := other.Element(0)
	if _, err := d.Find(foreign, []byte{1}); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if err := first.SetValue([]byte{9}); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Encode(); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if _, err := d.Find(parent, []byte{1}); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if _, err := first.SourceLocation(); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	parent, _ = d.Element(0)
	newFirst := parent.Children()[0]
	newSecond := parent.Children()[1]
	a, _ := parent.SourceLocation()
	b, _ := newFirst.SourceLocation()
	c, _ := newSecond.SourceLocation()
	if a.Offset.Present || b.Offset.Present || !c.Offset.Present || c.Offset.Value != 5 {
		t.Fatal(a, b, c)
	}
	d.Close()
	runtime.GC()
	if location.Offset.Value != 2 || encoded[4] != 7 {
		t.Fatal("snapshots")
	}
	if _, err := parent.EncodedSize(); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if _, err := d.EncodedSize(); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	var zero tlv.Node
	if zero.Next().Valid() || zero.NextSameTag().Valid() {
		t.Fatal("zero node")
	}
}

func TestDocumentQueryVisitorResumeGuardsAndUnwind(t *testing.T) {
	f := structureFormat(t)
	d := document(t, []byte{1, 1, 7, 1, 1, 8, 1, 1, 9}, f)
	p, err := tlv.CompileQuery("//01", tlv.ProgramOptions{Format: f, Optimize: true})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	q, err := p.Execution(tlv.DefaultQueryLimits(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	capacity, err := d.EncodedSize()
	if err != nil {
		t.Fatal(err)
	}
	if err := q.EvaluateDocument(d, tlv.Node{}, capacity); err != nil {
		t.Fatal(err)
	}
	p.Close() // Execution retains the native program.
	if err := q.VisitDocument(nil); !errors.Is(err, tlv.ErrNullArg) {
		t.Fatal(err)
	}
	var values []byte
	err = q.VisitDocument(func(n tlv.Node) tlv.QueryVisit {
		values = append(values, n.Value()[0])
		if err := n.SetValue([]byte{0}); !errors.Is(err, tlv.ErrInvalidState) {
			t.Fatal(err)
		}
		if err := d.Close(); !errors.Is(err, tlv.ErrInvalidState) {
			t.Fatal(err)
		}
		if err := q.Reset(); !errors.Is(err, tlv.ErrInvalidState) {
			t.Fatal(err)
		}
		if err := q.Close(); !errors.Is(err, tlv.ErrInvalidState) {
			t.Fatal(err)
		}
		if _, err := q.NextDocument(); !errors.Is(err, tlv.ErrInvalidState) {
			t.Fatal(err)
		}
		if err := q.VisitDocument(func(tlv.Node) tlv.QueryVisit { return tlv.QueryContinue }); !errors.Is(err, tlv.ErrInvalidState) {
			t.Fatal(err)
		}
		return tlv.QueryStop
	})
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() != "application" {
				t.Fatal("panic lost")
			}
		}()
		_ = q.VisitDocument(func(n tlv.Node) tlv.QueryVisit { values = append(values, n.Value()[0]); panic("application") })
	}()
	if _, err := q.NextDocument(); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if err := q.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := q.EvaluateDocument(d, tlv.Node{}, 0); err != nil {
		t.Fatal(err)
	}
	values = nil
	if err := q.VisitDocument(func(n tlv.Node) tlv.QueryVisit { values = append(values, n.Value()[0]); return tlv.QueryContinue }); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(values, []byte{7, 8, 9}) {
		t.Fatal(values)
	}
	if _, err := q.NextDocument(); !errors.Is(err, tlv.ErrEnd) {
		t.Fatal(err)
	}
	if err := q.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := q.EvaluateDocument(d, tlv.Node{}, 0); err != nil {
		t.Fatal(err)
	}
	if err := q.VisitDocument(func(tlv.Node) tlv.QueryVisit { return tlv.QueryVisitorError }); !errors.Is(err, tlv.ErrVisitor) {
		t.Fatal(err)
	} else {
		var detail *tlv.ProgramError
		if !errors.As(err, &detail) || detail.Kind != tlv.QueryErrorKindCallback {
			t.Fatal(err)
		}
	}
	if _, err := q.NextDocument(); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if err := q.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := q.EvaluateDocument(d, tlv.Node{}, 0); err != nil {
		t.Fatal(err)
	}
	if err := q.VisitDocument(func(tlv.Node) tlv.QueryVisit { return tlv.QueryVisit(-1) }); !errors.Is(err, tlv.ErrCallback) {
		t.Fatal(err)
	}
	if err := q.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := q.EvaluateDocument(d, tlv.Node{}, 0); err != nil {
		t.Fatal(err)
	}
	n, _ := d.Element(0)
	if err := n.SetValue([]byte{4}); err != nil {
		t.Fatal(err)
	}
	if err := q.VisitDocument(func(tlv.Node) tlv.QueryVisit { t.Fatal("stale callback"); return tlv.QueryContinue }); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
}
