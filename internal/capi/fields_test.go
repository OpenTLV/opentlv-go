// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package capi

import (
	"runtime"
	"testing"
)

// Release the Go owner's reference deterministically to isolate C retention from
// the public facade's additional Go references. The finalizer does this in use.
func releaseFormatForTest(f *Format) {
	runtime.SetFinalizer(f.owner, nil)
	f.owner.free()
	*f = Format{}
}

func TestNativeConsumersRetainComposedContext(t *testing.T) {
	tag, code := FixedIdentifier(1)
	if code != OK {
		t.Fatal(code)
	}
	length, code := FixedLength(1, 1)
	if code != OK {
		t.Fatal(code)
	}
	f, code := Composed(Composition{Identifier: tag, Length: length}, ComposedFormat, PackedConfig{})
	if code != OK {
		t.Fatal(code)
	}
	// Failed consumers must release only their own references.
	if d, code, _ := f.ParseDocument([]byte{1, 9}, 0, 0, true, false); d != nil || (code != Truncated && code != Unsupported) {
		t.Fatal(d, code)
	}
	if p, code, _ := f.CompileProgram([]byte("//["), ProgramOptions{}, false); p != nil || code == OK {
		t.Fatal(p, code)
	}
	document, docCode, _ := f.ParseDocument([]byte{1, 1, 7}, 0, 0, true, false)
	if docCode != OK && docCode != Unsupported {
		t.Fatal(docCode)
	}
	if document != nil {
		defer document.Close()
	}
	program, code, _ := f.CompileProgram([]byte("//01"), ProgramOptions{}, false)
	if code != OK {
		t.Fatal(code)
	}
	execution, code := program.Execution(8, 20, 10000, false)
	if code != OK {
		program.Close()
		t.Fatal(code)
	}
	program.Close()
	defer execution.Close()
	releaseFormatForTest(&f)
	runtime.GC()
	if code := execution.SetInput([]byte{1, 1, 7}, 0, true); code != OK {
		t.Fatal(code)
	}
	found, code, diagnostic := execution.Exists(false)
	if code != OK || !found {
		t.Fatal(found, code, diagnostic)
	}
	if document != nil {
		if document.Count() != 1 {
			t.Fatal(document.Count())
		}
		// A new destination descriptor must work after the source Go owner died.
		destination, code := NewFixed(FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: 1})
		if code != OK {
			t.Fatal(code)
		}
		if n, code := document.Encode(destination, make([]byte, 3), false); n != 3 || code != OK {
			t.Fatal(n, code)
		}
	}
}
