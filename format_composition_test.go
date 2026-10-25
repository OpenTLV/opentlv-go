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

func genericRoundTrip(t *testing.T, f tlv.Format, tag, value []byte) []byte {
	t.Helper()
	if !f.Valid() || !f.CanRead() || !f.CanWrite() {
		t.Fatal("format capabilities")
	}
	writer := tlv.NewWriter(f)
	n, err := writer.Measure(tag, value)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteElement(tag, value); err != nil {
		t.Fatal(err)
	}
	wire := writer.Bytes()
	if n != len(wire) {
		t.Fatal("measurement", n, len(wire))
	}
	reader := tlv.NewReader(wire, f)
	if !reader.Next() {
		t.Fatal(reader.Err())
	}
	element := reader.Element()
	if !bytes.Equal(element.Tag(), tag) || !bytes.Equal(element.Value(), value) {
		t.Fatal(element)
	}
	if reader.Next() || reader.Err() != nil {
		t.Fatal("end", reader.Err())
	}
	return wire
}

func TestComposedFieldCombinations(t *testing.T) {
	fixedTag, err := tlv.NewFixedIdentifier(2)
	if err != nil {
		t.Fatal(err)
	}
	variableTag, err := tlv.NewVariableIdentifier(variableIdentifierConfig())
	if err != nil {
		t.Fatal(err)
	}
	fixedLength, err := tlv.NewFixedLength(2, tlv.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	variableLength, err := tlv.NewVariableLength(variableLengthConfig())
	if err != nil {
		t.Fatal(err)
	}
	escapedLength, err := tlv.NewEscapedLength(escapedLengthConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, identifier := range []tlv.IdentifierEncoding{fixedTag, variableTag} {
		for _, length := range []tlv.LengthEncoding{fixedLength, variableLength, escapedLength} {
			for _, order := range []tlv.ElementOrder{tlv.TLV, tlv.LTV} {
				for _, scope := range []tlv.LengthScope{tlv.ValueLength, tlv.TagAndValueLength} {
					f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: identifier, Length: length, Order: order, LengthScope: scope})
					if err != nil {
						t.Fatal(err)
					}
					wire := genericRoundTrip(t, f, []byte{0x1f, 0x21}, []byte{3, 4})
					r := tlv.NewReader(wire[:len(wire)-1], f)
					var detail *tlv.ParseError
					if r.Next() || !errors.Is(r.Err(), tlv.ErrTruncated) || !errors.As(r.Err(), &detail) {
						t.Fatal(r.Err())
					}
				}
			}
		}
	}
	if f, err := tlv.NewComposed(tlv.CompositionConfig{}); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	if f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: fixedTag, Length: fixedLength, Order: 99}); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	if (tlv.Format{}).CanRead() || (tlv.Format{}).CanWrite() || (tlv.Format{}).IsConstructed([]byte{0x20}) {
		t.Fatal("zero capabilities")
	}
}

func TestTagOnlyAndEscapedSnapshots(t *testing.T) {
	identifier, _ := tlv.NewFixedIdentifier(1)
	length, _ := tlv.NewVariableLength(variableLengthConfig())
	for _, escaped := range []bool{false, true} {
		table := [][]byte{{0}, {0xff}}
		var f tlv.Format
		var err error
		if escaped {
			f, err = tlv.NewEscaped(tlv.EscapedConfig{TagSize: 1, Length: escapedLengthConfig(), TagOnly: table})
		} else {
			f, err = tlv.NewComposed(tlv.CompositionConfig{Identifier: identifier, Length: length, TagOnly: table})
		}
		if err != nil {
			t.Fatal(err)
		}
		table[0][0] = 7
		table[1] = []byte{8}
		runtime.GC()
		wire := genericRoundTrip(t, f, []byte{0}, nil)
		if !bytes.Equal(wire, []byte{0}) {
			t.Fatal(wire)
		}
		reader := tlv.NewReader(wire, f)
		if !reader.Next() || reader.Element().Source().Length.Present {
			t.Fatal(reader.Err())
		}
		writer := tlv.NewWriter(f)
		if err := writer.WriteElement([]byte{0xff}, []byte{1}); !errors.Is(err, tlv.ErrInvalidLength) || len(writer.Bytes()) != 0 {
			t.Fatal(err)
		}
		genericRoundTrip(t, f, []byte{7}, bytes.Repeat([]byte{9}, 256))
	}
	if f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: identifier, Length: length,
		TagOnly: [][]byte{{1}}, Order: tlv.LTV}); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	if f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: identifier, Length: length,
		TagOnly: [][]byte{{1, 2}}}); f.Valid() || err == nil {
		t.Fatal(f, err)
	}
	if f, err := tlv.NewEscaped(tlv.EscapedConfig{}); f.Valid() || err == nil {
		t.Fatal(f, err)
	}
}

