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
	writer := opentlv.NewWriter(format)
	if err = writer.WriteElement([]byte{1}, []byte{42}); err != nil {
		log.Fatal(err)
	}
	if err = writer.Finish(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("% X\n", writer.Bytes())
}
