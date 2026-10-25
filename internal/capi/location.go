// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include "tlv/diagnostic.h"
*/
import "C"

func checkedLocation(domain, kind int, begin, end uint64) C.tlv_location_t {
	var location C.tlv_location_t
	if int(C.int(domain)) != domain || int(C.int(kind)) != kind ||
		uint64(C.size_t(begin)) != begin || uint64(C.size_t(end)) != end {
		return location
	}
	C.tlv_location_set(&location, C.tlv_location_domain_t(domain), C.tlv_location_kind_t(kind), C.size_t(begin), C.size_t(end))
	return location
}

func locationValue(v C.tlv_location_t) Location {
	return Location{Domain: int(v.domain), Kind: int(v.kind), Begin: uint64(v.begin), End: uint64(v.end)}
}

func CheckedLocation(domain, kind int, begin, end uint64) Location {
	return locationValue(checkedLocation(domain, kind, begin, end))
}

func TranslateLocation(domain, kind int, begin, end, origin uint64) Location {
	v := checkedLocation(domain, kind, begin, end)
	if uint64(C.size_t(origin)) != origin {
		return Location{}
	}
	C.tlv_location_translate(&v, C.size_t(origin))
	return locationValue(v)
}

func ResolveLocation(domain, kind int, begin, end, origin uint64, enclosing int) Location {
	v := checkedLocation(domain, kind, begin, end)
	// Preserve Core's no-op for other domains and invalid enclosing domains,
	// even when an unused origin cannot fit the native coordinate type.
	if v.domain != C.TLV_LOCATION_VALUE || v.kind == C.TLV_LOCATION_UNKNOWN ||
		enclosing < int(C.TLV_LOCATION_INPUT) || enclosing > int(C.TLV_LOCATION_VALUE) {
		return locationValue(v)
	}
	if uint64(C.size_t(origin)) != origin {
		return Location{}
	}
	C.tlv_location_resolve(&v, C.size_t(origin), C.tlv_location_domain_t(enclosing))
	return locationValue(v)
}
