// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import (
	"errors"
	"github.com/OpenTLV/opentlv-go/internal/capi"
)

// TreeEventKind identifies one operation in a balanced structural stream.
type TreeEventKind int

const (
	TreeBegin   TreeEventKind = TreeEventKind(capi.TreeBegin)
	TreeElement TreeEventKind = TreeEventKind(capi.TreeElement)
	TreeEnd     TreeEventKind = TreeEventKind(capi.TreeEnd)
)

// TreeEvent carries a complete borrowed Element on BEGIN/ELEMENT. END carries
// no payload; its Offset is the end of the parent's value. Depth is root-relative.
// Skipped marks an END whose descendants were deliberately omitted.
type TreeEvent struct {
	Kind          TreeEventKind
	Element       Element
	Depth, Offset int
	Skipped       bool
}

// Clone detaches an event's byte storage from the input.
func (e TreeEvent) Clone() TreeEvent { e.Element = e.Element.Clone(); return e }

// TreeItem is the node-only preorder projection, hiding END events.
type TreeItem struct {
	Element       Element
	Depth, Offset int
	Constructed   bool
}

// Clone detaches an item's byte storage from the input.
func (i TreeItem) Clone() TreeItem { i.Element = i.Element.Clone(); return i }

// TreeReaderOptions independently bounds structural storage, item depth and
// publication count. Zero values are literal: no frames, roots only, no items.
// A nil options pointer selects 64 frames, depth 64 and an unlimited item count.
type TreeReaderOptions struct {
	FrameCapacity, MaxDepth, MaxElements int
}

// TreeReader pulls nodes or balanced events through Core without building a
// Document. Input and returned views are borrowed, and must stay unchanged while
// used. BEGIN waits for the complete encoded parent, including its descendants.
// It owns no native resources and needs no Close. Do not copy a TreeReader or use
// it concurrently. Its zero value is invalid. Select either Next or NextEvent
// for its lifetime; mixing projections reports ErrInvalidState without advancing.
type TreeReader struct {
	native              *capi.TreeCursor
	item                TreeItem
	event               TreeEvent
	err                 error
	waiting             bool
	mode                int
	visiting, reentered bool
}

// NewTreeReader borrows a complete, final input window and copies options.
func NewTreeReader(input []byte, format Format, options *TreeReaderOptions) (*TreeReader, error) {
	return newTreeReader(input, format, options, true)
}

// NewIncrementalTreeReader borrows a non-final contiguous input window.
func NewIncrementalTreeReader(input []byte, format Format, options *TreeReaderOptions) (*TreeReader, error) {
	return newTreeReader(input, format, options, false)
}

func newTreeReader(input []byte, format Format, options *TreeReaderOptions, final bool) (*TreeReader, error) {
	maxInt := int(^uint(0) >> 1)
	o := TreeReaderOptions{64, 64, maxInt}
	if options != nil {
		o = *options
	}
	if !format.CanRead() || o.FrameCapacity < 0 || o.MaxDepth < 0 || o.MaxElements < 0 {
		return nil, ErrInvalidArg
	}
	if o.FrameCapacity > maxInt/16 {
		return nil, ErrOverflow
	}
	native, code := capi.NewTreeCursor(format.native, input, final, o.FrameCapacity, o.MaxDepth, o.MaxElements)
	if code != capi.OK {
		return nil, StatusError{code: code}
	}
	return &TreeReader{native: native}, nil
}

func (r *TreeReader) guard() error {
	if r.visiting {
		r.reentered = true
		return ErrInvalidState
	}
	if r.native == nil {
		return ErrInvalidArg
	}
	return nil
}

// Next publishes the next preorder item. False clears Item; inspect Err and
// NeedsMoreData. Resource errors preserve the pending subtree for recovery.
func (r *TreeReader) Next() bool { return r.pull(false) }

// NextEvent publishes BEGIN, ELEMENT or END. Known END events are emitted before
// a pause or final EOF; they do not consume the configured item budget.
func (r *TreeReader) NextEvent() bool { return r.pull(true) }

func (r *TreeReader) pull(events bool) bool {
	if r.visiting {
		r.reentered = true
		return false
	}
	r.item, r.event = TreeItem{}, TreeEvent{}
	r.waiting = false
	if err := r.guard(); err != nil {
		r.err = err
		return false
	}
	mode := 1
	if events {
		mode = 2
	}
	if r.mode != 0 && r.mode != mode {
		r.err = ErrInvalidState
		return false
	}
	r.mode = mode
	// Core preserves the cursor after failures, so a caller may retry the same
	// operation or skip a pending subtree. No native wire error is hidden.
	e, code, diag := r.native.Next(events)
	r.err, r.waiting = nil, false
	switch code {
	case capi.OK:
		element := Element{}
		if e.Kind != capi.TreeEnd {
			element = elementFromNative(e.Element, e.Offset)
		}
		r.event = TreeEvent{Kind: TreeEventKind(e.Kind), Element: element, Depth: e.Depth, Offset: e.Offset, Skipped: e.Skipped}
		r.item = TreeItem{Element: element, Depth: e.Depth, Offset: e.Offset, Constructed: e.Kind == capi.TreeBegin}
		return true
	case capi.End:
	case capi.NeedMoreData:
		r.waiting = true
	default:
		r.err = parseError(code, diag, 0)
	}
	return false
}

