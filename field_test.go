// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv_test

import (
	"bytes"
	"errors"
	"math"
	"testing"

	tlv "github.com/OpenTLV/opentlv-go"
)

func variableIdentifierConfig() tlv.VariableIdentifierConfig {
	return tlv.VariableIdentifierConfig{InlineMask: 0x1f, Escape: 0x1f,
		ContinuationBit: 0x80, PayloadMask: 0x7f, MaxSize: 8}
}
func variableLengthConfig() tlv.VariableLengthConfig {
	return tlv.VariableLengthConfig{LongFormBit: 0x80, PayloadMask: 0x7f, ByteOrder: tlv.BigEndian}
}
func escapedLengthConfig() tlv.EscapedLengthConfig {
	return tlv.EscapedLengthConfig{Escape: 0xff, ExtendedSize: 2, ByteOrder: tlv.BigEndian,
		MinExtended: 0xff, MaxLength: 65535}
}

func TestIdentifierFieldPublicContract(t *testing.T) {
	fixed, err := tlv.NewFixedIdentifier(2)
	if err != nil {
		t.Fatal(err)
	}
	variable, err := tlv.NewVariableIdentifier(variableIdentifierConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []tlv.IdentifierEncoding{fixed, variable} {
		if !f.Valid() {
			t.Fatal("invalid field")
		}
		data := []byte{0x1f, 0x21, 0xee}
		tag, n, err := f.Read(data)
		if err != nil || n != 2 || !bytes.Equal(tag, data[:2]) {
			t.Fatal(tag, n, err)
		}
		data[1] = 0x22
		if tag[1] != 0x22 {
			t.Fatal("identifier must borrow input")
		}
		size, err := f.Measure(tag)
		if err != nil || size != 2 {
			t.Fatal(size, err)
		}
		out := []byte{0xaa, 0xbb, 0xcc}
		n, err = f.Write(out, tag)
		if err != nil || n != size || !bytes.Equal(out, []byte{0x1f, 0x22, 0xcc}) {
			t.Fatal(out, n, err)
		}
		before := bytes.Clone(out)
		n, err = f.Write(out[:1], tag)
		if n != 0 || !errors.Is(err, tlv.ErrBufferTooShort) || !bytes.Equal(out, before) {
			t.Fatal(n, err, out)
		}
		if n, err := f.Write(nil, tag); n != 0 || !errors.Is(err, tlv.ErrBufferTooShort) {
			t.Fatal(n, err)
		}
		if _, err := f.Write(data, data[:2]); !errors.Is(err, tlv.ErrInvalidArg) {
			t.Fatal("overlap", err)
		}
		tag, n, err = f.Read([]byte{0x1f})
		var status tlv.StatusError
		if tag != nil || n != 1 || !errors.Is(err, tlv.ErrTruncated) || !errors.As(err, &status) {
			t.Fatal(tag, n, err)
		}
	}
	if f, err := tlv.NewFixedIdentifier(0); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	if f, err := tlv.NewFixedIdentifier(-1); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	if _, _, err := (tlv.IdentifierEncoding{}).Read(nil); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}

func TestLengthFieldPublicContract(t *testing.T) {
	fixed, err := tlv.NewFixedLength(8, tlv.LittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	variable, err := tlv.NewVariableLength(variableLengthConfig())
	if err != nil {
		t.Fatal(err)
	}
	escaped, err := tlv.NewEscapedLength(escapedLengthConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		f     tlv.LengthEncoding
		value uint64
		wire  []byte
	}{
		{"fixed64", fixed, math.MaxUint64, bytes.Repeat([]byte{0xff}, 8)},
		{"variable64", variable, math.MaxUint64, append([]byte{0x88}, bytes.Repeat([]byte{0xff}, 8)...)},
		{"escaped", escaped, 256, []byte{0xff, 1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.f.Valid() {
				t.Fatal("invalid field")
			}
			size, err := tc.f.Measure(tc.value)
			if err != nil || size != len(tc.wire) {
				t.Fatal(size, err)
			}
			out := bytes.Repeat([]byte{0xaa}, size+1)
			n, err := tc.f.Write(out, tc.value)
			if err != nil || n != size || !bytes.Equal(out[:n], tc.wire) || out[n] != 0xaa {
				t.Fatal(out, n, err)
			}
			value, consumed, err := tc.f.Read(out)
			if err != nil || value != tc.value || consumed != size {
				t.Fatal(value, consumed, err)
			}
			before := bytes.Clone(out)
			n, err = tc.f.Write(out[:size-1], tc.value)
			if n != 0 || !errors.Is(err, tlv.ErrBufferTooShort) || !bytes.Equal(out, before) {
				t.Fatal(out, n, err)
			}
			if n, err := tc.f.Write(nil, tc.value); n != 0 || !errors.Is(err, tlv.ErrBufferTooShort) {
				t.Fatal(n, err)
			}
			value, consumed, err = tc.f.Read(tc.wire[:size-1])
			if value != 0 || consumed != size-1 || !errors.Is(err, tlv.ErrTruncated) {
				t.Fatal(value, consumed, err)
			}
		})
	}
	if _, err := escaped.Measure(65536); !errors.Is(err, tlv.ErrInvalidLength) {
		t.Fatal(err)
	}
	if f, err := tlv.NewFixedLength(9, tlv.BigEndian); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	if _, _, err := (tlv.LengthEncoding{}).Read(nil); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if int64(int(^uint(0)>>1)) > math.MaxInt32 {
		wide := int64(1)<<32 | 1
		if _, err := tlv.NewFixedLength(1, tlv.ByteOrder(wide)); !errors.Is(err, tlv.ErrInvalidArg) {
			t.Fatal("narrowed enum", err)
		}
	}
}

func TestFieldPolicySnapshots(t *testing.T) {
	config := variableIdentifierConfig()
	policy := &tlv.IdentifierPolicy{ForbiddenLeading: []byte{0x00, 0xff, 0xff}, RequireMinimal: true}
	config.Policy = policy
	field, err := tlv.NewVariableIdentifier(config)
	if err != nil {
		t.Fatal(err)
	}
	policy.ForbiddenLeading[0] = 0x22
	policy.RequireMinimal = false
	if _, _, err := field.Read([]byte{0}); !errors.Is(err, tlv.ErrInvalidTag) {
		t.Fatal("lost copied policy", err)
	}
	if _, _, err := field.Read([]byte{0x1f, 0x01}); !errors.Is(err, tlv.ErrInvalidTag) {
		t.Fatal("lost minimality", err)
	}
	lengthConfig := variableLengthConfig()
	lengthConfig.Policy = &tlv.LengthPolicy{AllowShort: true, AllowLong: true, RequireMinimal: true,
		MaxLongOctets: 2, MaxValue: 500}
	length, err := tlv.NewVariableLength(lengthConfig)
	if err != nil {
		t.Fatal(err)
	}
	lengthConfig.Policy.MaxValue = math.MaxUint64
	lengthConfig.Policy.RequireMinimal = false
	if _, err := length.Measure(501); !errors.Is(err, tlv.ErrInvalidLength) {
		t.Fatal(err)
	}
	if _, _, err := length.Read([]byte{0x81, 1}); !errors.Is(err, tlv.ErrInvalidLength) {
		t.Fatal(err)
	}
	bad := variableIdentifierConfig()
	bad.ContinuationBit = 3
	if f, err := tlv.NewVariableIdentifier(bad); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	lengthConfig.Policy = &tlv.LengthPolicy{}
	if f, err := tlv.NewVariableLength(lengthConfig); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
	if f, err := tlv.NewEscapedLength(tlv.EscapedLengthConfig{}); f.Valid() || !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(f, err)
	}
}

func TestPackedFieldPublicContract(t *testing.T) {
	field := tlv.PackedField{StorageSize: 2, BitOffset: 4, BitWidth: 5, ByteOrder: tlv.LittleEndian}
	out := []byte{0x0f, 0xfe, 0xee}
	if err := field.Write(out, 0x13); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, []byte{0x3f, 0xff, 0xee}) {
		t.Fatal(out)
	}
	if value, err := field.Read(out); err != nil || value != 0x13 {
		t.Fatal(value, err)
	}
	before := bytes.Clone(out)
	if err := field.Write(out, 32); !errors.Is(err, tlv.ErrOverflow) || !bytes.Equal(out, before) {
		t.Fatal(err, out)
	}
	if err := field.Write(out[:1], 1); !errors.Is(err, tlv.ErrBufferTooShort) || !bytes.Equal(out, before) {
		t.Fatal(err, out)
	}
	if _, err := field.Read(nil); !errors.Is(err, tlv.ErrTruncated) {
		t.Fatal(err)
	}
	if err := field.Write(nil, 1); !errors.Is(err, tlv.ErrBufferTooShort) {
		t.Fatal(err)
	}
	field.BitWidth = -1
	if _, err := field.Read(out); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}
