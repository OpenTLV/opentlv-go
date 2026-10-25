// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	tlv "github.com/OpenTLV/opentlv-go"
	"runtime"
	"sync"
	"testing"
)

func generatorOptions() tlv.GeneratorOptions {
	return tlv.GeneratorOptions{Seed: 42, MaxElements: 8, MaxDepth: 3, MaxValueSize: 16, MaxCaseSize: 128, MaxWorkspaceSize: 512,
		Candidates: []tlv.GeneratorCandidate{{Tag: []byte{1}, MaxValueSize: 16}, {Tag: []byte{0x80}, MaxValueSize: 16}}}
}

func TestGeneratorPinnedNativeSeedIndexMapping(t *testing.T) {
	// Direct tlv_generate output from Core cb3caed5b09706ce11ecfae839397fd4f99d0025,
	// algorithm v2. These three adapter fixtures check full-width seed/index
	// forwarding; algorithm/property behavior remains tested in Core.
	f, err := tlv.NewFixed(tlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: tlv.BigEndian})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		seed, index uint64
		wire        string
	}{
		{0, 0, "0120cca1d1ac8495358a71c5489ab31c1175c8a29577874d5c6b88e054500d0ed862"},
		{42, 7, "012048ecf8c41ccdc2235100ae955fe8a75ab630a3f440192ef03d91ebca7fb841fc"},
		{^uint64(0), ^uint64(0), "0120d138ef3603d87475fcb4015db47b800f8054eb183722de513089a18caf981b4d"},
	} {
		g, err := tlv.NewGenerator(f, tlv.GeneratorOptions{Seed: test.seed, MaxElements: 1, MaxValueSize: 32, MaxCaseSize: 64, MaxWorkspaceSize: 64,
			Candidates: []tlv.GeneratorCandidate{{Tag: []byte{1}, MinValueSize: 32, MaxValueSize: 32}}})
		if err != nil {
			t.Fatal(err)
		}
		wire, err := g.Generate(test.index)
		if err != nil || hex.EncodeToString(wire) != test.wire {
			t.Fatalf("seed=%d index=%d: %x %v", test.seed, test.index, wire, err)
		}
	}
}

func TestGeneratorSnapshotRandomAccessAndOwnedOutput(t *testing.T) {
	f := structureFormat(t)
	opts := generatorOptions()
	g, err := tlv.NewGenerator(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if g.WorkspaceSize() != 512 || tlv.GeneratorVersion != 2 {
		t.Fatal(g.WorkspaceSize(), tlv.GeneratorVersion)
	}
	baseline, err := g.Generate(7)
	if err != nil {
		t.Fatal(err)
	}
	opts.Candidates[0].Tag[0] = 9
	opts.Candidates[1].MaxValueSize = 0
	opts.Seed = 99
	runtime.GC()
	for _, index := range []uint64{0, 3, ^uint64(0), 7} {
		wire, err := g.Generate(index)
		if err != nil || len(wire) == 0 || len(wire) > 128 {
			t.Fatal(wire, err)
		}
		r, err := tlv.NewTreeReader(wire, f, &tlv.TreeReaderOptions{FrameCapacity: 4, MaxDepth: 3, MaxElements: 8})
		if err != nil {
			t.Fatal(err)
		}
		var events []tlv.TreeEvent
		for r.NextEvent() {
			e := r.Event()
			if e.Kind != tlv.TreeEnd && len(e.Element.Value()) > 16 {
				t.Fatal("Value bound lost")
			}
			events = append(events, e)
		}
		if r.Err() != nil {
			t.Fatal(r.Err())
		}
		rebuilt, err := f.EncodeEvents(events, nil)
		if err != nil || !bytes.Equal(wire, rebuilt) {
			t.Fatal(rebuilt, err)
		}
		if index == 7 && !bytes.Equal(wire, baseline) {
			t.Fatal("candidate snapshot or random access lost")
		}
		wire[0] ^= 1
	}
	repeated, err := g.Generate(7)
	if err != nil || !bytes.Equal(repeated, baseline) {
		t.Fatal("output aliases another result", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wire, err := g.Generate(7)
			if err != nil || !bytes.Equal(wire, baseline) {
				t.Error(wire, err)
			}
		}()
	}
	wg.Wait()
}

func TestGeneratorBoundsAndFailurePublication(t *testing.T) {
	f := structureFormat(t)
	for _, test := range []struct {
		change func(*tlv.GeneratorOptions)
		want   error
	}{
		{func(o *tlv.GeneratorOptions) { o.MaxElements = -1 }, tlv.ErrInvalidArg},
		{func(o *tlv.GeneratorOptions) { o.MaxElements = 0 }, tlv.ErrInvalidArg},
		{func(o *tlv.GeneratorOptions) { o.MaxDepth = 65 }, tlv.ErrUnsupported},
		{func(o *tlv.GeneratorOptions) { o.MaxWorkspaceSize = 511 }, tlv.ErrLimit},
		{func(o *tlv.GeneratorOptions) { o.MaxWorkspaceSize = 0 }, tlv.ErrInvalidArg},
		{func(o *tlv.GeneratorOptions) { o.Candidates = nil }, tlv.ErrInvalidArg},
		{func(o *tlv.GeneratorOptions) { o.Candidates[0].MinValueSize = -1 }, tlv.ErrInvalidArg},
		{func(o *tlv.GeneratorOptions) { o.Candidates[0].MinValueSize = 17 }, tlv.ErrInvalidArg},
		{func(o *tlv.GeneratorOptions) { o.MaxCaseSize = int(^uint(0) >> 1); o.MaxWorkspaceSize = o.MaxCaseSize }, tlv.ErrOverflow},
	} {
		opts := generatorOptions()
		test.change(&opts)
		if _, err := tlv.NewGenerator(f, opts); !errors.Is(err, test.want) {
			t.Fatalf("want %v: %v", test.want, err)
		}
	}
	opts := generatorOptions()
	opts.MaxCaseSize = 1
	opts.MaxDepth = 0
	opts.MaxValueSize = 0
	for i := range opts.Candidates {
		opts.Candidates[i].MaxValueSize = 0
	}
	g, err := tlv.NewGenerator(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := g.Generate(0); data != nil || !errors.Is(err, tlv.ErrLimit) {
		t.Fatal(data, err)
	}
	var zero tlv.Generator
	if _, err := zero.Generate(0); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if _, err := tlv.NewGenerator(tlv.Format{}, generatorOptions()); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	// Absence and present-empty are legal candidate descriptors, but neither can
	// form a stream in this one-byte identifier Format. Generation reports that.
	for _, tag := range [][]byte{nil, {}} {
		opts = tlv.GeneratorOptions{MaxElements: 1, MaxCaseSize: 2, MaxWorkspaceSize: 2, Candidates: []tlv.GeneratorCandidate{{Tag: tag}}}
		g, err = tlv.NewGenerator(f, opts)
		if err != nil {
			t.Fatal(err)
		}
		if data, err := g.Generate(0); data != nil || !errors.Is(err, tlv.ErrLimit) {
			t.Fatal(data, err)
		}
	}
}
