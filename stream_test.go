// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv_test

import (
	"bytes"
	"errors"
	"reflect"
	"runtime"
	"testing"

	tlv "github.com/OpenTLV/opentlv-go"
)

func streamFormat(t *testing.T) tlv.Format {
	t.Helper()
	tag, err := tlv.NewFixedIdentifier(1)
	if err != nil {
		t.Fatal(err)
	}
	length, err := tlv.NewFixedLength(1, tlv.BigEndian)
	if err != nil {
		t.Fatal(err)
	}
	f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: tag, Length: length,
		Constructed: &tlv.ConstructedBit{ByteIndex: 0, Mask: 0x20, Value: 0x20}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func streamWire() []byte { return []byte{0x20, 5, 1, 1, 42, 0x20, 0, 2, 0} }

func TestTreeReaderEventsAndBorrowing(t *testing.T) {
	input := streamWire()
	r, err := tlv.NewTreeReader(input, streamFormat(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []tlv.TreeEvent
	var kinds []tlv.TreeEventKind
	var depths, offsets []int
	for r.NextEvent() {
		runtime.GC() // Cursor must retain the composed native Format owner.
		e := r.Event()
		events = append(events, e)
		kinds = append(kinds, e.Kind)
		depths = append(depths, e.Depth)
		offsets = append(offsets, e.Offset)
		if e.Kind == tlv.TreeEnd && (e.Element.Source().Bytes != nil || e.Element.Tag() != nil || e.Element.Value() != nil) {
			t.Fatal("END retained a payload")
		}
	}
	if r.Err() != nil || !r.AtEnd() || r.Offset() != len(input) || r.Consumed() != len(input) {
		t.Fatal(r.Err(), r.Offset())
	}
	if !reflect.DeepEqual(kinds, []tlv.TreeEventKind{tlv.TreeBegin, tlv.TreeElement, tlv.TreeBegin, tlv.TreeEnd, tlv.TreeEnd, tlv.TreeElement}) ||
		!reflect.DeepEqual(depths, []int{0, 1, 1, 1, 0, 0}) || !reflect.DeepEqual(offsets, []int{0, 2, 5, 7, 7, 7}) {
		t.Fatal(kinds, depths, offsets)
	}
	clone := events[1].Clone()
	input[4] = 99
	if events[1].Element.Value()[0] != 99 || clone.Element.Value()[0] != 42 || clone.Element.Source().Bytes[2] != 42 {
		t.Fatal("borrow/clone")
	}
	if !reflect.DeepEqual(r.Event(), tlv.TreeEvent{}) {
		t.Fatal("stale event after EOF")
	}
}

func TestTreeReaderIncrementalDiscardEverySplit(t *testing.T) {
	f := streamFormat(t)
	wire := streamWire()
	for split := 0; split <= len(wire); split++ {
		r, err := tlv.NewIncrementalTreeReader(bytes.Clone(wire[:split]), f, nil)
		if err != nil {
			t.Fatal(err)
		}
		var events []tlv.TreeEvent
		for r.NextEvent() {
			events = append(events, r.Event().Clone())
		}
		if r.Err() != nil || !r.NeedsMoreData() || r.AtEnd() {
			t.Fatal(split, r.Err())
		}
		if split < 7 && len(events) != 0 {
			t.Fatal("early BEGIN", split)
		}
		discard := r.Consumed()
		if err := r.SetInput(bytes.Clone(wire[discard:]), discard, true); err != nil {
			t.Fatal(split, err)
		}
		for r.NextEvent() {
			events = append(events, r.Event().Clone())
		}
		if r.Err() != nil || !r.AtEnd() || r.Offset() != len(wire) || len(events) != 6 {
			t.Fatal(split, r.Err(), events)
		}
		encoded, err := f.EncodeEvents(events, nil)
		if err != nil || !bytes.Equal(encoded, wire) {
			t.Fatal(split, encoded, err)
		}
	}
}

func TestTreeReaderDiscardInsideParentAndInputFailures(t *testing.T) {
	input := streamWire()
	r, _ := tlv.NewTreeReader(input, streamFormat(t), nil)
	if !r.NextEvent() || r.Consumed() != 2 {
		t.Fatal(r.Err())
	}
	parent := r.Event()
	before := r.Offset()
	if err := r.SetInput(input, 3, true); !errors.Is(err, tlv.ErrInvalidArg) || r.Offset() != before {
		t.Fatal(err)
	}
	if err := r.SetInput(input, 0, false); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if err := r.SetInput(append(bytes.Clone(input), 0), 0, true); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	moved := bytes.Clone(input[2:])
	if err := r.SetInput(moved, 2, true); err != nil {
		t.Fatal(err)
	}
	if !r.NextEvent() || r.Event().Offset != 2 || r.Event().Element.Offset() != 2 {
		t.Fatal(r.Err())
	}
	moved[2] = 88
	if r.Event().Element.Value()[0] != 88 || parent.Element.Value()[2] != 42 {
		t.Fatal("relocated old view")
	}
	for r.NextEvent() {
	}
	if r.Err() != nil || !r.AtEnd() {
		t.Fatal(r.Err())
	}
}

func TestTreeReaderLimitsAndRecovery(t *testing.T) {
	f := streamFormat(t)
	r, err := tlv.NewTreeReader(streamWire(), f, &tlv.TreeReaderOptions{MaxDepth: 3, MaxElements: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !r.NextEvent() || r.NextEvent() || !errors.Is(r.Err(), tlv.ErrBufferTooShort) {
		t.Fatal(r.Err())
	}
	if err := r.GrowFrames(2); err != nil {
		t.Fatal(err)
	}
	if !r.NextEvent() || r.Event().Offset != 2 {
		t.Fatal("failed descent advanced", r.Err())
	}
	if err := r.GrowFrames(1); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}

	r, _ = tlv.NewTreeReader(streamWire(), f, &tlv.TreeReaderOptions{MaxElements: 10})
	if !r.NextEvent() || r.NextEvent() || !errors.Is(r.Err(), tlv.ErrLimit) {
		t.Fatal(r.Err())
	}
	if err := r.SkipSubtree(); err != nil {
		t.Fatal(err)
	}
	if !r.NextEvent() || r.Event().Kind != tlv.TreeEnd || !r.Event().Skipped {
		t.Fatal(r.Event(), r.Err())
	}
	if !r.NextEvent() || r.Event().Offset != 7 || r.NextEvent() || !r.AtEnd() {
		t.Fatal(r.Err())
	}
	if err := r.SkipSubtree(); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}

	r, _ = tlv.NewTreeReader([]byte{0x20, 0}, f, &tlv.TreeReaderOptions{MaxElements: 1})
	if !r.NextEvent() || !r.NextEvent() || r.Event().Kind != tlv.TreeEnd || r.NextEvent() || r.Err() != nil {
		t.Fatal("END charged against budget", r.Err())
	}
	r, _ = tlv.NewTreeReader([]byte{1, 0}, f, &tlv.TreeReaderOptions{})
	if r.Next() || !errors.Is(r.Err(), tlv.ErrLimit) {
		t.Fatal(r.Err())
	}
}

func TestTreeReaderDeepContinuationAndVisitorPanic(t *testing.T) {
	// Two-byte lengths allow a tree larger than 255 bytes without changing
	// the generic constructed predicate or involving a Builtins preset.
	tag, _ := tlv.NewFixedIdentifier(1)
	length, _ := tlv.NewFixedLength(2, tlv.BigEndian)
	f, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: tag, Length: length,
		Constructed: &tlv.ConstructedBit{ByteIndex: 0, Mask: 0x20, Value: 0x20}})
	if err != nil {
		t.Fatal(err)
	}
	var events []tlv.TreeEvent
	for depth := 0; depth < 70; depth++ {
		events = append(events, tlv.TreeEvent{Kind: tlv.TreeBegin, Depth: depth, Element: tlv.NewElement([]byte{0x20}, nil)})
	}
	events = append(events, tlv.TreeEvent{Kind: tlv.TreeElement, Depth: 70, Element: tlv.NewElement([]byte{1}, nil)})
	for depth := 69; depth >= 0; depth-- {
		events = append(events, tlv.TreeEvent{Kind: tlv.TreeEnd, Depth: depth})
	}
	wire, err := f.EncodeEvents(events, &tlv.TreeWriterOptions{MaxDepth: 70, MaxElements: 71})
	if err != nil {
		t.Fatal(err)
	}
	r, err := tlv.NewTreeReader(wire, f, &tlv.TreeReaderOptions{FrameCapacity: 1, MaxDepth: 70, MaxElements: 71})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for r.Next() {
		count++
	}
	if !errors.Is(r.Err(), tlv.ErrBufferTooShort) || count != 2 {
		t.Fatal(count, r.Err())
	}
	if err := r.GrowFrames(70); err != nil {
		t.Fatal(err)
	}
	for r.Next() {
		count++
	}
	if r.Err() != nil || count != 71 || !r.AtEnd() {
		t.Fatal(count, r.Err())
	}

	sequential := tlv.NewReader([]byte{1, 0, 2, 0}, streamFormat(t))
	func() {
		defer func() {
			if recover() != "callback panic" {
				t.Fatal("panic not propagated")
			}
		}()
		_ = sequential.Visit(func(tlv.Element) (tlv.VisitAction, error) { panic("callback panic") })
	}()
	if !sequential.Next() || sequential.Element().Tag()[0] != 2 {
		t.Fatal("panic retained visitor guard")
	}
}

func TestTreeReaderDiagnosticsProjectionAndZeroValues(t *testing.T) {
	f := streamFormat(t)
	input := []byte{1, 0, 0x20, 2, 1, 1}
	r, _ := tlv.NewTreeReader(input, f, nil)
	if !r.Next() || !r.Next() {
		t.Fatal(r.Err())
	}
	if r.Next() || !errors.Is(r.Err(), tlv.ErrTruncated) {
		t.Fatal(r.Err())
	}
	var detail *tlv.ParseError
	if !errors.As(r.Err(), &detail) || !detail.HasOffset || detail.Offset < 4 {
		t.Fatal(r.Err())
	}
	snapshot := bytes.Clone(detail.Tag)
	input[4] = 3
	if !bytes.Equal(detail.Tag, snapshot) {
		t.Fatal("borrowed diagnostic")
	}
	r, _ = tlv.NewTreeReader(streamWire(), f, nil)
	if !r.Next() {
		t.Fatal(r.Err())
	}
	offset := r.Offset()
	if r.NextEvent() || !errors.Is(r.Err(), tlv.ErrInvalidState) || r.Offset() != offset {
		t.Fatal(r.Err())
	}
	if !r.Next() || r.Item().Offset != 2 {
		t.Fatal("projection failure advanced", r.Err())
	}
	var zero tlv.TreeReader
	if zero.Next() || !errors.Is(zero.Err(), tlv.ErrInvalidArg) || zero.AtEnd() {
		t.Fatal(zero.Err())
	}
	if !errors.Is(zero.SetInput(nil, 0, true), tlv.ErrInvalidArg) {
		t.Fatal("zero set input")
	}
	if _, err := tlv.NewTreeReader(nil, tlv.Format{}, nil); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if _, err := tlv.NewTreeReader(nil, f, &tlv.TreeReaderOptions{FrameCapacity: -1}); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}

func TestReaderVisitorResumptionAndReentrancy(t *testing.T) {
	f := streamFormat(t)
	r := tlv.NewIncrementalReader([]byte{1, 0, 2, 0}, f)
	var seen []byte
	stop := func(e tlv.Element) (tlv.VisitAction, error) {
		seen = append(seen, e.Tag()[0])
		return tlv.VisitStop, nil
	}
	if err := r.Visit(stop); err != nil || r.Consumed() != 2 || r.AtEnd() {
		t.Fatal(err)
	}
	sentinel := errors.New("callback failure")
	if err := r.Visit(func(e tlv.Element) (tlv.VisitAction, error) {
		seen = append(seen, e.Tag()[0])
		return tlv.VisitContinue, sentinel
	}); err != sentinel {
		t.Fatal(err)
	}
	if r.Err() != nil || !reflect.DeepEqual(seen, []byte{1, 2}) {
		t.Fatal(r.Err(), seen)
	}
	if err := r.Visit(stop); !errors.Is(err, tlv.ErrNeedMoreData) {
		t.Fatal(err)
	}
	if err := r.SetInput([]byte{3, 0}, 4, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Visit(stop); err != nil || !r.AtEnd() || r.Offset() != 6 {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, []byte{1, 2, 3}) {
		t.Fatal(seen)
	}
	if !errors.Is(r.Visit(nil), tlv.ErrInvalidArg) {
		t.Fatal("nil visitor")
	}

	r = tlv.NewReader([]byte{1, 0, 2, 0}, f)
	if err := r.Visit(func(tlv.Element) (tlv.VisitAction, error) { r.Next(); return tlv.VisitContinue, nil }); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if !r.Next() || r.Element().Tag()[0] != 2 {
		t.Fatal("reentrant advancement")
	}
	r = tlv.NewReader([]byte{1, 0, 2, 0}, f)
	if err := r.Visit(func(tlv.Element) (tlv.VisitAction, error) { return 99, nil }); !errors.Is(err, tlv.ErrVisitor) {
		t.Fatal(err)
	}
	if !r.Next() || r.Element().Tag()[0] != 2 {
		t.Fatal("invalid callback replay")
	}
}

func TestTreeVisitorStopSkipAndCallbackErrors(t *testing.T) {
	r, _ := tlv.NewTreeReader(streamWire(), streamFormat(t), nil)
	if err := r.Visit(func(tlv.TreeItem) (tlv.VisitAction, error) { return tlv.VisitStop, nil }); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("stop here")
	if err := r.Visit(func(i tlv.TreeItem) (tlv.VisitAction, error) {
		if i.Offset != 2 {
			t.Fatal(i)
		}
		return tlv.VisitContinue, sentinel
	}); err != sentinel {
		t.Fatal(err)
	}
	if err := r.Visit(nil); err != nil || !r.AtEnd() {
		t.Fatal(err)
	}
	r, _ = tlv.NewTreeReader(streamWire(), streamFormat(t), nil)
	if err := r.Visit(func(tlv.TreeItem) (tlv.VisitAction, error) { r.SkipSubtree(); return tlv.VisitStop, nil }); !errors.Is(err, tlv.ErrInvalidState) {
		t.Fatal(err)
	}
	if err := r.SkipSubtree(); err != nil {
		t.Fatal(err)
	}
	if !r.Next() || r.Item().Offset != 7 {
		t.Fatal(r.Err())
	}
	r, _ = tlv.NewIncrementalTreeReader(nil, streamFormat(t), nil)
	if err := r.Visit(nil); !errors.Is(err, tlv.ErrNeedMoreData) {
		t.Fatal(err)
	}
}

func streamEvents(t *testing.T, f tlv.Format) []tlv.TreeEvent {
	t.Helper()
	r, err := tlv.NewTreeReader(streamWire(), f, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []tlv.TreeEvent
	for r.NextEvent() {
		events = append(events, r.Event().Clone())
	}
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
	return events
}

func TestTreeWriterEventsCapacityRetryAndOwnership(t *testing.T) {
	f := streamFormat(t)
	events := streamEvents(t, f)
	w, err := tlv.NewTreeWriterBuffer(f, make([]byte, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range events {
		err = w.WriteEvent(e)
		if i == 4 {
			var shortage tlv.CapacityError
			if !errors.As(err, &shortage) || shortage.Required != 7 || w.Size() != 0 {
				t.Fatal(err, w.Size())
			}
			if err = w.SetBuffer(make([]byte, 9)); err != nil {
				t.Fatal(err)
			}
			err = w.WriteEvent(e)
		}
		if err != nil {
			t.Fatal(i, err)
		}
		if e.Kind == tlv.TreeBegin {
			e.Element.Tag()[0] = 0x21
		}
	}
	if err = w.Finish(); err != nil || !bytes.Equal(w.Bytes(), streamWire()) {
		t.Fatal(w.Bytes(), err)
	}
	if err = w.End(); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	raw := tlv.NewWriter(f)
	for _, e := range streamEvents(t, f) {
		if err := raw.WriteEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(raw.Bytes(), streamWire()) {
		t.Fatal(raw.Bytes())
	}
}

func TestTreeWriterMalformedStreamsAndBudgets(t *testing.T) {
	f := streamFormat(t)
	primitive := tlv.NewElement([]byte{1}, nil)
	invalid := []tlv.TreeEvent{
		{Kind: tlv.TreeEnd}, {Kind: 99}, {Kind: tlv.TreeElement, Element: primitive, Depth: 1},
		{Kind: tlv.TreeBegin, Element: primitive, Depth: -1}, {Kind: tlv.TreeElement, Element: primitive, Skipped: true},
	}
	w := tlv.NewWriter(f)
	for _, e := range invalid {
		if err := w.WriteEvent(e); !errors.Is(err, tlv.ErrInvalidValue) || w.Size() != 0 {
			t.Fatal(e, err)
		}
	}
	if err := w.WriteEvent(tlv.TreeEvent{Kind: tlv.TreeElement, Element: tlv.NewElement([]byte{0x20}, nil)}); !errors.Is(err, tlv.ErrInvalidTag) {
		t.Fatal(err)
	}
	if err := w.WriteEvent(tlv.TreeEvent{Kind: tlv.TreeBegin, Element: primitive}); !errors.Is(err, tlv.ErrInvalidTag) {
		t.Fatal(err)
	}
	if data, err := f.EncodeEvents([]tlv.TreeEvent{{Kind: tlv.TreeBegin, Element: tlv.NewElement([]byte{0x20}, nil)}}, nil); !errors.Is(err, tlv.ErrInvalidValue) || data != nil {
		t.Fatal(data, err)
	}
	bounded, _ := tlv.NewTreeWriter(f, &tlv.TreeWriterOptions{MaxDepth: 0, MaxElements: 1})
	if err := bounded.Begin([]byte{0x20}); err != nil {
		t.Fatal(err)
	}
	if err := bounded.Write(primitive); !errors.Is(err, tlv.ErrLimit) {
		t.Fatal(err)
	}
	if err := bounded.End(); err != nil || bounded.Finish() != nil {
		t.Fatal(err)
	}
	if err := bounded.Write(primitive); !errors.Is(err, tlv.ErrLimit) {
		t.Fatal(err)
	}
	zeroBudget, _ := tlv.NewTreeWriter(f, &tlv.TreeWriterOptions{})
	if !errors.Is(zeroBudget.Write(primitive), tlv.ErrLimit) || zeroBudget.Finish() != nil {
		t.Fatal("zero budget")
	}
	var zero tlv.TreeWriter
	if !errors.Is(zero.Begin(nil), tlv.ErrInvalidArg) || !errors.Is(zero.Finish(), tlv.ErrInvalidArg) || zero.Size() != 0 {
		t.Fatal("zero writer")
	}
}

func TestTreeMeasurementAndPreorder(t *testing.T) {
	f := streamFormat(t)
	events := streamEvents(t, f)
	size, err := f.MeasureEvents(events, nil)
	encoded, encodeErr := f.EncodeEvents(events, nil)
	if err != nil || encodeErr != nil || size != len(encoded) || !bytes.Equal(encoded, streamWire()) {
		t.Fatal(size, err, encodeErr)
	}
	r, _ := tlv.NewTreeReader(streamWire(), f, nil)
	var items []tlv.TreeItem
	for r.Next() {
		items = append(items, r.Item().Clone())
	}
	encoded, err = f.EncodeTree(items, nil)
	size, measureErr := f.MeasureTree(items, nil)
	if err != nil || measureErr != nil || size != len(encoded) || !bytes.Equal(encoded, streamWire()) {
		t.Fatal(encoded, err, measureErr)
	}
	bad := []tlv.TreeItem{{Element: tlv.NewElement([]byte{1}, nil)}, {Element: tlv.NewElement([]byte{2}, nil), Depth: 1}}
	if _, err = f.EncodeTree(bad, nil); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	// Empty constructed values must survive the preorder projection.
	encoded, err = f.EncodeTree([]tlv.TreeItem{{Element: tlv.NewElement([]byte{0x20}, []byte{99}), Constructed: true}}, nil)
	if err != nil || !bytes.Equal(encoded, []byte{0x20, 0}) {
		t.Fatal(encoded, err)
	}
}

func TestWriterCopyPreserveAndCapacity(t *testing.T) {
	f := streamFormat(t)
	reader := tlv.NewReader([]byte{1, 1, 42}, f)
	if !reader.Next() {
		t.Fatal(reader.Err())
	}
	e := reader.Element()
	w := tlv.NewWriterBuffer(f, make([]byte, 2))
	if err := w.Preserve(e); !errors.Is(err, tlv.ErrBufferTooShort) || w.Size() != 0 {
		t.Fatal(err)
	}
	if err := w.SetBuffer(make([]byte, 12)); err != nil {
		t.Fatal(err)
	}
	if err := w.Preserve(e); err != nil {
		t.Fatal(err)
	}
	if remaining, fixed := w.Remaining(); !fixed || remaining != 9 {
		t.Fatal(remaining, fixed)
	}
	if err := w.CopyEncoded(w.Bytes()); err != nil || w.Size() != 6 {
		t.Fatal(err)
	}
	if err := w.Begin([]byte{0x20}); err != nil {
		t.Fatal(err)
	}
	if err := w.Preserve(e); err != nil {
		t.Fatal(err)
	}
	if err := w.End(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes(), []byte{1, 1, 42, 1, 1, 42, 0x20, 3, 1, 1, 42}) {
		t.Fatal(w.Bytes())
	}
	before := bytes.Clone(w.Bytes())
	clone := e.Clone()
	clone.Value()[0] = 99
	if err := w.Preserve(clone); err == nil || !bytes.Equal(w.Bytes(), before) {
		t.Fatal(err)
	}
	if !errors.Is(w.Preserve(tlv.NewElement([]byte{1}, nil)), tlv.ErrInvalidArg) {
		t.Fatal("missing source")
	}
	grow := tlv.NewWriter(f)
	if remaining, fixed := grow.Remaining(); remaining != 0 || fixed {
		t.Fatal(remaining, fixed)
	}
	if err := grow.CopyEncoded([]byte{255}); err != nil || !bytes.Equal(grow.Bytes(), []byte{255}) {
		t.Fatal("raw copy must not decode", err)
	}
}

func TestTreeStreamingIndefiniteAndExactSource(t *testing.T) {
	f, err := tlv.Builtin(tlv.BERIndefinite)
	if errors.Is(err, tlv.ErrUnsupported) {
		t.Skip("Builtins disabled")
	}
	if err != nil {
		t.Fatal(err)
	}
	input := []byte{0x30, 0x80, 0x04, 0x81, 0x01, 42, 0, 0}
	r, err := tlv.NewTreeReader(input, f, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []tlv.TreeEvent
	for r.NextEvent() {
		events = append(events, r.Event().Clone())
	}
	if r.Err() != nil || len(events) != 3 {
		t.Fatal(r.Err(), events)
	}
	w := tlv.NewWriter(f)
	if err := w.Preserve(events[0].Element); err != nil || !bytes.Equal(w.Bytes(), input) {
		t.Fatal(err, w.Bytes())
	}
	// BERIndefinite writes constructed elements only. Preserve that native
	// restriction; converting a mixed stream uses the ordinary BER destination.
	if _, err := f.EncodeEvents(events, nil); !errors.Is(err, tlv.ErrInvalidLength) {
		t.Fatal("indefinite primitive accepted", err)
	}
	destination, err := tlv.Builtin(tlv.BER)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := destination.EncodeEvents(events, nil)
	size, measureErr := destination.MeasureEvents(events, nil)
	if err != nil || measureErr != nil || size != len(encoded) {
		t.Fatal(encoded, err, measureErr)
	}
	again, _ := tlv.NewTreeReader(encoded, f, nil)
	n := 0
	for again.Next() {
		n++
	}
	if again.Err() != nil || n != 2 {
		t.Fatal(again.Err(), n)
	}
	parents := []tlv.TreeEvent{
		{Kind: tlv.TreeBegin, Element: tlv.NewElement([]byte{0x30}, nil)},
		{Kind: tlv.TreeBegin, Element: tlv.NewElement([]byte{0x30}, nil), Depth: 1},
		{Kind: tlv.TreeEnd, Depth: 1}, {Kind: tlv.TreeEnd},
	}
	encoded, err = f.EncodeEvents(parents, nil)
	size, measureErr = f.MeasureEvents(parents, nil)
	if err != nil || measureErr != nil || size != len(encoded) || !bytes.Equal(encoded, []byte{0x30, 0x80, 0x30, 0x80, 0, 0, 0, 0}) {
		t.Fatal(encoded, err, measureErr)
	}
}
