// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv_test

import (
	"bytes"
	"errors"
	tlv "github.com/OpenTLV/opentlv-go"
	"math"
	"runtime"
	"testing"
)

func TestSchemaNumberCodecSnapshotAndConversion(t *testing.T) {
	entry := tlv.SchemaEntry{Tag: []byte{7}, Name: "counter", MinLength: 2, MaxLength: 4, Endpoints: true}
	codec := tlv.SchemaNumberCodec(entry, tlv.NumberConfig{Encoding: tlv.NumberBE})
	entry.Tag[0] = 9
	entry.Name = "changed"
	for _, test := range []struct {
		value uint64
		want  []byte
	}{{1, []byte{0, 1}}, {65536, []byte{0, 1, 0, 0}}} {
		wire, err := codec.Encode(test.value)
		if err != nil || !bytes.Equal(wire, test.want) {
			t.Fatal(wire, err)
		}
		value, err := codec.Decode(wire)
		if err != nil || value != test.value {
			t.Fatal(value, err)
		}
	}
	_, err := codec.Decode([]byte{1})
	var detail *tlv.CodecError
	if !errors.Is(err, tlv.ErrSchema) || !errors.As(err, &detail) || detail.Detail == nil || detail.Detail.Schema == nil {
		t.Fatalf("%+v", err)
	}
	runtime.GC()
	if detail.Detail.Schema.Field != "counter" || !bytes.Equal(detail.Detail.Schema.Tag, []byte{7}) || detail.Detail.Schema.ActualLength != 1 {
		t.Fatalf("%+v", detail.Detail.Schema)
	}
	if data, err := codec.Encode(math.MaxUint64); data != nil || !errors.Is(err, tlv.ErrSchema) {
		t.Fatal(data, err)
	}
	fixed := tlv.SchemaNumberCodec(tlv.SchemaEntry{MinLength: 2, MaxLength: 4}, tlv.NumberConfig{Width: 1})
	if _, err := fixed.Encode(1); !errors.Is(err, tlv.ErrSchema) {
		t.Fatal(err)
	}
	for _, encoding := range []tlv.NumberEncoding{tlv.NumberLE, tlv.NumberBCD} {
		config := tlv.NumberConfig{Encoding: encoding}
		if encoding == tlv.NumberBCD {
			config.Digits = 6
		}
		c := tlv.SchemaNumberCodec(tlv.SchemaEntry{MinLength: 2, MaxLength: 4, LengthMultiple: 2}, config)
		data, err := c.Encode(123)
		if err != nil || len(data) != 2 {
			t.Fatal(data, err)
		}
		v, err := c.Decode(data)
		if err != nil || v != 123 {
			t.Fatal(v, err)
		}
	}
	invalid := tlv.SchemaNumberCodec(tlv.SchemaEntry{MinLength: 5, MaxLength: 2}, tlv.NumberConfig{})
	if _, err := invalid.Decode([]byte{1}); !errors.Is(err, tlv.ErrInvalidSchema) {
		t.Fatal(err)
	}
	for _, e := range []tlv.SchemaEntry{{MaxLength: -2}, {Name: "bad\x00name", MaxLength: 8}} {
		if _, err := tlv.SchemaNumberCodec(e, tlv.NumberConfig{}).Encode(1); !errors.Is(err, tlv.ErrInvalidArg) {
			t.Fatal(err)
		}
	}
}

func TestValueConstraintOwnedDefinitionsAndNames(t *testing.T) {
	cfg := tlv.ValueConstraintConfig{Kind: tlv.ValueConstraintAllowedValues, AllowedValues: []int64{-2, 0, math.MaxInt64}, Names: []string{"negative", "", "maximum"}}
	c, err := tlv.NewValueConstraint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AllowedValues[0] = 5
	cfg.Names[0] = "changed"
	runtime.GC()
	if err := c.Validate(-2); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(5); !errors.Is(err, tlv.ErrSchema) {
		t.Fatal(err)
	}
	name, found, err := c.Name(-2)
	if err != nil || !found || name != "negative" {
		t.Fatal(name, found, err)
	}
	name, found, err = c.Name(0)
	if err != nil || !found || name != "" {
		t.Fatal(name, found, err)
	}
	_, found, err = c.Name(12)
	if err != nil || found {
		t.Fatal(found, err)
	}
	unnamed, err := tlv.NewValueConstraint(tlv.ValueConstraintConfig{Kind: tlv.ValueConstraintAllowedValues, AllowedValues: []int64{1}})
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = unnamed.Name(1)
	if err != nil || found {
		t.Fatal(found, err)
	}
	var none tlv.ValueConstraint
	if err := none.Validate(math.MinInt64); err != nil {
		t.Fatal(err)
	}
	ranged, err := tlv.NewValueConstraint(tlv.ValueConstraintConfig{Kind: tlv.ValueConstraintRange, MinValue: -1, MaxValue: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []int64{-1, 0, 1} {
		if err := ranged.Validate(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := ranged.Validate(2); !errors.Is(err, tlv.ErrSchema) {
		t.Fatal(err)
	}
	empty, err := tlv.NewValueConstraint(tlv.ValueConstraintConfig{Kind: tlv.ValueConstraintAllowedValues})
	if err != nil {
		t.Fatal(err)
	}
	if err := empty.Validate(0); !errors.Is(err, tlv.ErrSchema) {
		t.Fatal(err)
	}
	for _, cfg := range []tlv.ValueConstraintConfig{{Kind: 99}, {Kind: tlv.ValueConstraintRange, MinValue: 3, MaxValue: 1}} {
		if _, err := tlv.NewValueConstraint(cfg); !errors.Is(err, tlv.ErrInvalidSchema) {
			t.Fatal(err)
		}
	}
	for _, names := range [][]string{{"extra"}, {"bad\x00name", ""}} {
		if _, err := tlv.NewValueConstraint(tlv.ValueConstraintConfig{Kind: tlv.ValueConstraintAllowedValues, AllowedValues: []int64{1, 2}, Names: names}); !errors.Is(err, tlv.ErrInvalidArg) {
			t.Fatal(err)
		}
	}
}
