// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package tests

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Guard the package boundary as additional APIs are introduced.
func TestCAPIImportsStayInternal(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == "build" || entry.Name() == ".git") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel("..", path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.ToSlash(rel), "internal/capi/") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if name == "C" || name == "unsafe" {
				t.Errorf("%s imports %q outside internal/capi", rel, name)
			}
			if strings.HasPrefix(filepath.ToSlash(rel), "examples/") &&
				strings.HasPrefix(name, "github.com/marekcingel/OpenTLV/bindings/go/") {
				t.Errorf("%s imports %q instead of the public opentlv package", rel, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
