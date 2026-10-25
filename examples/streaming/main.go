// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package main

import (
	"errors"
	"fmt"

	tlv "github.com/OpenTLV/opentlv-go"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	tag, err := tlv.NewFixedIdentifier(1)
	if err != nil {
		return err
	}
	length, err := tlv.NewFixedLength(1, tlv.BigEndian)
	if err != nil {
		return err
	}
	format, err := tlv.NewComposed(tlv.CompositionConfig{Identifier: tag, Length: length,
		Constructed: &tlv.ConstructedBit{ByteIndex: 0, Mask: 0x20, Value: 0x20}})
	if err != nil {
		return err
	}
	wire := []byte{0x20, 3, 1, 1, 42}
	reader, err := tlv.NewIncrementalTreeReader(wire[:2], format, nil)
	if err != nil {
		return err
	}
	if reader.NextEvent() || reader.Err() != nil || !reader.NeedsMoreData() {
		return errors.New("expected incomplete parent")
	}
	if err := reader.SetInput(wire, 0, true); err != nil {
		return err
	}
	writer, err := tlv.NewTreeWriterBuffer(format, make([]byte, 1), nil)
	if err != nil {
		return err
	}
	for reader.NextEvent() {
		event := reader.Event()
		err := writer.WriteEvent(event)
		var capacity tlv.CapacityError
		if errors.As(err, &capacity) {
			if err := writer.SetBuffer(make([]byte, capacity.Required)); err != nil {
				return err
			}
			err = writer.WriteEvent(event) // Retry this event before the next pull.
		}
		if err != nil {
			return err
		}
		fmt.Printf("event=%d depth=%d offset=%d\n", event.Kind, event.Depth, event.Offset)
	}
	if err := reader.Err(); err != nil {
		return err
	}
	if err := writer.Finish(); err != nil {
		return err
	}
	fmt.Printf("wire=%x\n", writer.Bytes())
	return nil
}
