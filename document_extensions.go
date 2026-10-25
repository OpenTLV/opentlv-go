// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// Find returns the first direct child with tag, or an invalid Node when absent.
// A zero parent searches roots. Foreign and stale parents return an error.
func (d *Document) Find(parent Node, tag []byte) (Node, error) {
	if !d.valid() {
		return Node{}, StatusError{code: capi.InvalidState}
	}
	if parent.owner != nil {
		if !parent.Valid() {
			return Node{}, StatusError{code: capi.InvalidState}
		}
		if parent.owner.documentState != d.documentState {
			return Node{}, StatusError{code: capi.InvalidArg}
		}
	}
	return d.wrap(d.native.Find(parent.native, tag)), nil
}

// Next returns the next sibling, or an invalid Node at the end or when stale.
func (n Node) Next() Node {
	if !n.Valid() {
		return Node{}
	}
	return n.owner.wrap(n.owner.native.Navigate(n.native, 2))
}

// NextSameTag returns the next sibling with the same tag, or an invalid Node.
func (n Node) NextSameTag() Node {
	if !n.Valid() {
		return Node{}
	}
	return n.owner.wrap(n.owner.native.Navigate(n.native, 4))
}

// NodeSourceLocation is an owned snapshot of optional original wire coordinates.
// It does not contain wire bytes or ranges into Document.Source.
type NodeSourceLocation struct{ Offset, HeaderSize OptionalSize }

// SourceLocation returns retained native coordinates. ParseWithOptions must enable
// RetainSourceLocations. Edits clear affected nodes and ancestors; reacquire Nodes
// after every successful edit. Unknown coordinates remain explicitly absent.
func (n Node) SourceLocation() (NodeSourceLocation, error) {
	if !n.Valid() {
		return NodeSourceLocation{}, StatusError{code: capi.InvalidState}
	}
	o, h := n.owner.native.SourceLocation(n.native)
	return NodeSourceLocation{OptionalSize{o.Value, o.Present}, OptionalSize{h.Value, h.Present}}, nil
}

// EncodedSize measures canonical document framing in its original Format.
// It also gives the snapshot capacity accepted by EvaluateDocument.
func (d *Document) EncodedSize() (int, error) {
	if !d.valid() {
		return 0, StatusError{code: capi.InvalidState}
	}
	return d.EncodedSizeAs(d.format)
}

// EncodedSizeAs measures the whole document in a compatible destination Format.
func (d *Document) EncodedSizeAs(format Format) (int, error) {
	if !d.valid() {
		return 0, StatusError{code: capi.InvalidState}
	}
	if !format.Valid() {
		return 0, StatusError{code: capi.InvalidArg}
	}
	size, code := d.native.Encode(format.native, nil, true)
	if code != capi.OK {
		return 0, writeError(code, capi.Diagnostic{})
	}
	return size, nil
}

// EncodedSize measures this node and its descendants in its original Format.
func (n Node) EncodedSize() (int, error) {
	if !n.Valid() {
		return 0, StatusError{code: capi.InvalidState}
	}
	return n.EncodedSizeAs(n.owner.format)
}

// EncodedSizeAs measures one subtree in a compatible destination Format.
func (n Node) EncodedSizeAs(format Format) (int, error) {
	if !n.Valid() {
		return 0, StatusError{code: capi.InvalidState}
	}
	if !format.Valid() {
		return 0, StatusError{code: capi.InvalidArg}
	}
	size, code := n.owner.native.EncodeNode(n.native, format.native, nil, true)
	if code != capi.OK {
		return 0, writeError(code, capi.Diagnostic{})
	}
	return size, nil
}

// Encode returns an owned canonical encoding of this node and its descendants.
func (n Node) Encode() ([]byte, error) {
	if !n.Valid() {
		return nil, StatusError{code: capi.InvalidState}
	}
	return n.EncodeAs(n.owner.format)
}

// EncodeAs encodes one subtree in a compatible Format without remapping tags.
// Failures return WriteError; native subtree encoding supplies status only.
func (n Node) EncodeAs(format Format) ([]byte, error) {
	size, err := n.EncodedSizeAs(format)
	if err != nil {
		return nil, err
	}
	output := make([]byte, size)
	_, code := n.owner.native.EncodeNode(n.native, format.native, output, false)
	if code != capi.OK {
		return nil, writeError(code, capi.Diagnostic{})
	}
	return output, nil
}

// VisitDocument visits remaining finalized Document results. QueryStop consumes
// the delivered node and returns successfully; a subsequent call resumes after it.
// During callbacks, execution reentry and Document edits/Close are rejected.
// Callback errors and panics invalidate execution until Reset. Panics propagate
// after C returns and guards are restored. Use the returned *QueryExecution without
// copying its value. The Document remains unsynchronized.
func (q *QueryExecution) VisitDocument(visitor func(Node) QueryVisit) error {
	if err := q.check(); err != nil {
		return err
	}
	if q.document == nil {
		return programError(capi.InvalidArg, capi.ProgramDiagnostic{})
	}
	if visitor == nil {
		return programError(capi.NullArg, capi.ProgramDiagnostic{})
	}
	q.busy = true
	q.document.queryActive++
	defer func() { q.busy = false; q.document.queryActive-- }()
	code, diag := q.native.VisitDocument(func(node capi.Node) int { return int(visitor(q.document.wrap(node))) })
	return programError(code, diag)
}
