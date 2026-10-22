// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv

import "github.com/marekcingel/OpenTLV/bindings/go/internal/capi"

// Query owns a bounded V1 exact Tag path. Copies are immutable and may be shared
// concurrently; they remain independent of the parsed string and returned Tags.
// Use CompileQuery for the extended language with axes and expressions.
type Query struct{ native *capi.PathQuery }

// ParseQuery delegates bounded V1 parsing to C, preserving syntax error offsets.
// V1 allows at most 65 steps and 512 total Tag bytes.
func ParseQuery(text string) (Query, error) {
	query, code, detail := capi.ParsePath(text)
	if code != capi.OK {
		return Query{}, pathQueryError(code, detail)
	}
	return Query{native: query}, nil
}
func pathQueryError(code capi.Code, detail capi.Diagnostic) error {
	if code == capi.OK {
		return nil
	}
	return &QueryError{Diagnostic: publicDiagnostic(detail), status: StatusError{code: code}}
}

// Count returns the number of Tag steps, or zero for an uninitialized Query.
func (q Query) Count() int { return q.native.Count() }

// Step returns an owned Tag snapshot and false if index is out of bounds.
func (q Query) Step(index int) ([]byte, bool) {
	if index < 0 || index >= q.Count() {
		return nil, false
	}
	return q.native.Step(index), true
}

// Format returns the canonical uppercase path produced by the native formatter.
func (q Query) Format() (string, error) {
	text, code := q.native.Format()
	return text, pathQueryError(code, capi.Diagnostic{})
}

// Matcher starts independent matching over a preorder stream. It owns a stable
// copy of the Query; the original Query can be discarded immediately.
func (q Query) Matcher() (*QueryMatcher, error) {
	matcher, code := q.native.Matcher()
	if code != capi.OK {
		return nil, pathQueryError(code, capi.Diagnostic{})
	}
	return &QueryMatcher{native: matcher}, nil
}

// QueryMatcher owns native V1 continuation storage. Methods serialize access;
// feed every preorder element including nonmatching ancestors and siblings.
// Close releases storage promptly; garbage collection also releases it.
type QueryMatcher struct{ native *capi.PathMatcher }

// Close is idempotent. Subsequent matching operations return ErrInvalidArg.
func (m *QueryMatcher) Close() {
	if m != nil {
		m.native.Close()
	}
}

// Feed returns whether this Tag at depth matches the path. Depth zero is a root.
// Tag bytes are borrowed only for this call.
func (m *QueryMatcher) Feed(tag []byte, depth int) (bool, error) {
	if m == nil {
		return false, pathQueryError(capi.InvalidArg, capi.Diagnostic{})
	}
	matched, code := m.native.Feed(tag, depth)
	return matched, pathQueryError(code, capi.Diagnostic{})
}

// Reset starts another preorder traversal using the same Query.
func (m *QueryMatcher) Reset() error {
	if m == nil {
		return pathQueryError(capi.InvalidArg, capi.Diagnostic{})
	}
	return pathQueryError(m.native.Reset(), capi.Diagnostic{})
}

// Rebind replaces the owned Query with an equivalent copy while preserving
// suspended matching. A different path is rejected without changing state.
func (m *QueryMatcher) Rebind(query Query) error {
	if m == nil {
		return pathQueryError(capi.InvalidArg, capi.Diagnostic{})
	}
	return pathQueryError(m.native.Rebind(query.native), capi.Diagnostic{})
}
