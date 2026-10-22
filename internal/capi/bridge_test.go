// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
)

func fixed(t *testing.T, config FixedConfig) Format {
	t.Helper()
	f, code := NewFixed(config)
	if code != OK {
		t.Fatal(code)
	}
	return f
}

func TestFixedRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config FixedConfig
		wire   []byte
	}{
		{"tlv", FixedConfig{2, 2, 1, 0, 0}, []byte{0x9f, 2, 0, 3, 1, 2, 3}},
		{"ltv", FixedConfig{2, 2, 2, 1, 1}, []byte{5, 0, 0x9f, 2, 1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixed(t, tc.config)
			tag, value := []byte{0x9f, 2}, []byte{1, 2, 3}
			size, code, _ := f.Measure(tag, value)
			if code != OK || size != len(tc.wire) {
				t.Fatalf("measure: %d %v", size, code)
			}
			wire := make([]byte, size)
			n, code, _ := f.Write(wire, tag, value)
			if code != OK || n != size || !bytes.Equal(wire, tc.wire) {
				t.Fatalf("write: %X %d %v", wire, n, code)
			}
			e, n, code, _ := f.Read(wire, true)
			if code != OK || n != size || !bytes.Equal(e.Tag, tag) || !bytes.Equal(e.Value, value) {
				t.Fatalf("read: %+v %d %v", e, n, code)
			}
			if &e.Value[0] != &wire[e.Source.Value.Offset] || &e.Tag[0] != &wire[e.Source.Tag.Offset] || &e.Source.Bytes[0] != &wire[0] {
				t.Fatal("input was copied")
			}
			if e.Source.Header.Size != 4 || e.Source.Value.Size != 3 || e.Source.FormatTag {
				t.Fatalf("source: %+v", e.Source)
			}
			runtime.GC()
			if !bytes.Equal(e.Value, value) {
				t.Fatal("borrowed slice lost owner")
			}
		})
	}
}

func TestIncompleteAndEmptyInput(t *testing.T) {
	f := fixed(t, FixedConfig{1, 1, 1, 0, 0})
	for _, input := range [][]byte{nil, {}, {1}, {1, 2, 3}} {
		for _, final := range []bool{false, true} {
			e, n, code, d := f.Read(input, final)
			want := NeedMoreData
			if final {
				want = Truncated
				if len(input) == 0 {
					want = End
				}
			}
			if code != want || n != 0 || e.Tag != nil || e.Value != nil || d.Code != want {
				t.Fatalf("input %X final=%v: %+v %d %v %+v", input, final, e, n, code, d)
			}
		}
	}
	input := []byte{1, 2, 3}
	_, _, _, d := f.Read(input, true)
	if !d.HasTag || !bytes.Equal(d.Tag, []byte{1}) || !d.HasRawLength || !bytes.Equal(d.RawLength, []byte{2}) || !d.DeclaredLength.Present || d.DeclaredLength.Value != 2 || !d.Offset.Present {
		t.Fatalf("diagnostic: %+v", d)
	}
	input[0], input[1] = 9, 9
	if d.Tag[0] != 1 || d.RawLength[0] != 2 {
		t.Fatal("diagnostic retained borrowed bytes")
	}
}

func TestWriteCapacityAndOverlap(t *testing.T) {
	f := fixed(t, FixedConfig{1, 1, 1, 0, 0})
	for _, output := range [][]byte{nil, {}, {0xaa, 0xaa}} {
		before := bytes.Clone(output)
		n, code, d := f.Write(output, []byte{1}, []byte{2})
		if code != BufferTooShort || n != 3 || !bytes.Equal(output, before) || !d.Required.Present || d.Required.Value != 3 {
			t.Fatalf("capacity: %d %v %+v", n, code, d)
		}
	}
	buffer := []byte{1, 2, 3, 4}
	if _, code, _ := f.Write(buffer, buffer[:1], nil); code != InvalidArg {
		t.Fatal("overlapping tag accepted")
	}
	if _, code, _ := f.Write(buffer, []byte{1}, buffer[2:]); code != InvalidArg {
		t.Fatal("overlapping value accepted")
	}
	if _, code, _ := f.Write(buffer[:2], buffer[2:3], nil); code != OK {
		t.Fatal("adjacent storage rejected:", code)
	}
	if _, code, _ := f.Measure(nil, nil); code != InvalidTagSize {
		t.Fatal("invalid tag:", code)
	}
	for _, value := range [][]byte{nil, {}} {
		output := make([]byte, 2)
		if _, code, _ := f.Write(output, []byte{1}, value); code != OK {
			t.Fatal(code)
		}
		if e, _, code, _ := f.Read(output, true); code != OK || len(e.Value) != 0 {
			t.Fatalf("empty value: %+v %v", e, code)
		}
	}
}

