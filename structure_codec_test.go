// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv_test

import (
	"bytes"
	"errors"
	tlv "github.com/OpenTLV/opentlv-go"
	"runtime"
	"sync"
	"testing"
)

func TestStructureCodecValidationOwnershipAndCallbacks(t *testing.T) {
	f := structureFormat(t)
	s := prepareStructure(t, &tlv.StructureDefinition{Rules: []tlv.StructureRule{{Entry: tlv.SchemaEntry{Tag: []byte{1}, Name: "payload", MinLength: 1, MaxLength: 1}, MinOccurs: 1, MaxOccurs: 1}}})
	decodeCalls, encodeCalls := 0, 0
	codec, err := tlv.NewStructureCodec(f, s, tlv.DefaultStructureCodecOptions(), func(wire []byte) ([]byte, error) { decodeCalls++; return wire, nil }, func(wire []byte) ([]byte, error) { encodeCalls++; return wire, nil })
	if err != nil {
		t.Fatal(err)
	}
	wire := []byte{1, 1, 42}
	decoded, err := codec.Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	wire[2] = 9
	if decoded[2] != 42 {
		t.Fatal("decoded object aliases caller input")
	}
	output, err := codec.Encode(wire)
	if err != nil {
		t.Fatal(err)
	}
	wire[2] = 10
	if output[2] != 9 {
		t.Fatal("encoded output aliases callback storage")
	}
	if decodeCalls != 1 || encodeCalls != 1 {
		t.Fatal(decodeCalls, encodeCalls)
	}
	_, err = codec.Decode([]byte{1, 2, 7})
	var detail *tlv.CodecError
	if !errors.Is(err, tlv.ErrTruncated) || !errors.As(err, &detail) || detail.Detail.Reader == nil || decodeCalls != 1 {
		t.Fatalf("%+v / %d", err, decodeCalls)
	}
	if output, err = codec.Encode([]byte{1, 2, 7}); output != nil || !errors.Is(err, tlv.ErrTruncated) || !errors.As(err, &detail) || detail.Location.Domain != tlv.LocationOutput {
		t.Fatalf("%x %+v", output, err)
	}
	_, err = codec.Decode([]byte{1, 0})
	if !errors.Is(err, tlv.ErrSchema) || !errors.As(err, &detail) || detail.Detail.Schema == nil {
		t.Fatalf("%+v", err)
	}
	s.Close()
	runtime.GC()
	if detail.Detail.Schema.Field != "payload" || !bytes.Equal(detail.Detail.Schema.Tag, []byte{1}) {
		t.Fatal("diagnostic borrows schema")
	}
	copyCodec := codec
	if _, err := copyCodec.Decode(wire); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	before := encodeCalls
	if _, err := copyCodec.Encode(wire); !errors.Is(err, tlv.ErrInvalidState) || encodeCalls != before {
		t.Fatal(err, encodeCalls)
	}
}

func TestStructureCodecLimitsAndApplicationErrors(t *testing.T) {
	f := structureFormat(t)
	identity := func(w []byte) ([]byte, error) { return w, nil }
	for _, test := range []struct {
		options tlv.StructureCodecOptions
		wire    []byte
	}{
		{tlv.StructureCodecOptions{Limits: tlv.DefaultSchemaLimits(), MaxBytes: 2}, []byte{1, 1, 9}},
		{tlv.StructureCodecOptions{Limits: tlv.SchemaLimits{MaxDepth: 4, MaxElements: 0}, MaxBytes: 32}, []byte{1, 0}},
		{tlv.StructureCodecOptions{Limits: tlv.SchemaLimits{MaxDepth: 0, MaxElements: 8}, MaxBytes: 32}, []byte{0x80, 2, 1, 0}},
	} {
		c, err := tlv.NewStructureCodec(f, nil, test.options, identity, identity)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := c.Decode(test.wire); got != nil || !errors.Is(err, tlv.ErrLimit) {
			t.Fatal(got, err)
		}
		if got, err := c.Encode(test.wire); got != nil || !errors.Is(err, tlv.ErrLimit) {
			t.Fatal(got, err)
		}
	}
	application := errors.New("application conversion failed")
	fail := func([]byte) ([]byte, error) { return []byte{99}, application }
	c, err := tlv.NewStructureCodec(f, nil, tlv.DefaultStructureCodecOptions(), fail, fail)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := c.Decode(nil); got != nil || err != application {
		t.Fatal(got, err)
	}
	if got, err := c.Encode(nil); got != nil || err != application {
		t.Fatal(got, err)
	}
	var zero tlv.StructureCodec[int]
	if _, err := zero.Decode(nil); !errors.Is(err, tlv.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := zero.Encode(0); !errors.Is(err, tlv.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := tlv.NewStructureCodec(tlv.Format{}, nil, tlv.DefaultStructureCodecOptions(), identity, identity); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if _, err := tlv.NewStructureCodec(f, &tlv.StructureSchema{}, tlv.DefaultStructureCodecOptions(), identity, identity); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	empty, err := tlv.NewStructureCodec(f, nil, tlv.StructureCodecOptions{}, identity, identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := empty.Decode(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := empty.Encode(nil); err != nil {
		t.Fatal(err)
	}
	decodeOnly, err := tlv.NewStructureCodec(f, nil, tlv.DefaultStructureCodecOptions(), identity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeOnly.Encode(nil); !errors.Is(err, tlv.ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestStructureCodecReentryCloseAndConcurrentUse(t *testing.T) {
	f := structureFormat(t)
	s := prepareStructure(t, &tlv.StructureDefinition{AllowUnknown: true})
	var c tlv.StructureCodec[[]byte]
	var err error
	c, err = tlv.NewStructureCodec(f, s, tlv.DefaultStructureCodecOptions(), func(w []byte) ([]byte, error) { return c.Encode(w) }, func(w []byte) ([]byte, error) { return w, nil })
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				out, err := c.Decode([]byte{1, 0})
				if err != nil || !bytes.Equal(out, []byte{1, 0}) {
					t.Error(out, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	closing, err := tlv.NewStructureCodec[[]byte](f, s, tlv.DefaultStructureCodecOptions(), nil, func(w []byte) ([]byte, error) { s.Close(); return w, nil })
	if err != nil {
		t.Fatal(err)
	}
	if out, err := closing.Encode([]byte{1, 0}); out != nil || !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(out, err)
	}
}
