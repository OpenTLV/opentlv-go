// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"bytes"
	"errors"
	tlv "github.com/marekcingel/OpenTLV/bindings/go"
	"runtime"
	"strings"
	"testing"
)

func TestV1FailureLocationsIncludeKnownZeroAndEmptyEOF(t *testing.T) {
	for _, test := range []struct {
		text       string
		begin, end uint64
	}{
		{"GG", 0, 1}, {"01/", 3, 3}, {"", 0, 0},
	} {
		_, err := tlv.ParseQuery(test.text)
		var detail *tlv.QueryError
		if !errors.As(err, &detail) {
			t.Fatalf("%q: %v", test.text, err)
		}
		want := tlv.Location{Domain: tlv.LocationExpression, Kind: tlv.LocationSpan, Begin: test.begin, End: test.end}
		if detail.Location != want {
			t.Fatalf("%q: got %+v, want %+v", test.text, detail.Location, want)
		}
	}
}

func TestV1QueryNativeFormattingMatchingAndRebinding(t *testing.T) {
	query, err := tlv.ParseQuery("6f/a5/50")
	if err != nil {
		t.Fatal(err)
	}
	if text, err := query.Format(); err != nil || text != "6F/A5/50" || query.Count() != 3 {
		t.Fatalf("format %q %v", text, err)
	}
	tag, found := query.Step(0)
	if !found || !bytes.Equal(tag, []byte{0x6f}) {
		t.Fatal(tag)
	}
	tag[0] = 0xff
	if copy, _ := query.Step(0); copy[0] != 0x6f {
		t.Fatal("borrowed mutable Tag")
	}
	if _, found := query.Step(-1); found {
		t.Fatal("negative step")
	}
	matcher, err := query.Matcher()
	if err != nil {
		t.Fatal(err)
	}
	defer matcher.Close()
	query = tlv.Query{}
	runtime.GC()
	if matched, err := matcher.Feed([]byte{0x6f}, 0); err != nil || matched {
		t.Fatalf("root %v %v", matched, err)
	}
	equivalent, _ := tlv.ParseQuery("6F/A5/50")
	if err := matcher.Rebind(equivalent); err != nil {
		t.Fatal(err)
	}
	different, _ := tlv.ParseQuery("6F/A5/51")
	if err := matcher.Rebind(different); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if matched, err := matcher.Feed([]byte{0xa5}, 1); err != nil || matched {
		t.Fatalf("ancestor %v %v", matched, err)
	}
	if matched, err := matcher.Feed([]byte{0x50}, 2); err != nil || !matched {
		t.Fatalf("leaf %v %v", matched, err)
	}
	if err := matcher.Reset(); err != nil {
		t.Fatal(err)
	}
	if matched, err := matcher.Feed([]byte{0x50}, 2); err != nil || matched {
		t.Fatalf("reset %v %v", matched, err)
	}
	matcher.Close()
	if _, err := matcher.Feed([]byte{0x50}, 0); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
	if _, err := (tlv.Query{}).Matcher(); !errors.Is(err, tlv.ErrInvalidArg) {
		t.Fatal(err)
	}
}

func TestV1QueryBoundedParseErrors(t *testing.T) {
	for _, text := range []string{"", "50/", "50\x0051", "éé"} {
		_, err := tlv.ParseQuery(text)
		var detail *tlv.QueryError
		if !errors.Is(err, tlv.ErrSyntax) || !errors.As(err, &detail) || !detail.HasOffset {
			t.Fatalf("%q: %v", text, err)
		}
	}
	if _, err := tlv.ParseQuery(strings.Repeat("50/", 65) + "50"); !errors.Is(err, tlv.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := tlv.ParseQuery(strings.Repeat("50", 513)); !errors.Is(err, tlv.ErrLimit) {
		t.Fatal(err)
	}
}
