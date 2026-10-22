// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"errors"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"testing"
)

func TestPublicFormats(t *testing.T) {
	for _, order := range []opentlv.ElementOrder{opentlv.TLV, opentlv.LTV} {
		f, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 2, LengthSize: 2, ByteOrder: opentlv.LittleEndian, Order: order, LengthScope: opentlv.TagAndValueLength})
		if err != nil || !f.Valid() {
			t.Fatalf("fixed: %v", err)
		}
	}
	f, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: -1})
	var status opentlv.StatusError
	if f.Valid() || !errors.As(err, &status) || status.Code() == 0 || status.Error() == "" {
		t.Fatal("invalid config accepted")
	}
	for _, kind := range []opentlv.FormatKind{opentlv.BER, opentlv.BERIndefinite, opentlv.DER, opentlv.CER, opentlv.EMV, opentlv.LLDP, opentlv.BluetoothLTV, opentlv.DHCPv4, opentlv.NFCType2, -1, 999} {
		f, err := opentlv.Builtin(kind)
		if f.Valid() != (err == nil) {
			t.Fatalf("kind %d: validity mismatch", kind)
		}
		if (kind == -1 || kind == 999) && err == nil {
			t.Fatal("unknown preset accepted")
		}
	}
	if (opentlv.Format{}).Valid() {
		t.Fatal("zero format valid")
	}
}
func TestPublicElementOwnership(t *testing.T) {
	tag, value := []byte{0, 1}, []byte{2}
	e := opentlv.NewElement(tag, value)
	owned := e.Clone()
	tag[0], value[0] = 9, 8
	if e.Tag()[0] != 9 || e.Value()[0] != 8 || owned.Tag()[0] != 0 || owned.Value()[0] != 2 {
		t.Fatal("ownership contract")
	}
	if e.Source().Bytes != nil || e.Offset() != 0 {
		t.Fatal("fabricated source")
	}
	if opentlv.NewElement(nil, nil).Clone().Tag() != nil || opentlv.NewElement([]byte{}, []byte{}).Clone().Tag() == nil {
		t.Fatal("nil/empty distinction lost")
	}
}
