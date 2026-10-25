// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv

import (
	"bytes"
	"github.com/OpenTLV/opentlv-go/internal/capi"
)

// GeneratorVersion identifies the canonical deterministic wire algorithm.
const GeneratorVersion = capi.GeneratorVersion

// GeneratorCandidate supplies an identifier and inclusive Value size bounds.
// Nil Tag means absent; a nonnil empty Tag is explicitly present and empty.
// Format alone classifies constructed identifiers. Exclude stream terminators
// and identifiers with contextual ordering requirements from this table.
type GeneratorCandidate struct {
	Tag                        []byte
	MinValueSize, MaxValueSize int
}

// GeneratorOptions selects an independently reproducible family of cases.
// Limits are literal: MaxElements, MaxCaseSize and MaxWorkspaceSize must be
// positive; zero depth and Value size are valid. Workspace is (MaxDepth+1)*
// MaxCaseSize and must fit MaxWorkspaceSize. Output separately uses MaxCaseSize.
type GeneratorOptions struct {
	Seed                                                               uint64
	MaxElements, MaxDepth, MaxValueSize, MaxCaseSize, MaxWorkspaceSize int
	Candidates                                                         []GeneratorCandidate
}

// DefaultGeneratorOptions supplies bounded sizes but no candidate domain.
// Fill Candidates before calling NewGenerator. Seed zero is valid.
func DefaultGeneratorOptions() GeneratorOptions {
	return GeneratorOptions{MaxElements: 64, MaxDepth: 8, MaxValueSize: 256, MaxCaseSize: 4096, MaxWorkspaceSize: 1 << 20}
}

// Generator owns an immutable snapshot of candidates/options and retains Format.
// It is safe for concurrent use and requires no Close. Its zero value is invalid.
// Primitive bytes are arbitrary: this generates reconstructible wire streams,
// not Schema-valid or Codec-valid content.
type Generator struct {
	format    Format
	config    *capi.GeneratorConfig
	workspace int
}

// NewGenerator copies candidates and validates limits using Core. ErrLimit means
// the required workspace exceeds the explicit budget. A valid domain may still
// fail Generate with ErrLimit if no nonempty stream fits its framing constraints.
func NewGenerator(format Format, options GeneratorOptions) (Generator, error) {
	if !format.Valid() {
		return Generator{}, ErrInvalidArg
	}
	if !format.CanRead() || !format.CanWrite() {
		return Generator{}, ErrUnsupported
	}
	c := &capi.GeneratorConfig{Seed: options.Seed, MaxElements: options.MaxElements, MaxDepth: options.MaxDepth,
		MaxValueSize: options.MaxValueSize, MaxCaseSize: options.MaxCaseSize, MaxWorkspaceSize: options.MaxWorkspaceSize}
	for _, candidate := range options.Candidates {
		c.Candidates = append(c.Candidates, capi.GeneratorCandidate{Tag: bytes.Clone(candidate.Tag), Min: candidate.MinValueSize, Max: candidate.MaxValueSize})
	}
	size, code := c.WorkspaceSize()
	if code != capi.OK {
		return Generator{}, StatusError{code: code}
	}
	return Generator{format, c, size}, nil
}

// WorkspaceSize reports validated native scratch bytes, excluding output and
// candidate storage; zero is returned for an invalid Generator.
func (g Generator) WorkspaceSize() int { return g.workspace }

// Generate returns independent owned bytes for a random-access case index.
// Same version, Format, ordered candidates, options, seed and index reproduce
// identical bytes regardless of call order. Different cases may coincide.
// Errors publish no partial output and preserve canonical statuses.
func (g Generator) Generate(caseIndex uint64) ([]byte, error) {
	if g.config == nil {
		return nil, ErrInvalidState
	}
	data, code := g.config.Generate(g.format.native, caseIndex)
	if code != capi.OK {
		return nil, StatusError{code: code}
	}
	return data, nil
}
