// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import (
	"bytes"
	"github.com/marekcingel/OpenTLV/bindings/go/internal/capi"
)

// Document owns a mutable native TLV tree. Call Close when finished.
// Copies of the pointer share ownership. It is not safe for concurrent use.
// Its zero value is closed. Successful edits invalidate all previously returned
// Nodes, including unaffected siblings; reacquire them through Elements.
type Document struct{ *documentState }
type documentState struct {
	native      *capi.Document
	format      Format
	source      []byte
	generation  uint64
	queryActive int
}

// DocumentOptions bounds native parsing and subsequent edits. Zero limits are
// literal limits; Parse uses the C defaults instead.
type DocumentOptions struct {
	MaxDepth, MaxElements int
	// RetainSourceLocations preserves original offsets/header lengths for Query.
	// Edits invalidate affected nodes and ancestors; input bytes are not borrowed.
	RetainSourceLocations bool
}

// Parse copies a complete input into an owned Document using C default limits.
// Native failures return *ParseError; disabled support matches ErrUnsupported.
func Parse(data []byte, format Format) (*Document, error) {
	return parseDocument(data, format, DocumentOptions{}, true)
}

// ParseWithOptions parses using explicit depth and total element limits.
// Native failures return *ParseError with owned diagnostics and optional offsets.
func ParseWithOptions(data []byte, format Format, options DocumentOptions) (*Document, error) {
	return parseDocument(data, format, options, false)
}

func parseDocument(data []byte, format Format, options DocumentOptions, defaults bool) (*Document, error) {
	if !format.Valid() {
		return nil, StatusError{code: capi.InvalidArg}
	}
	native, code, diag := format.native.ParseDocument(data, options.MaxDepth, options.MaxElements, defaults, options.RetainSourceLocations)
	if code != capi.OK {
		return nil, parseError(code, diag, 0)
	}
	return &Document{&documentState{native: native, format: format, source: bytes.Clone(data)}}, nil
}

// Close deterministically releases native storage. It is idempotent and nil-safe;
// node content snapshots already returned remain valid.
func (d *Document) Close() error {
	if d.valid() && d.queryActive > 0 {
		return StatusError{code: capi.InvalidState}
	}
	if d.valid() {
		d.native.Close()
		d.native = nil
		d.source = nil
		d.generation++
	}
	return nil
}
func (d *Document) valid() bool { return d != nil && d.documentState != nil && d.native != nil }

// Source returns an independent copy of the original complete encoding.
// It returns nil after any successful edit or Close. Nodes have no wire ranges:
// the C Document stores logical content, and Encode regenerates framing.
func (d *Document) Source() []byte {
	if !d.valid() {
		return nil
	}
	return bytes.Clone(d.source)
}

// Count returns the total number of nodes, including descendants; zero if closed.
func (d *Document) Count() int {
	if !d.valid() {
		return 0
	}
	return d.native.Count()
}

// Node is a checked reference into a Document. It keeps its owner alive, but
// Close or any successful edit invalidates it. The zero value is invalid.
type Node struct {
	owner      *Document
	native     capi.Node
	generation uint64
}

// Valid reports whether this node can still be accessed.
func (n Node) Valid() bool {
	return n.owner.valid() && n.generation == n.owner.generation && n.native.Valid()
}

// Identity returns the canonical C node identity while this checked handle is valid.
func (n Node) Identity() (uint64, error) {
	if !n.Valid() {
		return 0, StatusError{code: capi.InvalidState}
	}
	return n.native.Identity(), nil
}
func (d *Document) wrap(n capi.Node) Node { return Node{d, n, d.generation} }
func (d *Document) siblings(first capi.Node) []Node {
	var nodes []Node
	for n := first; n.Valid(); n = d.native.Navigate(n, 2) {
		nodes = append(nodes, d.wrap(n))
	}
	return nodes
}

// Elements returns root nodes in encoding order, or nil for a closed document.
func (d *Document) Elements() []Node {
	if !d.valid() {
		return nil
	}
	return d.siblings(d.native.Navigate(capi.Node{}, 0))
}

// Element returns the root at a zero-based index; invalid indexes return an error.
func (d *Document) Element(index int) (Node, error) {
	nodes := d.Elements()
	if index < 0 || index >= len(nodes) {
		return Node{}, StatusError{code: capi.InvalidArg}
	}
	return nodes[index], nil
}

// Children returns direct children in encoding order, or nil for invalid nodes.
func (n Node) Children() []Node {
	if !n.Valid() {
		return nil
	}
	return n.owner.siblings(n.owner.native.Navigate(n.native, 1))
}