func TestSpecializedLayoutOptions(t *testing.T) {
	variable, err := tlv.NewVariable(tlv.VariableConfig{Identifier: variableIdentifierConfig(),
		Length: variableLengthConfig(), Order: tlv.LTV, LengthScope: tlv.TagAndValueLength})
	if err != nil {
		t.Fatal(err)
	}
	if wire := genericRoundTrip(t, variable, []byte{0x1f, 0x21}, []byte{9}); !bytes.Equal(wire, []byte{3, 0x1f, 0x21, 9}) {
		t.Fatal(wire)
	}
	escaped, err := tlv.NewEscaped(tlv.EscapedConfig{TagSize: 2,
		Length: escapedLengthConfig(), Order: tlv.LTV, LengthScope: tlv.TagAndValueLength})
	if err != nil {
		t.Fatal(err)
	}
	if wire := genericRoundTrip(t, escaped, []byte{0x1f, 0x21}, []byte{9}); !bytes.Equal(wire, []byte{3, 0x1f, 0x21, 9}) {
		t.Fatal(wire)
	}
	identifier, _ := tlv.NewFixedIdentifier(1)
	length, _ := tlv.NewFixedLength(1, tlv.BigEndian)
	predicate := &tlv.ConstructedBit{Mask: 2, Value: 2}
	mixed, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: identifier, Length: length, Constructed: predicate})
	if err != nil {
		t.Fatal(err)
	}
	config := packedConfig()
	config.Constructed = predicate
	config.LengthScope = tlv.TagAndValueLength
	packed, err := tlv.NewPacked(config)
	if err != nil {
		t.Fatal(err)
	}
	predicate.Value = 0
	for _, f := range []tlv.Format{mixed, packed} {
		if !f.IsConstructed([]byte{2}) || f.IsConstructed([]byte{1}) {
			t.Fatal("predicate snapshot")
		}
		w := tlv.NewWriter(f)
		if err := w.Begin([]byte{2}); err != nil {
			t.Fatal(err)
		}
		if err := w.WriteElement([]byte{1}, []byte{7}); err != nil {
			t.Fatal(err)
		}
		if err := w.End(); err != nil {
			t.Fatal(err)
		}
	}
	if f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: identifier, Length: length,
		Constructed: &tlv.ConstructedBit{Mask: 1, Value: 2}}); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
}

func packedConfig() tlv.PackedConfig {
	return tlv.PackedConfig{HeaderSize: 2, TagSize: 1,
		Tag:        tlv.PackedField{StorageSize: 2, BitOffset: 12, BitWidth: 2, ByteOrder: tlv.BigEndian},
		Length:     tlv.PackedField{StorageSize: 2, BitWidth: 12, ByteOrder: tlv.BigEndian},
		TagStorage: []byte{0, 1, 2, 3}}
}
func TestPackedFormatOwnership(t *testing.T) {
	config := packedConfig()
	f, err := tlv.NewPacked(config)
	if err != nil {
		t.Fatal(err)
	}
	clear(config.TagStorage)
	wire := genericRoundTrip(t, f, []byte{2}, []byte{7, 8})
	if !bytes.Equal(wire, []byte{0x20, 2, 7, 8}) {
		t.Fatal(wire)
	}
	reader := tlv.NewReader(wire, f)
	if !reader.Next() {
		t.Fatal(reader.Err())
	}
	element := reader.Element()
	if !element.Source().FormatTag {
		t.Fatal("packed tag must be format supplied")
	}
	element.Tag()[0] = 3
	genericRoundTrip(t, f, []byte{2}, nil)
	wire[2] = 9
	if element.Value()[0] != 9 {
		t.Fatal("value must still borrow input")
	}
	f = tlv.Format{}
	reader = nil
	runtime.GC()
	if element.Tag()[0] != 3 {
		t.Fatal("tag lifetime")
	}
	config = packedConfig()
	config.Length.BitOffset = 12
	if f, err := tlv.NewPacked(config); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	config = packedConfig()
	config.TagStorage = []byte{0}
	if f, err := tlv.NewPacked(config); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	config = packedConfig()
	config.TagStorage[2] = 9
	f, err = tlv.NewPacked(config)
	if err != nil {
		t.Fatal("Core checks table content on decode", err)
	}
	reader = tlv.NewReader([]byte{0x20, 0}, f)
	if reader.Next() || !errors.Is(reader.Err(), tlv.ErrInvalidTag) {
		t.Fatal(reader.Err())
	}
}

