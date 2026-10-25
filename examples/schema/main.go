// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package main

import (
	"errors"
	"fmt"

	tlv "github.com/OpenTLV/opentlv-go"
)

func main() {
	f, err := tlv.NewFixed(tlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: tlv.BigEndian})
	check(err)
	s, err := tlv.PrepareStructure(&tlv.StructureDefinition{Rules: []tlv.StructureRule{{
		Entry:     tlv.SchemaEntry{Tag: []byte{1}, Name: "payload", MinLength: 2, MaxLength: 8, LengthMultiple: 2},
		MinOccurs: 1, MaxOccurs: 1, Kind: tlv.SchemaPrimitive,
	}}})
	check(err)
	defer s.Close()
	check(s.Validate([]byte{1, 2, 7, 8}, f, tlv.DefaultSchemaLimits()))
	report, err := s.ValidateAll([]byte{1, 1, 7}, f, tlv.DefaultSchemaLimits(), tlv.SchemaUnknownBySchema, 4)
	if !errors.Is(err, tlv.ErrSchema) || report.Count != 1 || len(report.Findings) != 1 {
		panic("expected one schema finding")
	}
	finding := report.Findings[0]
	fmt.Printf("%s: %s at %s (length %d)\n", finding.Detail.Field, finding.Detail.Kind, finding.Location, finding.Detail.ActualLength)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
