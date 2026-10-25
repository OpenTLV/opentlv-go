// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

// VisitAction controls traversal after the current item has been consumed.
// Return a Go error to abort; returning VisitStop permits later resumption.
type VisitAction int

const (
	VisitContinue VisitAction = iota
	VisitStop
)

// Visit visits remaining sequential elements, preserving the Reader's source
// metadata and diagnostic snapshots. Callback errors are returned unchanged;
// they do not become terminal Reader errors. No callback is replayed on resume.
// ErrNeedMoreData signals a pause. A nil callback returns ErrInvalidArg.
// Callbacks must not advance or mutate this reader.
func (r *Reader) Visit(visitor func(Element) (VisitAction, error)) error {
	if r.visiting {
		r.reentered = true
		return ErrInvalidState
	}
	if visitor == nil {
		return ErrInvalidArg
	}
	for r.Next() {
		action, err := r.visitElement(visitor)
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

func (r *Reader) visitElement(visitor func(Element) (VisitAction, error)) (VisitAction, error) {
	r.visiting, r.reentered = true, false
	defer func() { r.visiting = false }()
	action, err := visitor(r.Element())
	if r.reentered {
		return action, ErrInvalidState
	}
	return action, err
}
