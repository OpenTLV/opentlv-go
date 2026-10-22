// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

// Package capi contains the private cgo bridge to OpenTLV.
// Its boundary uses Go values so C types cannot escape to the public facade.
// Calls borrow Go byte storage synchronously and retain no Go pointers in C.
// Document owns C allocations and persistent C descriptors until Close.
// Read results retain Go slices; native diagnostic bytes and format-owned Tags
// are copied before returning. See bridge.go and the binding README for lifetimes.
package capi

/*
#cgo LDFLAGS: -ltlv
#include <tlv/version.h>
*/
import "C"

// Version copies the linked library's static version string into Go storage.
func Version() string {
	return C.GoString(C.tlv_version_string())
}
