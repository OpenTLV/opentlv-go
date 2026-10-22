// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package main

import (
	"fmt"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"log"
)

func main() {
	format, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	if err != nil {
		log.Fatal(err)
	}
	reader := opentlv.NewReader([]byte{1, 1, 0xAA, 2, 0}, format)
	for reader.Next() {
		fmt.Printf("%X @ %d: %X\n", reader.Element().Tag(), reader.Element().Offset(), reader.Element().Value())
	}
	if err := reader.Err(); err != nil {
		log.Fatal(err)
	}
}
