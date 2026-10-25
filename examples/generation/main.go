// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
// Demonstrate generic object conversion and reproducible wire generation.
package main

import (
	"bytes"
	"fmt"
	tlv "github.com/OpenTLV/opentlv-go"
	"log"
)

func run() error {
	format, err := tlv.NewFixed(tlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: tlv.BigEndian})
	if err != nil {
		return err
	}
	entry := tlv.SchemaEntry{Tag: []byte{1}, Name: "counter", MinLength: 2, MaxLength: 2}
	number := tlv.SchemaNumberCodec(entry, tlv.NumberConfig{Encoding: tlv.NumberBE})
	constraint, err := tlv.NewValueConstraint(tlv.ValueConstraintConfig{Kind: tlv.ValueConstraintRange, MaxValue: 100})
	if err != nil {
		return err
	}
	schema, err := tlv.PrepareStructure(&tlv.StructureDefinition{Rules: []tlv.StructureRule{{Entry: entry, MinOccurs: 1, MaxOccurs: 1, Kind: tlv.SchemaPrimitive}}})
	if err != nil {
		return err
	}
	defer schema.Close()
	codec, err := tlv.NewStructureCodec(format, schema, tlv.DefaultStructureCodecOptions(),
		func(wire []byte) (uint64, error) {
			reader := tlv.NewReader(wire, format)
			if !reader.Next() {
				return 0, reader.Err()
			}
			value, err := number.Decode(reader.Element().Value())
			if err != nil {
				return 0, err
			}
			return value, constraint.Validate(int64(value))
		}, func(value uint64) ([]byte, error) {
			if value > 100 {
				return nil, tlv.ErrSchema
			}
			if err := constraint.Validate(int64(value)); err != nil {
				return nil, err
			}
			valueBytes, err := number.Encode(value)
			if err != nil {
				return nil, err
			}
			writer := tlv.NewWriter(format)
			if err := writer.WriteElement(entry.Tag, valueBytes); err != nil {
				return nil, err
			}
			return writer.Bytes(), nil
		})
	if err != nil {
		return err
	}
	wire, err := codec.Encode(42)
	if err != nil {
		return err
	}
	value, err := codec.Decode(wire)
	if err != nil {
		return err
	}
	if value != 42 {
		return fmt.Errorf("unexpected counter %d", value)
	}
	fmt.Printf("counter=%d wire=%x\n", value, wire)

	options := tlv.DefaultGeneratorOptions()
	options.Seed = 42
	options.Candidates = []tlv.GeneratorCandidate{{Tag: []byte{1}, MinValueSize: 2, MaxValueSize: 2}}
	generator, err := tlv.NewGenerator(format, options)
	if err != nil {
		return err
	}
	first, err := generator.Generate(7)
	if err != nil {
		return err
	}
	repeated, err := generator.Generate(7)
	if err != nil {
		return err
	}
	if !bytes.Equal(first, repeated) {
		return fmt.Errorf("case is not reproducible")
	}
	// Arbitrary generated Values need not satisfy the application constraint.
	fmt.Printf("generator=v%d seed=42 case=7 bytes=%d reproducible=true\n", tlv.GeneratorVersion, len(first))
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
