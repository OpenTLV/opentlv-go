// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"bytes"
	"errors"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"testing"
)

func TestWriterFormatsAndRoundTrip(t *testing.T) {
	for _, order := range []opentlv.ElementOrder{opentlv.TLV, opentlv.LTV} {
		f, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian, Order: order})
		if err != nil {
			t.Fatal(err)
		}
		w := opentlv.NewWriter(f)
		tag, value := []byte{7}, []byte{42, 43}
		n, err := w.Measure(tag, value)
		if err != nil || n != 4 {
			t.Fatal(n, err)
		}
		if err = w.Write(opentlv.NewElement(tag, value)); err != nil {
			t.Fatal(err)
		}
		want := []byte{7, 2, 42, 43}
		if order == opentlv.LTV {
			want = []byte{2, 7, 42, 43}
		}
		if !bytes.Equal(w.Bytes(), want) || w.Size() != 4 {
			t.Fatal(w.Bytes())
		}
		if err = w.WriteElement([]byte{8}, nil); err != nil {
			t.Fatal(err)
		}
		r := opentlv.NewReader(w.Bytes(), f)
		if !r.Next() || !bytes.Equal(r.Element().Value(), value) || !r.Next() || len(r.Element().Value()) != 0 || r.Next() || r.Err() != nil {
			t.Fatal("round trip", r.Err())
		}
	}
}

func TestWriterNestedStagingAndOwnership(t *testing.T) {
	f, err := opentlv.Builtin(opentlv.BER)
	if err != nil {
		t.Skip("BER disabled")
	}
	w := opentlv.NewWriter(f)
	if err := w.WriteElement([]byte{1}, nil); err != nil {
		t.Fatal(err)
	}
	tag := []byte{0x30}
	if err := w.Begin(tag); err != nil {
		t.Fatal(err)
	}
	tag[0] = 10
	if err := w.Begin([]byte{0x31}); err != nil {
		t.Fatal(err)
	}
	value := []byte{42}
	if err := w.Value(value); err != nil {
		t.Fatal(err)
	}
	value[0] = 0
	if w.Size() != 2 || w.Finish() == nil {
		t.Fatal("unfinished prefix")
	}
	if err := w.End(); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteElement([]byte{7}, nil); err != nil {
		t.Fatal(err)
	}
	if err := w.End(); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes(), []byte{1, 0, 0x30, 5, 0x31, 1, 42, 7, 0}) {
		t.Fatal(w.Bytes())
	}
	if w.End() == nil || w.Value(nil) == nil {
		t.Fatal("unbalanced calls")
	}
}

func TestWriterCapacityRetryAndAliasing(t *testing.T) {
	buffer := bytes.Repeat([]byte{0xaa}, 4)
	w := opentlv.NewWriterBuffer(readerFormat(t), buffer)
	if err := w.WriteElement([]byte{1}, nil); err != nil {
		t.Fatal(err)
	}
	err := w.WriteElement([]byte{2}, []byte{42})
	var capacity opentlv.CapacityError
	var status opentlv.StatusError
	if !errors.As(err, &capacity) || capacity.Required != 5 || capacity.Available != 4 || !errors.As(err, &status) {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer, []byte{1, 0, 0xaa, 0xaa}) || w.Size() != 2 {
		t.Fatal(buffer)
	}
	if err := w.SetBuffer(make([]byte, 12)); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteElement([]byte{2}, []byte{42}); err != nil {
		t.Fatal(err)
	}
	// Both tag and value alias output; staging makes the append safe.
	data := w.Bytes()
	if err := w.WriteElement(data[:1], data[2:5]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes(), []byte{1, 0, 2, 1, 42, 1, 3, 2, 1, 42}) {
		t.Fatal(w.Bytes())
	}
	before := bytes.Clone(w.Bytes())
	if w.WriteElement([]byte{1, 2}, nil) == nil || !bytes.Equal(before, w.Bytes()) {
		t.Fatal("failed write changed output")
	}
	if w.SetBuffer(make([]byte, 1)) == nil || !bytes.Equal(before, w.Bytes()) {
		t.Fatal("failed resize")
	}
}

