// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package tests

import (
	"bytes"
	"errors"
	"sync"
	"testing"

	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
)

// Immutable Formats and Codecs may be shared; each consumer owns its cursors
// and output. Exercise that contract through the same facade used by examples.
func TestConcurrentPublicConsumers(t *testing.T) {
	format := readerFormat(t)
	codec := opentlv.Uint16BECodec()
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(value uint16) {
			defer workers.Done()
			for iteration := 0; iteration < 50; iteration++ {
				encoded, err := codec.Encode(value)
				if err != nil {
					t.Error(err)
					return
				}
				writer := opentlv.NewWriter(format)
				if err := writer.WriteElement([]byte{1}, encoded); err != nil {
					t.Error(err)
					return
				}
				reader := opentlv.NewReader(writer.Bytes(), format)
				if !reader.Next() {
					t.Errorf("read failed: %v", reader.Err())
					return
				}
				decoded, err := codec.Decode(reader.Element().Value())
				if err != nil || decoded != value || !bytes.Equal(reader.Element().Tag(), []byte{1}) {
					t.Errorf("round trip: %d, %v", decoded, err)
					return
				}
				if reader.Next() || reader.Err() != nil {
					t.Errorf("expected EOF: %v", reader.Err())
					return
				}
			}
		}(uint16(worker))
	}
	workers.Wait()
}

func TestReaderRejectsEveryTruncatedElementPrefix(t *testing.T) {
	for _, order := range []opentlv.ElementOrder{opentlv.TLV, opentlv.LTV} {
		for _, byteOrder := range []opentlv.ByteOrder{opentlv.BigEndian, opentlv.LittleEndian} {
			format, err := opentlv.NewFixed(opentlv.FixedConfig{
				TagSize: 2, LengthSize: 2, ByteOrder: byteOrder, Order: order,
				LengthScope: opentlv.TagAndValueLength,
			})
			if err != nil {
				t.Fatal(err)
			}
			writer := opentlv.NewWriter(format)
			if err := writer.WriteElement([]byte{1, 2}, []byte{3, 4, 5}); err != nil {
				t.Fatal(err)
			}
			wire := writer.Bytes()
			for end := 1; end < len(wire); end++ {
				reader := opentlv.NewReader(wire[:end], format)
				if reader.Next() || !errors.Is(reader.Err(), opentlv.ErrTruncated) || reader.Offset() != 0 || reader.NeedsMoreData() {
					t.Fatalf("order %v endian %v prefix %d: %v", order, byteOrder, end, reader.Err())
				}
				var detail *opentlv.ParseError
				if !errors.As(reader.Err(), &detail) || reader.Element().Tag() != nil || reader.Next() {
					t.Fatal("malformed input did not retain terminal parse error")
				}
			}
		}
	}
}

func TestReaderMalformedBER(t *testing.T) {
	format, err := opentlv.Builtin(opentlv.BER)
	if errors.Is(err, opentlv.ErrUnsupported) {
		t.Skip("BER disabled")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, wire := range [][]byte{
		{0x1f, 0x81},       // Unterminated high-tag-number encoding.
		{0x04, 0x82, 0x01}, // Truncated long-form Length.
		{0x04, 0x02, 0x01}, // Value shorter than declared Length.
		{0x04, 0x80},       // Indefinite Length on a primitive.
	} {
		reader := opentlv.NewReader(wire, format)
		var detail *opentlv.ParseError
		if reader.Next() || !errors.As(reader.Err(), &detail) || reader.Offset() != 0 || reader.NeedsMoreData() {
			t.Fatalf("malformed BER %x: %v", wire, reader.Err())
		}
	}
}
