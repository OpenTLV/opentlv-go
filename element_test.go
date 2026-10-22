// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv

import (
	"bytes"
	"github.com/marekcingel/OpenTLV/bindings/go/internal/capi"
	"testing"
)

func TestNativeSourceAndClone(t *testing.T) {
	f, err := NewFixed(FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: BigEndian})
	if err != nil {
		t.Fatal(err)
	}
	wire := []byte{9, 2, 3, 4}
	native, _, code, _ := f.native.Read(wire, true)
	if code != capi.OK {
		t.Fatal(code)
	}
	e := elementFromNative(native, 17)
	s := e.Source()
	if e.Offset() != 17 || s.Header.Size != 2 || !s.Tag.Present || s.Value.Offset != 2 || !s.Trailer.Present || s.Trailer.Size != 0 || !bytes.Equal(s.Bytes, wire) {
		t.Fatalf("source: %+v", s)
	}
	if &e.Tag()[0] != &wire[0] || &e.Value()[0] != &wire[2] {
		t.Fatal("borrowed views copied")
	}
	clone := e.Clone()
	wire[0], wire[2] = 8, 7
	if clone.Tag()[0] != 9 || clone.Value()[0] != 3 || clone.Source().Bytes[0] != 9 || clone.Offset() != 17 {
		t.Fatal("clone lost ownership or metadata")
	}
	empty := elementFromNative(capi.Element{Source: capi.Source{Tag: capi.Range{Present: true}, FormatTag: true}}, 0)
	if !empty.Source().Tag.Present || !empty.Clone().Source().FormatTag {
		t.Fatal("presence or tag binding lost")
	}
}
