// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import (
	"bytes"

	"github.com/OpenTLV/opentlv-go/internal/capi"
)

// Definition names a canonical identifier in an application-selected registry.
// Name is descriptive, not unique; an empty name means unnamed. It does not
// select a Format or Codec, impose a Schema, or validate a wire identifier.
type Definition struct {
	Tag  []byte
	Name string
}

// DefinitionRegistry is an immutable snapshot, safe for concurrent lookups.
// Copies share native ownership; no Close is needed. The zero value is empty.
type DefinitionRegistry struct {
	entries []Definition
	native  capi.DefinitionRegistry
}

// NewDefinitionRegistry copies entries and tag bytes. Entry order and duplicate
// tags are preserved; Find returns the first match. Empty tags are allowed.
// Callers must not mutate entries concurrently with construction.
func NewDefinitionRegistry(entries []Definition) (DefinitionRegistry, error) {
	result := DefinitionRegistry{entries: make([]Definition, len(entries))}
	tags := make([][]byte, len(entries))
	for i, entry := range entries {
		result.entries[i] = Definition{Tag: bytes.Clone(entry.Tag), Name: entry.Name}
		tags[i] = result.entries[i].Tag
	}
	var code capi.Code
	result.native, code = capi.NewDefinitionRegistry(tags)
	if code != capi.OK {
		return DefinitionRegistry{}, StatusError{code: code}
	}
	return result, nil
}

// Find uses Core's exact tag-byte lookup and returns an owned entry and true.
// Unknown tags return a zero Definition and false; they are not invalid tags.
// Nil and empty lookup tags match an empty entry. The result may be mutated
// independently of the registry, its copies and subsequent lookup results.
func (r DefinitionRegistry) Find(tag []byte) (Definition, bool) {
	index := r.native.Find(tag)
	if index < 0 {
		return Definition{}, false
	}
	entry := r.entries[index]
	entry.Tag = bytes.Clone(entry.Tag)
	return entry, true
}