// Item returns the current node from Next, or zero after a failed pull.
func (r *TreeReader) Item() TreeItem { return r.item }

// Event returns the current event from NextEvent, or zero after a failed pull.
func (r *TreeReader) Event() TreeEvent { return r.event }

// Err returns the last pull error. EOF and input pauses are not errors.
// Unlike sequential Reader, TreeReader permits retry or subtree skipping after
// native failures; errors preserve the Core continuation.
func (r *TreeReader) Err() error { return r.err }

// NeedsMoreData reports whether the last pull paused for non-final input.
func (r *TreeReader) NeedsMoreData() bool { return r.waiting }

// Consumed returns the current-window prefix no longer needed for traversal.
// Retained views may still borrow these bytes; clone them before reusing storage.
func (r *TreeReader) Consumed() int {
	if r.native == nil {
		return 0
	}
	return r.native.Consumed()
}

// Offset returns the absolute traversal frontier, including pending descent.
func (r *TreeReader) Offset() int {
	if r.native == nil {
		return 0
	}
	return r.native.Offset()
}

// AtEnd reports final input exhaustion and closure of every structural scope.
func (r *TreeReader) AtEnd() bool { return r.native != nil && r.native.AtEnd() }

// SetInput replaces the contiguous window, preserving the old suffix following
// discard byte-for-byte, then optionally appending input. Prefix identity is a
// caller precondition. discard cannot exceed Consumed. Final input cannot be
// reopened or extended. Failure leaves the cursor and its last result unchanged.
func (r *TreeReader) SetInput(input []byte, discard int, final bool) error {
	if err := r.guard(); err != nil {
		return err
	}
	if discard < 0 || discard > r.Consumed() {
		return ErrInvalidArg
	}
	maxInt := int(^uint(0) >> 1)
	if len(input) > maxInt-r.native.Base()-discard {
		return ErrOverflow
	}
	if code := r.native.SetInput(input, discard, final); code != capi.OK {
		return StatusError{code: code}
	}
	r.err, r.waiting = nil, false
	return nil
}

// SkipSubtree skips pending nonempty descendants without decoding them. It can
// recover from a failed descent (depth/frame limits). Event mode then emits a
// skipped END, which cannot be passed to Writer.WriteEvent as complete content.
func (r *TreeReader) SkipSubtree() error {
	if err := r.guard(); err != nil {
		return err
	}
	if code := r.native.Skip(); code != capi.OK {
		return StatusError{code: code}
	}
	r.err, r.waiting = nil, false
	return nil
}

// GrowFrames increases structural capacity while preserving the continuation.
// It does not relax MaxDepth or MaxElements. Retry the failed pull afterwards.
func (r *TreeReader) GrowFrames(capacity int) error {
	if err := r.guard(); err != nil {
		return err
	}
	if capacity < r.native.Capacity() {
		return ErrInvalidArg
	}
	if capacity > int(^uint(0)>>1)/16 {
		return ErrOverflow
	}
	r.native.Grow(capacity)
	if errors.Is(r.err, ErrBufferTooShort) {
		r.err = nil
	}
	return nil
}

// Visit visits remaining preorder items. A nil callback validates only.
// STOP and callback errors leave the item consumed and its descendants pending;
// another Visit resumes without replay. ErrNeedMoreData signals a resumable pause.
// Callbacks must not advance or mutate this reader; such attempts return ErrInvalidState.
func (r *TreeReader) Visit(visitor func(TreeItem) (VisitAction, error)) error {
	if err := r.guard(); err != nil {
		return err
	}
	for r.Next() {
		if visitor == nil {
			continue
		}
		action, err := r.visitItem(visitor)
		if err != nil {
			return err
		}
		if action == VisitStop {
			return nil
		}
		if action != VisitContinue {
			return ErrVisitor
		}
	}
	if r.Err() != nil {
		return r.Err()
	}
	if r.NeedsMoreData() {
		return ErrNeedMoreData
	}
	return nil
}

func (r *TreeReader) visitItem(visitor func(TreeItem) (VisitAction, error)) (VisitAction, error) {
	r.visiting, r.reentered = true, false
	defer func() { r.visiting = false }()
	action, err := visitor(r.Item())
	if r.reentered {
		return action, ErrInvalidState
	}
	return action, err
}
