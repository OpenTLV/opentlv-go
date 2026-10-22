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
	doc, err := opentlv.Parse([]byte{1, 2, 0, 42, 1, 2, 0, 7}, format)
	if err != nil {
		log.Fatal(err)
	}
	defer doc.Close()
	matches, err := doc.Query("01")
	if err != nil {
		log.Fatal(err)
	}
	for _, node := range matches {
		value, err := opentlv.Uint16BECodec().Decode(node.Value())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(value)
	}
}
