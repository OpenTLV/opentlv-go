// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/marekcingel/OpenTLV/bindings/go/internal/capi"

// Reader reads sequential elements through the canonical C Reader. Input and
// returned views are borrowed; keep their bytes unchanged while in use. Reader
// owns no native resources and needs no Close. It is not safe for concurrent use.
// Its zero value reports an invalid-argument error on Next.
type Reader struct {
	input                []byte
	format               Format
	pos, base            int
	final, waiting, done bool
	element              Element
	err                  error
}

// NewReader borrows a complete input buffer. Invalid Formats are reported by Err.
func NewReader(input []byte, format Format) *Reader {
	r := NewIncrementalReader(input, format)
	r.final = true
	return r
}

// NewIncrementalReader borrows a non-final contiguous input window. Next pauses
// with NeedsMoreData when incomplete input needs extension through SetInput.
func NewIncrementalReader(input []byte, format Format) *Reader {
	r := &Reader{input: input, format: format}
	if !format.Valid() {
		r.err = StatusError{code: capi.InvalidArg}
	}
	return r
}

// Next publishes the next complete element. False means end, error, or a pause
// for more input; inspect Err and NeedsMoreData. It clears the current Element
// on false and never advances the cursor on error or incomplete input.
func (r *Reader) Next() bool {
	r.element = Element{}
	if !r.format.Valid() && r.err == nil {
		r.err = StatusError{code: capi.InvalidArg}
	}
	if r.err != nil || r.done {
		return false
	}
	e, n, code, diag := r.format.native.Read(r.input[r.pos:], r.final)
	r.waiting = false
	switch code {
	case capi.OK:
		r.element = elementFromNative(e, r.base+r.pos)
		r.pos += n
		return true
	case capi.End:
		r.done = true
	case capi.NeedMoreData:
		r.waiting = true
	default:
		r.err = parseError(code, diag, uint64(r.base+r.pos))
	}
	return false
}

// Element returns the current borrowed view, or a zero Element after Next
// returns false. Retained views remain valid while their original bytes survive
// unchanged; use Clone before reusing input storage.
func (r *Reader) Element() Element { return r.element }

// Err returns the terminal parse or construction error. Clean EOF and a pause
// for more input are not errors. Native failures return *ParseError with owned
// diagnostic detail; errors.Is matches named errors and errors.As finds StatusError.
func (r *Reader) Err() error { return r.err }

// NeedsMoreData reports whether the last Next paused for non-final input.
func (r *Reader) NeedsMoreData() bool { return r.waiting }

// Offset returns the absolute cursor position, including discarded bytes.
func (r *Reader) Offset() int { return r.base + r.pos }

// SetInput replaces the contiguous window, discarding at most the consumed
// prefix. Input must start with the old bytes from discard through the old end,
// followed by optional new bytes; prefix identity is a caller precondition.
// final declares EOF. Final input cannot be reopened or extended, but consumed
// bytes may still be discarded. Retained views borrow their original storage.
// Failure leaves Reader unchanged; terminal parse errors cannot be resumed.
func (r *Reader) SetInput(input []byte, discard int, final bool) error {
	if !r.format.Valid() || r.err != nil || discard < 0 || discard > r.pos || len(input) < len(r.input)-discard || (r.final && (!final || len(input) != len(r.input)-discard)) {
		return StatusError{code: capi.InvalidArg}
	}
	maxInt := int(^uint(0) >> 1)
	if discard > maxInt-r.base || len(input) > maxInt-(r.base+discard) {
		return StatusError{code: capi.Overflow}
	}
	r.input, r.base, r.pos = input, r.base+discard, r.pos-discard
	r.final, r.waiting = final, false
	return nil
}
