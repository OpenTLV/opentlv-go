// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"bytes"
	"errors"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"testing"
)

func document(t *testing.T, data []byte, f opentlv.Format) *opentlv.Document {
	t.Helper()
	d, err := opentlv.Parse(data, f)
	if err != nil {
		if errors.Is(err, opentlv.ErrUnsupported) {
			t.Skip("Document disabled")
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestDocumentOwnershipMutationAndCleanup(t *testing.T) {
	f, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	input := []byte{1, 1, 42, 2, 0}
	d := document(t, input, f)
	input[2] = 99
	roots := d.Elements()
	if d.Count() != 2 || !bytes.Equal(roots[0].Value(), []byte{42}) || d.Source()[2] != 42 {
		t.Fatal("ownership")
	}
	snapshot, _ := roots[0].Element()
	source := d.Source()
	source[2] = 77
	if d.Source()[2] != 42 {
		t.Fatal("source aliases document")
	}
	if err := roots[0].SetValue([]byte{7, 8}); err != nil {
		t.Fatal(err)
	}
	if roots[0].Valid() || roots[1].Valid() || d.Source() != nil {
		t.Fatal("invalidation")
	}
	if _, err := roots[0].Element(); err == nil {
		t.Fatal("stale access")
	}
	inserted, err := d.Insert(opentlv.Node{}, opentlv.Node{}, opentlv.NewElement([]byte{3}, []byte{9}))
	if err != nil || !inserted.Valid() {
		t.Fatal(err)
	}
	if err = inserted.Erase(); err != nil {
		t.Fatal(err)
	}
	encoded, err := d.Encode()
	if err != nil || !bytes.Equal(encoded, []byte{1, 2, 7, 8, 2, 0}) {
		t.Fatal(encoded, err)
	}
	root, _ := d.Element(0)
	copyDocument := *d
	if err = copyDocument.Close(); err != nil {
		t.Fatal(err)
	}
	if root.Valid() || d.Count() != 0 || d.Elements() != nil || snapshot.Value()[0] != 42 {
		t.Fatal("close")
	}
	d.Close()
	if _, err = d.Encode(); err == nil {
		t.Fatal("closed encode")
	}
	var zero opentlv.Document
	zero.Close()
	if _, err = zero.Encode(); err == nil {
		t.Fatal("zero encode")
	}
}

func TestDocumentNestedNativeSemantics(t *testing.T) {
	f, err := opentlv.Builtin(opentlv.BER)
	if err != nil {
		t.Skip("BER disabled")
	}
	d := document(t, []byte{0x30, 0x80, 2, 1, 5, 0, 0}, f)
	root, _ := d.Element(0)
	child := root.Children()[0]
	if !root.Constructed() || child.Constructed() || !child.Parent().Valid() || root.Parent().Valid() || d.Count() != 2 {
		t.Fatal("navigation")
	}
	if err = root.SetValue([]byte{2, 2, 1}); err == nil || !child.Valid() {
		t.Fatal("failed edit must retain handles")
	}
	encoded, err := d.Encode()
	if err != nil || !bytes.Equal(encoded, []byte{0x30, 3, 2, 1, 5}) {
		t.Fatal(encoded, err)
	}
	w := opentlv.NewWriterBuffer(f, make([]byte, 1))
	if err = w.WriteDocument(d); err == nil || w.Size() != 0 {
		t.Fatal("capacity atomicity")
	}
	w = opentlv.NewWriter(f)
	if err = w.Begin([]byte{0x30}); err != nil {
		t.Fatal(err)
	}
	if err = w.WriteDocument(d); err != nil {
		t.Fatal(err)
	}
	if err = w.End(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes(), []byte{0x30, 5, 0x30, 3, 2, 1, 5}) {
		t.Fatal(w.Bytes())
	}
	if err = child.SetValue([]byte{9}); err != nil {
		t.Fatal(err)
	}
	if child.Valid() {
		t.Fatal("replaced handle")
	}
}

func TestDocumentLimitsForeignNodesAndConversion(t *testing.T) {
	f, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	d := document(t, []byte{7, 1, 42}, f)
	other := document(t, []byte{2, 0}, f)
	foreign, _ := other.Element(0)
	if _, err := d.Insert(foreign, opentlv.Node{}, opentlv.NewElement([]byte{3}, nil)); err == nil {
		t.Fatal("foreign parent")
	}
	if _, err := d.Insert(opentlv.Node{}, foreign, opentlv.NewElement([]byte{3}, nil)); err == nil {
		t.Fatal("foreign before")
	}
	if _, err := d.Element(-1); err == nil {
		t.Fatal("index")
	}
	if _, err := opentlv.Parse([]byte{1, 2, 42}, f); err == nil {
		t.Fatal("truncated input")
	}
	if _, err := opentlv.ParseWithOptions([]byte{1, 0}, f, opentlv.DocumentOptions{MaxDepth: 0, MaxElements: 0}); err == nil {
		t.Fatal("element limit")
	}
	if _, err := opentlv.ParseWithOptions(nil, f, opentlv.DocumentOptions{MaxDepth: -1}); err == nil {
		t.Fatal("negative limit")
	}
	ltv, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian, Order: opentlv.LTV})
	encoded, err := d.EncodeAs(ltv)
	if err != nil || !bytes.Equal(encoded, []byte{1, 7, 42}) {
		t.Fatal(encoded, err)
	}
	empty := document(t, nil, f)
	if b, err := empty.Encode(); err != nil || len(b) != 0 {
		t.Fatal(b, err)
	}
}