// Parent returns the enclosing node, or an invalid Node for roots or stale nodes.
func (n Node) Parent() Node {
	if !n.Valid() {
		return Node{}
	}
	return n.owner.wrap(n.owner.native.Navigate(n.native, 3))
}

// Element returns an independent content snapshot. Constructed nodes have an
// empty Value; inspect Children instead. Source ranges are unavailable.
func (n Node) Element() (Element, error) {
	if !n.Valid() {
		return Element{}, StatusError{code: capi.InvalidState}
	}
	e, _ := n.owner.native.Read(n.native)
	return elementFromNative(e, 0), nil
}

// Tag returns an owned identifier snapshot, or nil for an invalid node.
func (n Node) Tag() []byte { e, _ := n.Element(); return e.Tag() }

// Value returns owned primitive bytes, or nil for constructed or invalid nodes.
func (n Node) Value() []byte { e, _ := n.Element(); return e.Value() }

// Constructed reports the classification stored by C at node creation.
func (n Node) Constructed() bool {
	if !n.Valid() {
		return false
	}
	_, constructed := n.owner.native.Read(n.native)
	return constructed
}
func (d *Document) changed() { d.generation++; d.source = nil }

// SetValue replaces primitive bytes or parses replacement children for a
// constructed node. Failure leaves the tree and all handles unchanged.
func (n Node) SetValue(value []byte) error {
	if !n.Valid() || n.owner.queryActive > 0 {
		return StatusError{code: capi.InvalidState}
	}
	_, code := n.owner.native.Edit(n.native, capi.Node{}, nil, value, 0)
	if code != capi.OK {
		return StatusError{code: code}
	}
	n.owner.changed()
	return nil
}

// Erase removes this node and descendants and invalidates all node handles.
func (n Node) Erase() error {
	if !n.Valid() || n.owner.queryActive > 0 {
		return StatusError{code: capi.InvalidState}
	}
	_, code := n.owner.native.Edit(n.native, capi.Node{}, nil, nil, 1)
	if code != capi.OK {
		return StatusError{code: code}
	}
	n.owner.changed()
	return nil
}

// Insert copies an element before a direct child, or appends when before is
// zero. A zero parent selects roots. Foreign or stale nodes are rejected.
// Success invalidates existing handles and returns a fresh handle to the new node.
func (d *Document) Insert(parent, before Node, element Element) (Node, error) {
	if !d.valid() || d.queryActive > 0 {
		return Node{}, StatusError{code: capi.InvalidState}
	}
	for _, n := range []Node{parent, before} {
		if n.owner != nil || n.native.Valid() {
			if !n.Valid() {
				return Node{}, StatusError{code: capi.InvalidState}
			}
			if n.owner.documentState != d.documentState {
				return Node{}, StatusError{code: capi.InvalidArg}
			}
		}
	}
	n, code := d.native.Edit(parent.native, before.native, element.Tag(), element.Value(), 2)
	if code != capi.OK {
		return Node{}, StatusError{code: code}
	}
	d.changed()
	return d.wrap(n), nil
}

// Encode regenerates the whole document through C Tree Writer in its Format.
func (d *Document) Encode() ([]byte, error) {
	if !d.valid() {
		return nil, StatusError{code: capi.InvalidState}
	}
	return d.EncodeAs(d.format)
}

// EncodeAs regenerates framing in a compatible destination Format; tags are
// never remapped, and incompatible constructed topology is rejected by C.
// Native failures return *WriteError. The C Document encoding API supplies only
// a status, so optional diagnostic detail is absent.
func (d *Document) EncodeAs(format Format) ([]byte, error) {
	if !d.valid() || !format.Valid() {
		return nil, StatusError{code: capi.InvalidArg}
	}
	size, code := d.native.Encode(format.native, nil, true)
	if code != capi.OK {
		return nil, writeError(code, capi.Diagnostic{})
	}
	output := make([]byte, size)
	_, code = d.native.Encode(format.native, output, false)
	if code != capi.OK {
		return nil, writeError(code, capi.Diagnostic{})
	}
	return output, nil
}

// WriteDocument appends a complete Document encoded in this Writer's Format.
// It works inside staged parents. Errors leave Writer state unchanged.
func (w *Writer) WriteDocument(d *Document) error {
	encoded, err := d.EncodeAs(w.format)
	if err != nil {
		return err
	}
	if len(w.frames) > 0 {
		return w.Value(encoded)
	}
	if len(encoded) > int(^uint(0)>>1)-len(w.output) {
		return StatusError{code: capi.Overflow}
	}
	if w.fixed && len(encoded) > cap(w.output)-len(w.output) {
		return CapacityError{Required: len(w.output) + len(encoded), Available: cap(w.output)}
	}
	w.output = append(w.output, encoded...)
	return nil
}
