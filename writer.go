// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import (
	"bytes"
	"github.com/marekcingel/OpenTLV/bindings/go/internal/capi"
)

type writerFrame struct{ tag, value []byte }

// CapacityError reports the total required root buffer size before any write.
// Unwrap preserves the native BUFFER_TOO_SHORT status as a StatusError.
type CapacityError struct {
	// Required is the total root buffer size needed to complete the operation.
	Required int
	// Available is the current root buffer capacity.
	Available int
}

// Error returns the canonical native capacity error description.
func (e CapacityError) Error() string { return capi.BufferTooShort.String() }

// Unwrap returns the native status for errors.As.
func (e CapacityError) Unwrap() error { return StatusError{code: capi.BufferTooShort} }

// Writer stages and encodes elements using the C engine and an immutable Format.
// Open parents use Go-owned staging storage, rather than streaming output.
// It owns no native resources, needs no Close, and is not safe for concurrent use.
// Its zero value is invalid. Errors leave the logical output and open frames unchanged.
type Writer struct {
	format Format
	output []byte
	fixed  bool
	frames []writerFrame
}

// NewWriter creates a Writer with automatically allocated output.
func NewWriter(format Format) *Writer { return &Writer{format: format} }

// NewWriterBuffer borrows the entire buffer as output capacity, starting empty.
// It never grows this buffer. Open parents still use allocated staging storage.
// Keep the buffer unchanged while the Writer is in use.
func NewWriterBuffer(format Format, buffer []byte) *Writer {
	return &Writer{format: format, output: buffer[:0:len(buffer)], fixed: true}
}

// Measure returns the exact encoded size of one element through Format,
// including content-dependent framing. It does not change Writer state.
// Native failures return *WriteError with an owned diagnostic snapshot.
func (w *Writer) Measure(tag, value []byte) (int, error) {
	if !w.format.Valid() {
		return 0, StatusError{code: capi.InvalidArg}
	}
	n, code, diag := w.format.native.Measure(tag, value)
	if code != capi.OK {
		return 0, writeError(code, diag)
	}
	return n, nil
}

// appendElement encodes into independent storage before publishing it.
func (w *Writer) appendElement(tag, value []byte, closing bool) error {
	n, err := w.Measure(tag, value)
	if err != nil {
		return err
	}
	target := w.output
	depth := len(w.frames)
	if closing {
		depth--
	}
	if depth > 0 {
		target = w.frames[depth-1].value
	}
	if n > int(^uint(0)>>1)-len(target) {
		return StatusError{code: capi.Overflow}
	}
	if depth == 0 && w.fixed && n > cap(w.output)-len(w.output) {
		return CapacityError{Required: len(w.output) + n, Available: cap(w.output)}
	}
	encoded := make([]byte, n)
	var code capi.Code
	var diag capi.Diagnostic
	if closing {
		_, code, diag = w.format.native.End(encoded, tag, value)
	} else {
		_, code, diag = w.format.native.Write(encoded, tag, value)
	}
	if code != capi.OK {
		return writeError(code, diag)
	}
	target = append(target, encoded...)
	if depth > 0 {
		w.frames[depth-1].value = target
	} else {
		w.output = target
	}
	return nil
}

// WriteElement appends a complete logical element, re-encoding it in this Format.
// Input is borrowed only during the call and may alias previously returned Bytes.
// Native failures return *WriteError; fixed-buffer shortages return CapacityError.
func (w *Writer) WriteElement(tag, value []byte) error {
	return w.appendElement(tag, value, false)
}

// Write appends a public Element's logical content; Source framing is not preserved.
func (w *Writer) Write(element Element) error { return w.WriteElement(element.Tag(), element.Value()) }

// Begin opens a constructed parent recognized by Format. Primitive-only
// Formats reject Begin. The Tag is copied; further Format validation may be
// deferred until End, as in the C Tree Writer. Nested parents are supported.
// Native failures return *WriteError with an owned diagnostic snapshot.
func (w *Writer) Begin(tag []byte) error {
	if !w.format.Valid() {
		return StatusError{code: capi.InvalidArg}
	}
	if code, diag := w.format.native.Begin(tag); code != capi.OK {
		return writeError(code, diag)
	}
	w.frames = append(w.frames, writerFrame{tag: bytes.Clone(tag)})
	return nil
}

// Value appends uninterpreted bytes to the current open parent's Value.
// Bytes are copied; a parent must be open. This performs no wire interpretation.
func (w *Writer) Value(value []byte) error {
	if !w.format.Valid() || len(w.frames) == 0 {
		return StatusError{code: capi.InvalidArg}
	}
	f := &w.frames[len(w.frames)-1]
	if len(value) > int(^uint(0)>>1)-len(f.value) {
		return StatusError{code: capi.Overflow}
	}
	f.value = append(f.value, value...)
	return nil
}

// End encodes and publishes the innermost staged parent through C Tree Writer.
// Failure keeps the parent open, allowing a capacity error to be retried.
// Native failures return *WriteError; fixed-buffer shortages return CapacityError.
func (w *Writer) End() error {
	if len(w.frames) == 0 {
		return StatusError{code: capi.InvalidArg}
	}
	f := w.frames[len(w.frames)-1]
	if err := w.appendElement(f.tag, f.value, true); err != nil {
		return err
	}
	w.frames[len(w.frames)-1] = writerFrame{}
	w.frames = w.frames[:len(w.frames)-1]
	return nil
}

// Finish verifies that all parents are closed. It does not implicitly close them.
func (w *Writer) Finish() error {
	if !w.format.Valid() || len(w.frames) != 0 {
		return StatusError{code: capi.InvalidArg}
	}
	return nil
}

// Bytes returns the completed root prefix, excluding all open parents.
// It borrows output storage; copy it before modifying or reusing that storage.
func (w *Writer) Bytes() []byte { return w.output[:len(w.output):len(w.output)] }

// Size returns the number of completed root bytes.
func (w *Writer) Size() int { return len(w.output) }

// SetBuffer copies the completed prefix into buffer and switches to fixed
// caller storage. It preserves open parents, permitting End to be retried.
// Failure leaves Writer unchanged; buffer's length determines capacity.
func (w *Writer) SetBuffer(buffer []byte) error {
	if !w.format.Valid() {
		return StatusError{code: capi.InvalidArg}
	}
	if len(buffer) < len(w.output) {
		return CapacityError{Required: len(w.output), Available: len(buffer)}
	}
	copy(buffer, w.output)
	w.output, w.fixed = buffer[:len(w.output):len(buffer)], true
	return nil
}
