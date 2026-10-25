// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv

import (
	"github.com/OpenTLV/opentlv-go/internal/capi"
	"slices"
)

// ValueConstraintKind selects a constraint on a decoded signed scalar.
type ValueConstraintKind int

const (
	ValueConstraintNone ValueConstraintKind = iota
	ValueConstraintRange
	ValueConstraintAllowedValues
)

// ValueConstraintConfig defines inclusive bounds or an allowed set. Names is
// optional; when supplied it must parallel AllowedValues (empty names are valid).
type ValueConstraintConfig struct {
	Kind               ValueConstraintKind
	MinValue, MaxValue int64
	AllowedValues      []int64
	Names              []string
}

// ValueConstraint is an immutable owned snapshot, safe for concurrent use.
// Its zero value permits every scalar. It does not decode or inspect wire bytes.
type ValueConstraint struct{ config capi.ValueConstraintConfig }

// NewValueConstraint copies and checks a definition through Core.
// Names containing NUL or a nonparallel names table return ErrInvalidArg.
func NewValueConstraint(c ValueConstraintConfig) (ValueConstraint, error) {
	n := capi.ValueConstraintConfig{Kind: int(c.Kind), Min: c.MinValue, Max: c.MaxValue,
		Values: slices.Clone(c.AllowedValues), Names: slices.Clone(c.Names)}
	if code, _, _ := n.Call(0, 0); code != capi.OK {
		return ValueConstraint{}, StatusError{code: code}
	}
	return ValueConstraint{n}, nil
}

// Validate returns ErrSchema for a value outside the allowed range or set.
func (c ValueConstraint) Validate(value int64) error {
	code, _, _ := c.config.Call(1, value)
	if code != capi.OK {
		return StatusError{code: code}
	}
	return nil
}

// Name returns an owned name and whether Core found a named matching entry.
func (c ValueConstraint) Name(value int64) (string, bool, error) {
	code, name, found := c.config.Call(2, value)
	if code != capi.OK {
		return "", false, StatusError{code: code}
	}
	return name, found, nil
}
