// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

// Demonstrate mixed fields, tag-only framing and native packed headers.
package main

import (
	"fmt"
	"log"

	tlv "github.com/OpenTLV/opentlv-go"
)

func roundTrip(format tlv.Format, tag, value []byte) {
	writer := tlv.NewWriter(format)
	if err := writer.WriteElement(tag, value); err != nil {
		log.Fatal(err)
	}
	reader := tlv.NewReader(writer.Bytes(), format)
	if !reader.Next() {
		log.Fatal(reader.Err())
	}
	fmt.Printf("wire=%x tag=%x value=%x\n", writer.Bytes(), reader.Element().Tag(), reader.Element().Value())
}

func main() {
	identifier, err := tlv.NewFixedIdentifier(1)
	if err != nil {
		log.Fatal(err)
	}
	length, err := tlv.NewVariableLength(tlv.VariableLengthConfig{
		LongFormBit: 0x80, PayloadMask: 0x7f, ByteOrder: tlv.BigEndian,
		Policy: &tlv.LengthPolicy{AllowShort: true, AllowLong: true, RequireMinimal: true,
			MaxLongOctets: 2, MaxValue: 65535},
	})
	if err != nil {
		log.Fatal(err)
	}
	mixed, err := tlv.NewComposed(tlv.CompositionConfig{
		Identifier: identifier, Length: length, TagOnly: [][]byte{{0}},
	})
	if err != nil {
		log.Fatal(err)
	}
	roundTrip(mixed, []byte{1}, []byte("Go"))
	roundTrip(mixed, []byte{0}, nil)

	packed, err := tlv.NewPacked(tlv.PackedConfig{
		HeaderSize: 1, TagSize: 1, TagStorage: []byte{0, 1, 2, 3},
		Tag:    tlv.PackedField{StorageSize: 1, BitOffset: 6, BitWidth: 2, ByteOrder: tlv.BigEndian},
		Length: tlv.PackedField{StorageSize: 1, BitWidth: 6, ByteOrder: tlv.BigEndian},
	})
	if err != nil {
		log.Fatal(err)
	}
	roundTrip(packed, []byte{2}, []byte("Go"))
}
