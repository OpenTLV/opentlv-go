// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package tests

import (
	"bytes"
	"errors"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"reflect"
	"testing"
)

func TestQueryDocumentEditsAndErrors(t *testing.T) {
	f, _ := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	d := document(t, []byte{1, 1, 42, 2, 0, 1, 1, 7}, f)
	nodes, err := d.Query("01")
	if err != nil || len(nodes) != 2 || nodes[1].Value()[0] != 7 {
		t.Fatal(nodes, err)
	}
	snapshot := nodes[0].Value()
	if err = nodes[0].SetValue([]byte{99}); err != nil {
		t.Fatal(err)
	}
	if nodes[0].Valid() {
		t.Fatal("stale query handle")
	}
	nodes, err = d.Query("01")
	if err != nil || nodes[0].Value()[0] != 99 || snapshot[0] != 42 {
		t.Fatal("edited tree", err)
	}
	empty, err := d.Query("FF")
	if err != nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
	for _, path := range []string{"", "//01", "01/", "0Z", "01\x00FF"} {
		_, err = d.Query(path)
		var detail *opentlv.QueryError
		if !errors.Is(err, opentlv.ErrSyntax) || !errors.As(err, &detail) || !detail.HasOffset {
			t.Fatal(path, err)
		}
	}
	d.Close()
	if nodes[0].Valid() {
		t.Fatal("closed query handle")
	}
	if _, err = d.Query("01"); !errors.Is(err, opentlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}

func TestQueryNestedDuplicateBranches(t *testing.T) {
	f, err := opentlv.Builtin(opentlv.BER)
	if errors.Is(err, opentlv.ErrUnsupported) {
		t.Skip("BER disabled")
	}
	if err != nil {
		t.Fatal(err)
	}
	d := document(t, []byte{0x30, 3, 4, 1, 1, 0x30, 3, 4, 1, 2}, f)
	nodes, err := d.Query("30/04")
	if err != nil || len(nodes) != 2 || nodes[0].Value()[0] != 1 || nodes[1].Value()[0] != 2 {
		t.Fatal(nodes, err)
	}
}

func checkCodec[T any](t *testing.T, c opentlv.Codec[T], wire []byte, want T) {
	t.Helper()
	got, err := c.Decode(wire)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("decode %x: %v %v", wire, got, err)
	}
	encoded, err := c.Encode(want)
	if err != nil || !bytes.Equal(encoded, wire) {
		t.Fatalf("encode: %x %v", encoded, err)
	}
}
func TestValueCodecs(t *testing.T) {
	checkCodec(t, opentlv.Uint8Codec(), []byte{255}, uint8(255))
	checkCodec(t, opentlv.Uint16BECodec(), []byte{0x12, 0x34}, uint16(0x1234))
	checkCodec(t, opentlv.Uint16LECodec(), []byte{0x34, 0x12}, uint16(0x1234))
	checkCodec(t, opentlv.Uint32BECodec(), []byte{1, 2, 3, 4}, uint32(0x01020304))
	checkCodec(t, opentlv.Uint32LECodec(), []byte{4, 3, 2, 1}, uint32(0x01020304))
	checkCodec(t, opentlv.Int64Codec(), []byte{0xff, 0x7f}, int64(-129))
	checkCodec(t, opentlv.NumberCodec(opentlv.NumberConfig{Encoding: opentlv.NumberBE, Width: 8}), []byte{255, 255, 255, 255, 255, 255, 255, 255}, ^uint64(0))
	checkCodec(t, opentlv.NumberCodec(opentlv.NumberConfig{Encoding: opentlv.NumberLE, Width: 2}), []byte{0x34, 0x12}, uint64(0x1234))
	checkCodec(t, opentlv.NumberCodec(opentlv.NumberConfig{Encoding: opentlv.NumberBCD, Width: 3, Digits: 6}), []byte{0x00, 0x12, 0x34}, uint64(1234))
	checkCodec(t, opentlv.TextCodec(opentlv.TextConfig{Width: 4, ZeroPadding: true}), []byte{'A', 'B', 0, 0}, "AB")
	checkCodec(t, opentlv.DigitsCodec(3), []byte{0x00, 0x12, 0x3f}, "00123")
	checkCodec(t, opentlv.DigitsCodec(0), []byte{}, "")
	checkCodec(t, opentlv.IPv4Codec(), []byte{192, 0, 2, 1}, [4]byte{192, 0, 2, 1})
	checkCodec(t, opentlv.IPv4ListCodec(), []byte{192, 0, 2, 1, 192, 0, 2, 1}, [][4]byte{{192, 0, 2, 1}, {192, 0, 2, 1}})
	checkCodec(t, opentlv.BytesCodec(), []byte{0, 255}, []byte{0, 255})
	input := []byte{1, 2}
	got, err := opentlv.BytesCodec().Decode(input)
	input[0] = 99
	if err != nil || got[0] != 1 {
		t.Fatal("borrowed codec result")
	}
}
func TestCodecFailures(t *testing.T) {
	_, err := opentlv.Uint16BECodec().Decode([]byte{1})
	if !errors.Is(err, opentlv.ErrInvalidValue) {
		t.Fatal(err)
	}
	var detail *opentlv.CodecError
	if !errors.As(err, &detail) || !errors.Is(err, opentlv.ErrInvalidValue) {
		t.Fatal("shared codec status")
	}
	if detail.Detail == nil || detail.Detail.Operation != 0 || detail.Detail.Reported != opentlv.ErrInvalidValue.Code() {
		t.Fatal("missing codec evidence", detail)
	}
	_, err = opentlv.Int64Codec().Decode([]byte{0, 1})
	if !errors.Is(err, opentlv.ErrInvalidValue) {
		t.Fatal(err)
	}
	_, err = opentlv.NumberCodec(opentlv.NumberConfig{Width: 1}).Encode(256)
	if !errors.Is(err, opentlv.ErrInvalidValue) {
		t.Fatal(err)
	}
	_, err = opentlv.TextCodec(opentlv.TextConfig{Alphabet: opentlv.ASCIIAlnum}).Decode([]byte{' '})
	if !errors.Is(err, opentlv.ErrInvalidValue) {
		t.Fatal(err)
	}
	_, err = opentlv.DigitsCodec(0).Decode([]byte{0xa1})
	if !errors.Is(err, opentlv.ErrInvalidValue) {
		t.Fatal(err)
	}
	_, err = opentlv.IPv4ListCodec().Decode([]byte{1})
	if !errors.Is(err, opentlv.ErrInvalidValue) {
		t.Fatal(err)
	}
	var zero opentlv.Codec[uint8]
	if _, err = zero.Decode(nil); !errors.Is(err, opentlv.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err = zero.Encode(0); !errors.Is(err, opentlv.ErrUnsupported) {
		t.Fatal(err)
	}
}
