// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package main

import (
	"errors"
	"fmt"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
)

func main() {
	format, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	if err != nil {
		panic(err)
	}
	reader := opentlv.NewReader([]byte{1, 2, 42}, format)
	for reader.Next() {
	}
	err = reader.Err()
	if errors.Is(err, opentlv.ErrTruncated) {
		fmt.Println("incomplete final input")
	}
	var parsed *opentlv.ParseError
	if errors.As(err, &parsed) && parsed.HasOffset {
		fmt.Printf("parse error at offset %d, tag %X, declared %d, available %d\n", parsed.Offset, parsed.Tag, parsed.DeclaredLength.Value, parsed.Available.Value)
	}
}
