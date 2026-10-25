// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/OpenTLV/opentlv-go/internal/capi"

// NewLocation checks coordinates through Core. Invalid domains, kinds, bounds
// or values outside native size_t produce an unknown location. Non-span anchors
// require begin == end. Invalid optional evidence never changes an error code.
func NewLocation(domain LocationDomain, kind LocationKind, begin, end uint64) Location {
	return publicLocation(capi.CheckedLocation(int(domain), int(kind), begin, end))
}

// Translate returns checked coordinates offset by origin in the same domain.
// Overflow or invalid input produces an unknown location. The receiver is unchanged.
func (v Location) Translate(origin uint64) Location {
	return publicLocation(capi.TranslateLocation(int(v.Domain), int(v.Kind), v.Begin, v.End, origin))
}

// Resolve places Value-local evidence at origin in a valid enclosing domain.
// Other locations and unknown/invalid enclosing domains retain their checked
// coordinates. Overflow produces unknown evidence. The receiver is unchanged.
func (v Location) Resolve(origin uint64, domain LocationDomain) Location {
	return publicLocation(capi.ResolveLocation(int(v.Domain), int(v.Kind), v.Begin, v.End, origin, int(domain)))
}

func publicLocation(v capi.Location) Location {
	return Location{Domain: LocationDomain(v.Domain), Kind: LocationKind(v.Kind), Begin: v.Begin, End: v.End}
}