func TestVariableConstructedConsumers(t *testing.T) {
	predicate := &tlv.ConstructedBit{Mask: 0x20, Value: 0x20}
	config := tlv.VariableConfig{Identifier: variableIdentifierConfig(), Length: variableLengthConfig(), Constructed: predicate}
	f, err := tlv.NewVariable(config)
	if err != nil {
		t.Fatal(err)
	}
	predicate.Mask = 0
	if !f.IsConstructed([]byte{0x30}) || f.IsConstructed([]byte{1}) || f.IsConstructed(nil) {
		t.Fatal("predicate snapshot")
	}
	writer := tlv.NewWriter(f)
	if err := writer.Begin([]byte{0x30}); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteElement([]byte{1}, []byte{7}); err != nil {
		t.Fatal(err)
	}
	if err := writer.End(); err != nil {
		t.Fatal(err)
	}
	wire := bytes.Clone(writer.Bytes())
	program, err := tlv.CompileQuery("//01", tlv.ProgramOptions{Format: f})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := program.Execution(tlv.QueryLimits{Depth: 8, Nodes: 20, Work: 10000}, false)
	if err != nil {
		t.Fatal(err)
	}
	program.Close()
	defer execution.Close()
	doc, docErr := tlv.Parse(wire, f)
	if docErr != nil && !errors.Is(docErr, tlv.ErrUnsupported) {
		t.Fatal(docErr)
	}
	if doc != nil {
		defer doc.Close()
	}
	f, writer = tlv.Format{}, nil
	runtime.GC()
	if err := execution.SetInput(wire, 0, true); err != nil {
		t.Fatal(err)
	}
	match, err := execution.Next()
	if err != nil || !bytes.Equal(match.Element.Tag(), []byte{1}) {
		t.Fatal(match, err)
	}
	if doc != nil {
		if doc.Count() != 2 {
			t.Fatal(doc.Count())
		}
		encoded, err := doc.Encode()
		if err != nil || !bytes.Equal(encoded, wire) {
			t.Fatal(encoded, err)
		}
	}
	if f, err := tlv.NewVariable(config); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
}

func TestConcurrentComposedConsumers(t *testing.T) {
	f, err := tlv.NewPacked(packedConfig())
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 30; j++ {
				genericRoundTrip(t, f, []byte{2}, []byte{3})
				doc, err := tlv.Parse([]byte{0x20, 1, 3}, f)
				if errors.Is(err, tlv.ErrUnsupported) {
					continue
				}
				if err != nil {
					t.Error(err)
					return
				}
				if err := doc.Close(); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	group.Wait()
}

func TestFormatPreservesOriginalFraming(t *testing.T) {
	length := escapedLengthConfig()
	length.MinExtended = 0
	f, err := tlv.NewEscaped(tlv.EscapedConfig{TagSize: 1, Length: length})
	if err != nil {
		t.Fatal(err)
	}
	wire := []byte{1, 0xff, 0, 1, 7}
	reader := tlv.NewReader(wire, f)
	if !reader.Next() {
		t.Fatal(reader.Err())
	}
	element := reader.Element().Clone()
	clear(wire)
	preserved, err := f.Preserve(element)
	if err != nil || !bytes.Equal(preserved, []byte{1, 0xff, 0, 1, 7}) {
		t.Fatal(preserved, err)
	}
	writer := tlv.NewWriter(f)
	if err := writer.Write(element); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(writer.Bytes(), []byte{1, 1, 7}) {
		t.Fatal(writer.Bytes())
	}
	preserved[0] = 9
	if element.Source().Bytes[0] != 1 {
		t.Fatal("preserved output aliases source")
	}
	element.Value()[0] = 8
	if out, err := f.Preserve(element); out != nil || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(out, err)
	}
	if _, err := f.Preserve(tlv.NewElement([]byte{1}, []byte{7})); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	packed, err := tlv.NewPacked(packedConfig())
	if err != nil {
		t.Fatal(err)
	}
	reader = tlv.NewReader([]byte{0x20, 0}, packed)
	if !reader.Next() {
		t.Fatal(reader.Err())
	}
	if out, err := packed.Preserve(reader.Element()); err != nil || !bytes.Equal(out, []byte{0x20, 0}) {
		t.Fatal(out, err)
	}
}
