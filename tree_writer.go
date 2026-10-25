// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

// WriteEvent applies one balanced structural operation. BEGIN copies its tag
// and ignores its complete source Value; ELEMENT writes a primitive; END closes
// the innermost parent. Source and Offset are informational and not encoded.
// Skipped events, invalid depths/order and unknown kinds return ErrInvalidValue.
// Errors preserve all output and frames. On a capacity error, retain this event,
// enlarge the buffer with SetBuffer, then retry before advancing its producer.
func (w *Writer) WriteEvent(event TreeEvent) error {
	if !w.format.Valid() {
		return ErrInvalidArg
	}
	depth := len(w.frames)
	if event.Skipped || event.Depth < 0 {
		return ErrInvalidValue
	}
	switch event.Kind {
	case TreeBegin:
		if event.Depth != depth {
			return ErrInvalidValue
		}
		return w.Begin(event.Element.Tag())
	case TreeElement:
		if event.Depth != depth {
			return ErrInvalidValue
		}
		if w.format.IsConstructed(event.Element.Tag()) {
			return ErrInvalidTag
		}
		return w.Write(event.Element)
	case TreeEnd:
		if depth == 0 || event.Depth != depth-1 {
			return ErrInvalidValue
		}
		return w.End()
	default:
		return ErrInvalidValue
	}
}

// TreeWriterOptions bounds node depth and successful BEGIN/ELEMENT operations.
// Zero values permit roots only and no items. Nil selects depth 64 and unlimited
// items. Complete constructed elements count once; END never consumes a budget.
type TreeWriterOptions struct{ MaxDepth, MaxElements int }

// TreeWriter adds explicit traversal budgets to Writer's Go-owned staging.
// Open tags and values are copied. Only closed roots appear in Bytes; complete
// parents are staged before encoding, including indefinite formats. Frame/tag/
// scratch storage grows in Go memory; a fixed output buffer bounds closed roots,
// not intermediate staging. It needs no Close. Do not copy or use concurrently.
// The zero value reports ErrInvalidArg.
type TreeWriter struct {
	writer *Writer
	limits TreeWriterOptions
	count  int
}

// NewTreeWriter creates a bounded traversal writer with growable output.
func NewTreeWriter(format Format, options *TreeWriterOptions) (*TreeWriter, error) {
	return newTreeWriter(NewWriter(format), options)
}

// NewTreeWriterBuffer borrows buffer as fixed root output capacity.
func NewTreeWriterBuffer(format Format, buffer []byte, options *TreeWriterOptions) (*TreeWriter, error) {
	return newTreeWriter(NewWriterBuffer(format, buffer), options)
}

func newTreeWriter(w *Writer, options *TreeWriterOptions) (*TreeWriter, error) {
	o := TreeWriterOptions{64, int(^uint(0) >> 1)}
	if options != nil {
		o = *options
	}
	if !w.format.CanWrite() || o.MaxDepth < 0 || o.MaxElements < 0 {
		return nil, ErrInvalidArg
	}
	return &TreeWriter{writer: w, limits: o}, nil
}

func (w *TreeWriter) check() error {
	if w.writer == nil {
		return ErrInvalidArg
	}
	if len(w.writer.frames) > w.limits.MaxDepth || w.count >= w.limits.MaxElements {
		return ErrLimit
	}
	return nil
}

// Begin opens a constructed parent, copying the tag and counting one node.
func (w *TreeWriter) Begin(tag []byte) error {
	if err := w.check(); err != nil {
		return err
	}
	if err := w.writer.Begin(tag); err != nil {
		return err
	}
	w.count++
	return nil
}

// Write appends a primitive or a complete constructed element, counting once.
func (w *TreeWriter) Write(element Element) error {
	if err := w.check(); err != nil {
		return err
	}
	if err := w.writer.Write(element); err != nil {
		return err
	}
	w.count++
	return nil
}

// WriteEvent applies a balanced event under this writer's traversal budgets.
func (w *TreeWriter) WriteEvent(event TreeEvent) error {
	if w.writer == nil {
		return ErrInvalidArg
	}
	if event.Kind != TreeEnd {
		if err := w.check(); err != nil {
			return err
		}
	}
	if err := w.writer.WriteEvent(event); err != nil {
		return err
	}
	if event.Kind != TreeEnd {
		w.count++
	}
	return nil
}

// End closes the current parent. Failure keeps its staged contents for retry.
func (w *TreeWriter) End() error {
	if w.writer == nil {
		return ErrInvalidArg
	}
	return w.writer.End()
}

// Finish checks balance without closing parents or sealing the writer.
func (w *TreeWriter) Finish() error {
	if w.writer == nil {
		return ErrInvalidArg
	}
	return w.writer.Finish()
}

// SetBuffer copies the closed root prefix and preserves open parents for retry.
func (w *TreeWriter) SetBuffer(buffer []byte) error {
	if w.writer == nil {
		return ErrInvalidArg
	}
	return w.writer.SetBuffer(buffer)
}

// Bytes borrows the completed root prefix; open parents are excluded.
func (w *TreeWriter) Bytes() []byte {
	if w.writer == nil {
		return nil
	}
	return w.writer.Bytes()
}

// Size returns the number of completed root bytes.
func (w *TreeWriter) Size() int {
	if w.writer == nil {
		return 0
	}
	return w.writer.Size()
}

// EncodeEvents stages a complete balanced stream, returning independent encoded
// bytes. An incomplete stream is ErrInvalidValue; errors return no partial output.
func (f Format) EncodeEvents(events []TreeEvent, options *TreeWriterOptions) ([]byte, error) {
	w, err := NewTreeWriter(f, options)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if err := w.WriteEvent(event); err != nil {
			return nil, err
		}
	}
	if len(w.writer.frames) != 0 {
		return nil, ErrInvalidValue
	}
	return w.Bytes(), nil
}

// MeasureEvents returns the exact encoded size by staging through EncodeEvents.
// Content-dependent formats require encoding children, so this allocates the
// complete output. Use EncodeEvents once when both bytes and size are needed.
func (f Format) MeasureEvents(events []TreeEvent, options *TreeWriterOptions) (int, error) {
	data, err := f.EncodeEvents(events, options)
	return len(data), err
}

// EncodeTree stages preorder nodes. Depth may increase by one only after a
// constructed node; source Values on constructed nodes are ignored. Classification
// must agree with the destination Format. Empty parents close automatically.
func (f Format) EncodeTree(items []TreeItem, options *TreeWriterOptions) ([]byte, error) {
	w, err := NewTreeWriter(f, options)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Depth < 0 || item.Depth > len(w.writer.frames) {
			return nil, ErrInvalidArg
		}
		for len(w.writer.frames) > item.Depth {
			if err := w.End(); err != nil {
				return nil, err
			}
		}
		kind := TreeElement
		if item.Constructed {
			kind = TreeBegin
		}
		if err := w.WriteEvent(TreeEvent{Kind: kind, Element: item.Element, Depth: item.Depth}); err != nil {
			return nil, err
		}
	}
	for len(w.writer.frames) > 0 {
		if err := w.End(); err != nil {
			return nil, err
		}
	}
	return w.Bytes(), nil
}

// MeasureTree returns the exact size by staging through EncodeTree.
func (f Format) MeasureTree(items []TreeItem, options *TreeWriterOptions) (int, error) {
	data, err := f.EncodeTree(items, options)
	return len(data), err
}
