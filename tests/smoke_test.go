// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package tests

import (
	"regexp"
	"testing"

	"github.com/marekcingel/OpenTLV/bindings/go"
)

func TestLinkedLibraryVersion(t *testing.T) {
	version := opentlv.Version()
	if !regexp.MustCompile(`^\d+\.\d+\.\d+`).MatchString(version) {
		t.Fatalf("linked library returned an invalid version: %q", version)
	}
	if again := opentlv.Version(); again != version {
		t.Fatalf("library version changed: %q -> %q", version, again)
	}
}