func TestFormatValidation(t *testing.T) {
	for _, config := range []FixedConfig{{0, 1, 1, 0, 0}, {-1, 1, 1, 0, 0}, {1, 9, 1, 0, 0}, {1, 1, 0, 0, 0}, {1, 1, 1, 2, 0}, {1, 1, 1, 0, 2}} {
		if _, code := NewFixed(config); code == OK {
			t.Fatalf("accepted %+v", config)
		}
	}
	for _, kind := range []Kind{Fixed, -1, 999} {
		if _, code := Builtin(kind); code != Unsupported {
			t.Fatal(code)
		}
	}
	if _, _, code, _ := (Format{}).Read(nil, true); code != InvalidArg {
		t.Fatal(code)
	}
	if OK.String() == "" || InvalidLength.String() == "" {
		t.Fatal("missing native status text")
	}
}

func TestBuiltins(t *testing.T) {
	for _, tc := range []struct {
		kind             Kind
		wire, tag, value []byte
	}{
		{BER, []byte{4, 1, 42}, []byte{4}, []byte{42}},
		{BERIndefinite, []byte{0x30, 0x80, 4, 1, 42, 0, 0}, []byte{0x30}, []byte{4, 1, 42}},
		{DER, []byte{4, 1, 42}, []byte{4}, []byte{42}},
		{CER, []byte{4, 1, 42}, []byte{4}, []byte{42}},
		{EMV, []byte{0x9f, 2, 1, 42}, []byte{0x9f, 2}, []byte{42}},
		{LLDP, []byte{6, 2, 0, 120}, []byte{3}, []byte{0, 120}},
		{BluetoothLTV, []byte{2, 1, 42}, []byte{1}, []byte{42}},
		{DHCPv4, []byte{12, 1, 42}, []byte{12}, []byte{42}},
		{NFCType2, []byte{3, 1, 42}, []byte{3}, []byte{42}},
	} {
		f, code := Builtin(tc.kind)
		if code == Unsupported {
			t.Logf("preset %d disabled", tc.kind)
			continue
		}
		if code != OK {
			t.Fatal(code)
		}
		e, n, code, _ := f.Read(tc.wire, true)
		if code != OK || n != len(tc.wire) || !bytes.Equal(e.Tag, tc.tag) || !bytes.Equal(e.Value, tc.value) {
			t.Fatalf("preset %d: %+v %d %v", tc.kind, e, n, code)
		}
		if tc.kind == LLDP && !e.Source.FormatTag {
			t.Fatal("LLDP transformed identifier lost")
		}
		output := make([]byte, len(tc.wire))
		if n, code, _ := f.Write(output, tc.tag, tc.value); code != OK || n != len(output) || !bytes.Equal(output, tc.wire) {
			t.Fatalf("preset %d write: %X %d %v", tc.kind, output, n, code)
		}
		if tc.kind == BERIndefinite && e.Source.Trailer.Size != 2 {
			t.Fatal("EOC trailer lost")
		}
	}
}

func TestConcurrentFormatCalls(t *testing.T) {
	f := fixed(t, FixedConfig{1, 1, 1, 0, 0})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				wire := make([]byte, 3)
				if _, code, _ := f.Write(wire, []byte{1}, []byte{2}); code != OK {
					t.Error(code)
					return
				}
				if e, _, code, _ := f.Read(wire, true); code != OK || e.Value[0] != 2 {
					t.Error(code)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestResultsSurviveLaterCalls(t *testing.T) {
	f := fixed(t, FixedConfig{1, 1, 1, 0, 0})
	first := func() Element {
		e, _, code, _ := f.Read([]byte{1, 1, 42}, true)
		if code != OK {
			t.Fatal(code)
		}
		return e
	}()
	for i := 0; i < 100; i++ {
		if _, _, code, _ := f.Read([]byte{2, 1, 99}, true); code != OK {
			t.Fatal(code)
		}
	}
	runtime.GC()
	if !bytes.Equal(first.Tag, []byte{1}) || !bytes.Equal(first.Value, []byte{42}) {
		t.Fatal("result lifetime depends on temporary native storage")
	}
	if lldp, code := Builtin(LLDP); code == OK {
		e, _, code, _ := lldp.Read([]byte{6, 2, 0, 120}, true)
		if code != OK {
			t.Fatal(code)
		}
		e.Tag[0] = 99
		next, _, code, _ := lldp.Read([]byte{6, 2, 0, 120}, true)
		if code != OK || next.Tag[0] != 3 {
			t.Fatal("format-owned Tag was exposed directly")
		}
	}
}
