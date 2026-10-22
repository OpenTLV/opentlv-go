// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package main

import (
	"fmt"
	"log"

	opentlv "github.com/marekcingel/OpenTLV/bindings/go"
)

func run() error {
	format, err := opentlv.NewFixed(opentlv.FixedConfig{
		TagSize: 1, LengthSize: 1, ByteOrder: opentlv.BigEndian,
	})
	if err != nil {
		return err
	}
	writer := opentlv.NewWriter(format)
	if err := writer.WriteElement([]byte{0x01}, []byte("Hello, world!")); err != nil {
		return err
	}

	// Elements borrow the Writer's output; keep it unchanged while reading.
	reader := opentlv.NewReader(writer.Bytes(), format)
	for reader.Next() {
		fmt.Println(string(reader.Element().Value()))
	}
	return reader.Err()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
