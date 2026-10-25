// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// Remaining returns unused fixed root capacity and true, or zero and false for
// growable output. Staged open parents are not subtracted from root capacity.
func (w *Writer) Remaining() (int, bool) {
	if !w.fixed {
		return 0, false
	}
	return cap(w.output) - len(w.output), true
}

// CopyEncoded copies bytes verbatim into the current sequence. It does not
// validate encoding or compatibility with Format. Input may alias Bytes.
// Failure preserves output/frames; fixed-root shortages return CapacityError.
func (w *Writer) CopyEncoded(encoded []byte) error {
	return w.appendSource(encoded, Element{}, false)
}

// Preserve appends an element's original encoding after Core validates it with
// this writer's Format and checks its tag/value identity. Use the originating
// Format; logical elements without source return ErrInvalidArg. Source bytes
// must remain unchanged since decode. Inputs are copied and may alias Bytes.
func (w *Writer) Preserve(element Element) error {
	if len(element.Source().Bytes) == 0 {
		return ErrInvalidArg
	}
	return w.appendSource(element.Source().Bytes, element, true)
}

func (w *Writer) appendSource(source []byte, element Element, preserve bool) error {
	if !w.format.Valid() {
		return ErrInvalidArg
	}
	target := w.output
	if len(w.frames) > 0 {
		target = w.frames[len(w.frames)-1].value
	}
	if len(source) > int(^uint(0)>>1)-len(target) {
		return ErrOverflow
	}
	encoded := make([]byte, len(source))
	code, diag := w.format.native.AppendEncoded(encoded, source, element.Tag(), element.Value(), preserve)
	if code != capi.OK {
		return writeError(code, diag)
	}
	if len(w.frames) == 0 && w.fixed && len(encoded) > cap(w.output)-len(w.output) {
		return CapacityError{Required: len(w.output) + len(encoded), Available: cap(w.output)}
	}
	target = append(target, encoded...)
	if len(w.frames) > 0 {
		w.frames[len(w.frames)-1].value = target
	} else {
		w.output = target
	}
	return nil
}
