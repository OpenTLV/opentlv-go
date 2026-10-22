// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package main

import (
	"fmt"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
	"log"
)

func main() {
	codec := opentlv.NumberCodec(opentlv.NumberConfig{Encoding: opentlv.NumberBCD, Width: 3, Digits: 6})
	encoded, err := codec.Encode(1234)
	if err != nil {
		log.Fatal(err)
	}
	value, err := codec.Decode(encoded)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%x = %d\n", encoded, value)
}