func TestWriterInvalidAndEmpty(t *testing.T) {
	var w opentlv.Writer
	if w.WriteElement(nil, nil) == nil || w.Begin(nil) == nil || w.Value(nil) == nil || w.End() == nil || w.Finish() == nil || w.SetBuffer(nil) == nil {
		t.Fatal("zero writer")
	}
	if _, err := w.Measure(nil, nil); err == nil {
		t.Fatal("zero measure")
	}
	f := readerFormat(t)
	for _, buffer := range [][]byte{nil, {}, make([]byte, 0, 10)} {
		fixed := opentlv.NewWriterBuffer(f, buffer)
		if fixed.WriteElement([]byte{1}, nil) == nil {
			t.Fatal("empty capacity")
		}
	}
	w2 := opentlv.NewWriter(f)
	if w2.Begin([]byte{1}) == nil || w2.End() == nil || w2.Finish() != nil || w2.Size() != 0 {
		t.Fatal("primitive-only Format accepted parent")
	}
}

func TestWriterEndCapacityRetry(t *testing.T) {
	f, err := opentlv.Builtin(opentlv.BER)
	if err != nil {
		t.Skip("BER disabled")
	}
	w := opentlv.NewWriterBuffer(f, make([]byte, 2))
	if err = w.Begin([]byte{0x30}); err != nil {
		t.Fatal(err)
	}
	if err = w.WriteElement([]byte{2}, []byte{42}); err != nil {
		t.Fatal(err)
	}
	if w.End() == nil || w.Finish() == nil || w.Size() != 0 {
		t.Fatal("capacity")
	}
	if err = w.SetBuffer(make([]byte, 5)); err != nil {
		t.Fatal(err)
	}
	if err = w.End(); err != nil {
		t.Fatal(err)
	}
	if w.Finish() != nil || !bytes.Equal(w.Bytes(), []byte{0x30, 3, 2, 1, 42}) {
		t.Fatal(w.Bytes())
	}
}

func TestWriterBuiltinPresets(t *testing.T) {
	for _, kind := range []opentlv.FormatKind{opentlv.BER, opentlv.DER, opentlv.LLDP, opentlv.BluetoothLTV, opentlv.DHCPv4, opentlv.NFCType2} {
		f, err := opentlv.Builtin(kind)
		if err != nil {
			t.Logf("preset %d unavailable: %v", kind, err)
			continue
		}
		tag := []byte{2}
		if kind == opentlv.NFCType2 {
			tag = []byte{3}
		}
		w := opentlv.NewWriter(f)
		if err = w.WriteElement(tag, []byte{42}); err != nil {
			t.Fatal(kind, err)
		}
		r := opentlv.NewReader(w.Bytes(), f)
		if !r.Next() || !bytes.Equal(r.Element().Tag(), tag) || !bytes.Equal(r.Element().Value(), []byte{42}) || r.Next() || r.Err() != nil {
			t.Fatal(kind, r.Err())
		}
	}
}

func TestWriterFixedConfiguration(t *testing.T) {
	f, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 2, LengthSize: 2, ByteOrder: opentlv.LittleEndian, Order: opentlv.LTV, LengthScope: opentlv.TagAndValueLength})
	if err != nil {
		t.Fatal(err)
	}
	w := opentlv.NewWriter(f)
	if err = w.WriteElement([]byte{0x12, 0x34}, []byte{42}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes(), []byte{3, 0, 0x12, 0x34, 42}) {
		t.Fatal(w.Bytes())
	}
}

func TestWriterBERLongAndEmptyParent(t *testing.T) {
	f, err := opentlv.Builtin(opentlv.BER)
	if err != nil {
		t.Skip("BER disabled")
	}
	w := opentlv.NewWriter(f)
	if err = w.Begin([]byte{0x30}); err != nil {
		t.Fatal(err)
	}
	if err = w.End(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes(), []byte{0x30, 0}) {
		t.Fatal(w.Bytes())
	}
	value := bytes.Repeat([]byte{42}, 128)
	if err = w.Begin([]byte{0x30}); err != nil {
		t.Fatal(err)
	}
	if err = w.Value(value); err != nil {
		t.Fatal(err)
	}
	if err = w.End(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes()[2:5], []byte{0x30, 0x81, 0x80}) || !bytes.Equal(w.Bytes()[5:], value) {
		t.Fatal("long framing")
	}
	if err = w.Begin([]byte{2}); err == nil || w.Finish() != nil {
		t.Fatal("primitive parent")
	}
}
