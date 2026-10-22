// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package main

import (
	"fmt"
	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
)

func main() {
	format, err := opentlv.NewFixed(opentlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian})
	if err != nil {
		panic(err)
	}
	doc, err := opentlv.Parse([]byte{1, 1, 42}, format)
	if err != nil {
		panic(err)
	}
	defer doc.Close()
	for _, element := range doc.Elements() {
		fmt.Printf("%X = %X\n", element.Tag(), element.Value())
	}
	root, err := doc.Element(0)
	if err != nil {
		panic(err)
	}
	if err = root.SetValue([]byte{43}); err != nil {
		panic(err)
	}
	writer := opentlv.NewWriter(format)
	if err = writer.WriteDocument(doc); err != nil {
		panic(err)
	}
	fmt.Printf("%X\n", writer.Bytes())
}
