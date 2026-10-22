// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// QueryError preserves the native Query status, the optional byte offset in the
// query text and the common diagnostic of the C Document traversal.
type QueryError struct {
	Diagnostic
	status StatusError
}

// Error returns the native failure description.
func (e *QueryError) Error() string { return e.status.Error() }

// Unwrap supports errors.Is and errors.As for native statuses.
func (e *QueryError) Unwrap() error { return e.status }

// Query returns all matching nodes in document order. Paths contain exact
// hexadecimal tags separated by '/', such as "6F/A5/50". Recursive searches,
// leading slashes, wildcards and predicates are unsupported. No matches returns
// an empty slice. Close or a successful edit invalidates returned nodes.
func (d *Document) Query(path string) ([]Node, error) {
	if !d.valid() {
		return nil, &QueryError{Diagnostic: Diagnostic{Message: capi.InvalidArg.String()}, status: StatusError{code: capi.InvalidArg}}
	}
	matches, code, detail := d.native.Query(path)
	if code != capi.OK {
		return nil, &QueryError{Diagnostic: publicDiagnostic(detail), status: StatusError{code: code}}
	}
	result := make([]Node, len(matches))
	for i, n := range matches {
		result[i] = d.wrap(n)
	}
	return result, nil
}
