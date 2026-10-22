// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
// Common Query corpus adapter through the public Go API.
package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	tlv "github.com/OpenTLV/opentlv-go"
)

func fail(err error) {
	var diagnostic *tlv.ProgramError
	if errors.As(err, &diagnostic) {
		// The expression span is primary, or related evidence beside another location.
		span := diagnostic.Expression
		if diagnostic.Location.Domain == tlv.LocationExpression {
			span = diagnostic.Location
		}
		fmt.Fprintf(os.Stderr, "%d %d %d %d\n", diagnostic.Code(), diagnostic.Kind, span.Begin, span.End)
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}
func scalar(value tlv.QueryValue) {
	switch value.Type {
	case tlv.QueryBoolean:
		if value.Boolean {
			fmt.Println("bool:1")
		} else {
			fmt.Println("bool:0")
		}
	case tlv.QueryInteger:
		fmt.Printf("int:%d\n", value.Integer)
	case tlv.QueryBytes:
		fmt.Printf("bytes:%x\n", value.Bytes)
	case tlv.QueryString:
		fmt.Printf("string:%x\n", []byte(value.String))
	}
}
func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--capabilities" {
		if _, err := tlv.Builtin(tlv.BER); err == nil {
			fmt.Println("asn1")
		}
		f, _ := tlv.NewFixed(tlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: tlv.BigEndian})
		if d, err := tlv.Parse(nil, f); err == nil {
			fmt.Println("document")
			_ = d.Close()
		}
		return
	}
	if len(args) > 0 && args[0] == "--language-features" {
		return
	}
	mode := "o"
	if len(args) > 2 {
		mode = args[2]
	}
	f, err := tlv.Builtin(tlv.BER)
	if err != nil {
		fail(err)
	}
	options := tlv.ProgramOptions{Format: f, Optimize: !strings.Contains(mode, "u"), Variables: map[string]tlv.QueryType{}, Names: map[string][]byte{"fixture:leaf": {0x5a}, "fixture:container": {0x70}}}
	bindings := map[string]tlv.QueryValue{}
	for _, declaration := range args[min(4, len(args)):] {
		fields := strings.SplitN(declaration, ":", 3)
		value := tlv.QueryValue{}
		switch fields[1] {
		case "int":
			value.Type = tlv.QueryInteger
			value.Integer, err = strconv.ParseInt(fields[2], 10, 64)
		case "bytes":
			value.Type = tlv.QueryBytes
			value.Bytes, err = hex.DecodeString(fields[2])
		case "string":
			value.Type = tlv.QueryString
			value.String = fields[2]
		}
		if err != nil {
			fail(err)
		}
		options.Variables[fields[0]] = value.Type
		bindings[fields[0]] = value
	}
	p, err := tlv.CompileQuery(args[0], options)
	if err != nil {
		fail(err)
	}
	defer p.Close()
	info, err := p.Info()
	if err != nil {
		fail(err)
	}
	q, err := p.Execution(tlv.QueryLimits{Depth: 128, Nodes: 1024, Work: 100000000}, strings.ContainsAny(mode, "rd") || info["level"] >= 2)
	if err != nil {
		fail(err)
	}
	defer q.Close()
	variables, err := p.Variables()
	if err != nil {
		fail(err)
	}
	for name := range variables {
		if err := q.Bind(name, bindings[name]); err != nil {
			fail(err)
		}
	}
	wire, err := hex.DecodeString(args[1])
	if err != nil {
		fail(err)
	}
	if strings.Contains(mode, "d") || info["level"] == 3 {
		d, err := tlv.ParseWithOptions(wire, f, tlv.DocumentOptions{
			MaxDepth: 128, MaxElements: 1024, RetainSourceLocations: true,
		})
		if err != nil {
			fail(err)
		}
		defer d.Close()
		if err := q.EvaluateDocument(d, tlv.Node{}, -1); err != nil {
			fail(err)
		}
		if info["result_kind"] != 0 {
			value, err := q.Result()
			if err != nil {
				fail(err)
			}
			scalar(value)
			return
		}
		// Fixture framing is independently generated one-byte TLV. This mapping
		// only associates public checked identities with fixture source offsets.
		offsets := map[uint64]int{}
		type pending struct {
			node   tlv.Node
			offset int
		}
		stack := []pending{}
		at := 0
		for _, root := range d.Elements() {
			stack = append(stack, pending{root, at})
			at += 2 + int(wire[at+1])
		}
		for len(stack) > 0 {
			item := stack[0]
			stack = stack[1:]
			id, err := item.node.Identity()
			if err != nil {
				fail(err)
			}
			offsets[id] = item.offset
			at = item.offset + 2
			children := []pending{}
			for _, child := range item.node.Children() {
				children = append(children, pending{child, at})
				at += 2 + int(wire[at+1])
			}
			stack = append(children, stack...)
		}
		for {
			node, err := q.NextDocument()
			if errors.Is(err, tlv.ErrEnd) {
				break
			}
			if err != nil {
				fail(err)
			}
			id, err := node.Identity()
			if err != nil {
				fail(err)
			}
			fmt.Println(offsets[id])
		}
		return
	}
	visit := func(match tlv.QueryMatch) tlv.QueryVisit { fmt.Println(match.Offset); return tlv.QueryContinue }
	if len(args) > 3 {
		split, err := strconv.Atoi(args[3])
		if err != nil {
			fail(err)
		}
		if err := q.SetInput(wire[:split], 0, false); err != nil {
			fail(err)
		}
		if err := q.Visit(visit); err != nil && !errors.Is(err, tlv.ErrNeedMoreData) {
			fail(err)
		}
	}
	if err := q.SetInput(wire, 0, true); err != nil {
		fail(err)
	}
	if err := q.Visit(visit); err != nil {
		fail(err)
	}
	if info["result_kind"] != 0 {
		value, err := q.Result()
		if err != nil {
			fail(err)
		}
		scalar(value)
	}
}
