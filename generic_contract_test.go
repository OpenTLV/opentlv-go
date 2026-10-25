// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv_test

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"

	tlv "github.com/OpenTLV/opentlv-go"
)

func ExampleDefinitionRegistry_Find() {
	registry, err := tlv.NewDefinitionRegistry([]tlv.Definition{{Tag: []byte{1}, Name: "counter"}})
	if err != nil {
		panic(err)
	}
	definition, found := registry.Find([]byte{1})
	fmt.Println(definition.Name, found)
	// Output: counter true
}

func ExampleLocation_Resolve() {
	local := tlv.NewLocation(tlv.LocationValue, tlv.LocationSpan, 0, 2)
	fmt.Println(local.Resolve(4, tlv.LocationInput))
	// Output: input span 4..6
}

func TestDefinitionRegistrySnapshotLookupAndLifetime(t *testing.T) {
	entries := []tlv.Definition{
		{Tag: []byte{0, 0xff}, Name: "first\x00name"},
		{Tag: []byte{0, 0xff}, Name: "duplicate"},
		{Tag: []byte{}, Name: "empty"},
		{Tag: []byte{1}},
	}
	registry, err := tlv.NewDefinitionRegistry(entries)
	if err != nil {
		t.Fatal(err)
	}
	entries[0].Tag[0], entries[0].Name = 9, "changed"
	copyOfRegistry := registry
	registry = tlv.DefinitionRegistry{}
	runtime.GC()
	for _, tag := range [][]byte{{0, 0xff}, nil, {}, {1}} {
		entry, found := copyOfRegistry.Find(tag)
		if !found || !bytes.Equal(entry.Tag, tag) {
			t.Fatalf("lookup %x: %+v %v", tag, entry, found)
		}
		if len(tag) == 2 {
			if entry.Name != "first\x00name" {
				t.Fatal("order/name lost", entry)
			}
			entry.Tag[0] = 7
		}
	}
	if entry, found := copyOfRegistry.Find([]byte{0, 0xff}); !found || entry.Tag[0] != 0 {
		t.Fatal("result aliases registry")
	}
	for _, tag := range [][]byte{{0}, {0, 0xff, 0}, {8}} {
		if _, found := copyOfRegistry.Find(tag); found {
			t.Fatalf("unexpected match %x", tag)
		}
	}
	empty, err := tlv.NewDefinitionRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []tlv.DefinitionRegistry{registry, empty} {
		if _, found := r.Find(nil); found {
			t.Fatal("empty registry matched")
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				entry, found := copyOfRegistry.Find([]byte{0, 0xff})
				if !found || entry.Name != "first\x00name" {
					t.Error("concurrent lookup")
				}
				entry.Tag[0] = 3
			}
		}()
	}
	wg.Wait()
}

func TestLocationCheckedCoordinatesAndResolution(t *testing.T) {
	local := tlv.NewLocation(tlv.LocationValue, tlv.LocationSpan, 0, 3)
	resolved := local.Resolve(11, tlv.LocationInput)
	if resolved != (tlv.Location{Domain: tlv.LocationInput, Kind: tlv.LocationSpan, Begin: 11, End: 14}) || local.Begin != 0 {
		t.Fatal(local, resolved)
	}
	if got := resolved.Translate(2); got.Begin != 13 || got.End != 16 || got.Domain != tlv.LocationInput {
		t.Fatal(got)
	}
	if got := resolved.Resolve(^uint64(0), tlv.LocationOutput); got != resolved {
		t.Fatal("non-value resolution changed", got)
	}
	for _, domain := range []tlv.LocationDomain{tlv.LocationDomainUnknown, -1, 99, tlv.LocationDomain(int(^uint(0) >> 1))} {
		if got := local.Resolve(^uint64(0), domain); got != local {
			t.Fatal("invalid enclosing domain changed", got)
		}
		if got := tlv.NewLocation(domain, tlv.LocationPoint, 0, 0); got != (tlv.Location{}) {
			t.Fatal("invalid domain narrowed", got)
		}
	}
	for _, v := range []tlv.Location{
		{}, {Domain: tlv.LocationInput, Kind: 99},
		{Domain: tlv.LocationInput, Kind: tlv.LocationSpan, Begin: 3, End: 2},
		{Domain: tlv.LocationInput, Kind: tlv.LocationPoint, Begin: 1, End: 2},
		{Domain: tlv.LocationInput, Kind: tlv.LocationKind(int(^uint(0) >> 1))},
	} {
		if got := v.Translate(0); got != (tlv.Location{}) {
			t.Fatal("invalid location", got)
		}
	}
	for _, kind := range []tlv.LocationKind{tlv.LocationPoint, tlv.LocationSpan, tlv.LocationScopeEnd, tlv.LocationInsertion} {
		if got := tlv.NewLocation(tlv.LocationInput, kind, 0, 0); got.Kind != kind || got.Begin != 0 {
			t.Fatal("known zero lost", got)
		}
	}
	if got := local.Resolve(^uint64(0), tlv.LocationInput); got != (tlv.Location{}) {
		t.Fatal("resolve overflow", got)
	}
	if got := local.Translate(^uint64(0)); got != (tlv.Location{}) {
		t.Fatal("translate overflow", got)
	}
	// Exercise the public uint64/native-size boundary on both 32- and 64-bit hosts.
	maxNative := uint64(^uint(0))
	last := tlv.NewLocation(tlv.LocationOutput, tlv.LocationPoint, maxNative, maxNative)
	if last.Kind != tlv.LocationPoint || last.Translate(1) != (tlv.Location{}) {
		t.Fatal("native limit", last)
	}
	if maxNative < ^uint64(0) && tlv.NewLocation(tlv.LocationInput, tlv.LocationPoint, maxNative+1, maxNative+1) != (tlv.Location{}) {
		t.Fatal("native narrowing")
	}
}

func TestGenericCopyAlternativesOwnershipAndFailure(t *testing.T) {
	f, err := tlv.NewFixed(tlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: tlv.BigEndian})
	if err != nil {
		t.Fatal(err)
	}
	input := []byte{1, 2, 3, 4}
	reader := tlv.NewReader(input, f)
	if !reader.Next() {
		t.Fatal(reader.Err())
	}
	owned := reader.Element().Clone()
	input[0], input[2] = 9, 8
	writer := tlv.NewWriter(f)
	if err := writer.Write(owned); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(writer.Bytes(), []byte{1, 2, 3, 4}) {
		t.Fatal(writer.Bytes())
	}
	// Exact copying is independent of framing validation and handles aliasing.
	if err := writer.CopyEncoded(writer.Bytes()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(writer.Bytes(), []byte{1, 2, 3, 4, 1, 2, 3, 4}) {
		t.Fatal(writer.Bytes())
	}
	if err := writer.CopyEncoded([]byte{0xff}); err != nil {
		t.Fatal(err)
	}
	fixed := tlv.NewWriterBuffer(f, make([]byte, 0, 1))
	if err := fixed.Write(owned); !errors.Is(err, tlv.ErrBufferTooShort) || len(fixed.Bytes()) != 0 {
		t.Fatal("short serialization", err)
	}
	if err := fixed.CopyEncoded(owned.Source().Bytes); !errors.Is(err, tlv.ErrBufferTooShort) || len(fixed.Bytes()) != 0 {
		t.Fatal("short exact copy", err)
	}
}
