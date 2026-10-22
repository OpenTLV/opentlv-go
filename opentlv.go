// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

// Package opentlv provides the Go facade over the canonical OpenTLV C engine.
//
// This experimental binding exposes Format, Element, Reader, Writer, Document
// and library version information. Applications use Go types; C types and cgo
// implementation details are confined to internal/capi.
//
// Building requires cgo, a compatible C compiler, and a prebuilt OpenTLV
// library with its matching source and generated headers. See README.md for
// build configuration. There is no pure-Go processing fallback.
package opentlv

import "github.com/marekcingel/OpenTLV/bindings/go/internal/capi"

// Version returns the version string of the linked OpenTLV C library.
// The returned string is owned by Go and remains valid independently of C storage.
func Version() string {
	return capi.Version()
}
