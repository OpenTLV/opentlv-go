// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"bytes"
	"errors"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"testing"
)

func readerFormat(t *testing.T) opentlv.Format {
	t.Helper()
	f, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestReaderSequenceAndOwnership(t *testing.T) {
	data := []byte{1, 1, 42, 2, 0}
	r := opentlv.NewReader(data, readerFormat(t))
	if !r.Next() {
		t.Fatal(r.Err())
	}
	e, owned := r.Element(), r.Element().Clone()
	if e.Offset() != 0 || !bytes.Equal(e.Source().Bytes, data[:3]) || &e.Value()[0] != &data[2] || &e.Tag()[0] != &data[0] {
		t.Fatal("source or borrowing")
	}
	if !r.Next() || r.Element().Offset() != 3 || len(r.Element().Value()) != 0 || r.Offset() != 5 {
		t.Fatal("second element")
	}
	for i := 0; i < 2; i++ {
		if r.Next() || r.Err() != nil || r.NeedsMoreData() || r.Element().Tag() != nil {
			t.Fatal("EOF")
		}
	}
	data[2] = 43
	if e.Value()[0] != 43 || owned.Value()[0] != 42 {
		t.Fatal("retained views")
	}
}

func TestReaderFormats(t *testing.T) {
	for _, kind := range []opentlv.FormatKind{opentlv.BER, opentlv.BERIndefinite, opentlv.LLDP} {
		f, err := opentlv.Builtin(kind)
		if err != nil {
			continue
		} // Optional presets may be disabled.
		data, tag, value := []byte{0x9F, 0x81, 0x01, 1, 42}, []byte{0x9F, 0x81, 0x01}, []byte{42}
		if kind == opentlv.BERIndefinite {
			data, tag, value = []byte{0x30, 0x80, 2, 1, 42, 0, 0}, []byte{0x30}, []byte{2, 1, 42}
		}
		if kind == opentlv.LLDP {
			data, tag, value = []byte{2, 1, 42}, []byte{1}, []byte{42}
		}
		r := opentlv.NewReader(data, f)
		if !r.Next() || !bytes.Equal(r.Element().Tag(), tag) || !bytes.Equal(r.Element().Value(), value) || r.Next() || r.Err() != nil {
			t.Fatalf("preset %d: %v", kind, r.Err())
		}
	}
	f, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.LittleEndian, Order: opentlv.LTV, LengthScope: opentlv.TagAndValueLength})
	if err != nil {
		t.Fatal(err)
	}
	r := opentlv.NewReader([]byte{2, 7, 42}, f)
	if !r.Next() || !bytes.Equal(r.Element().Tag(), []byte{7}) || !bytes.Equal(r.Element().Value(), []byte{42}) || r.Next() || r.Err() != nil {
		t.Fatal("LTV")
	}
}

func TestReaderTruncationAndInvalidFormat(t *testing.T) {
	r := opentlv.NewReader([]byte{1, 0, 2, 2, 42}, readerFormat(t))
	if !r.Next() || r.Next() || r.Offset() != 2 {
		t.Fatal("failure advanced cursor")
	}
	var status opentlv.StatusError
	if !errors.As(r.Err(), &status) || status.Code() == 0 || r.Next() || r.Element().Tag() != nil {
		t.Fatal("terminal error")
	}
	for _, r := range []*opentlv.Reader{opentlv.NewReader(nil, opentlv.Format{}), new(opentlv.Reader)} {
		if r.Next() || r.Err() == nil {
			t.Fatal("invalid format")
		}
	}
	for _, input := range [][]byte{nil, {}} {
		r := opentlv.NewReader(input, readerFormat(t))
		if r.Next() || r.Err() != nil {
			t.Fatal("empty EOF")
		}
	}
}

func TestReaderIncrementalEverySplit(t *testing.T) {
	data := []byte{1, 1, 42, 2, 2, 43, 44}
	for split := 0; split <= len(data); split++ {
		r := opentlv.NewIncrementalReader(data[:split], readerFormat(t))
		var offsets []int
		for r.Next() {
			offsets = append(offsets, r.Element().Offset())
		}
		if r.Err() != nil || !r.NeedsMoreData() {
			t.Fatalf("split %d: %v", split, r.Err())
		}
		consumed := r.Offset()
		window := bytes.Clone(data[consumed:])
		if err := r.SetInput(window, consumed, true); err != nil {
			t.Fatal(err)
		}
		for r.Next() {
			offsets = append(offsets, r.Element().Offset())
		}
		if r.Err() != nil || r.NeedsMoreData() || r.Offset() != len(data) || len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 3 {
			t.Fatalf("split %d: %v %v", split, offsets, r.Err())
		}
	}
}

func TestReaderInputValidationAndFinalTruncation(t *testing.T) {
	f := readerFormat(t)
	r := opentlv.NewIncrementalReader([]byte{1, 2, 42}, f)
	if r.Next() || !r.NeedsMoreData() {
		t.Fatal("expected pause")
	}
	for _, discard := range []int{-1, 1} {
		if r.SetInput([]byte{1, 2, 42}, discard, false) == nil || !r.NeedsMoreData() || r.Offset() != 0 {
			t.Fatal("invalid update changed state")
		}
	}
	if r.SetInput([]byte{1, 2}, 0, false) == nil {
		t.Fatal("shortened unread input")
	}
	if err := r.SetInput([]byte{1, 2, 42}, 0, true); err != nil {
		t.Fatal(err)
	}
	if r.Next() || r.Err() == nil || r.NeedsMoreData() {
		t.Fatal("final truncation")
	}
	r = opentlv.NewReader([]byte{1, 0}, f)
	if r.SetInput([]byte{1, 0, 2, 0}, 0, true) == nil || r.SetInput([]byte{1, 0}, 0, false) == nil {
		t.Fatal("reopened final input")
	}
	if !r.Next() || r.SetInput(nil, 2, true) != nil || r.Next() || r.Err() != nil || r.Offset() != 2 {
		t.Fatal("discard final prefix")
	}
}
