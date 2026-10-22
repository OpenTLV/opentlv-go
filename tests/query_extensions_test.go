// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package tests

import (
	"bytes"
	"errors"
	tlv "github.com/marekcingel/OpenTLV/bindings/go"
	"testing"
)

func TestQueryTagAdaptersAndCheckedResolvers(t *testing.T) {
	format, err := tlv.NewFixed(tlv.FixedConfig{TagSize: 1, LengthSize: 1, ByteOrder: tlv.BigEndian})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	options := tlv.ProgramOptions{Format: format, Optimize: true, Tags: &tlv.QueryTagAdapter{
		ID: 1001, Class: func([]byte) (int64, error) { return 17, nil },
		Number: func(tag []byte) (int64, error) { calls++; return int64(tag[0]) + 100, nil },
	}}
	program, err := tlv.CompileQuery("//5A[class()=17 and number()=190]", options)
	if err != nil {
		t.Fatal(err)
	}
	image, err := program.Image()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := tlv.LoadQuery(image, options)
	if err != nil {
		t.Fatal(err)
	}
	program.Close()
	execution, err := loaded.Execution(tlv.DefaultQueryLimits(), true)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Close()
	defer execution.Close()
	if err := execution.SetInput([]byte{0x5a, 0}, 0, true); err != nil {
		t.Fatal(err)
	}
	if _, err := execution.Next(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls %d", calls)
	}
	options.Tags.ID = 1002
	if unexpected, err := tlv.LoadQuery(image, options); err == nil {
		unexpected.Close()
		t.Fatal("foreign Tag adapter ID accepted")
	}
	options.Tags = nil
	if unexpected, err := tlv.LoadQuery(image, options); err == nil {
		unexpected.Close()
		t.Fatal("missing Tag adapter accepted")
	}
	for _, panicCallback := range []bool{false, true} {
		options.Tags = &tlv.QueryTagAdapter{ID: 1003, Number: func([]byte) (int64, error) {
			if panicCallback {
				panic("tag")
			}
			return 0, tlv.ErrInvalidTag
		}}
		p, err := tlv.CompileQuery("//5A[number()=1]", options)
		if err != nil {
			t.Fatal(err)
		}
		q, err := p.Execution(tlv.DefaultQueryLimits(), true)
		if err != nil {
			t.Fatal(err)
		}
		p.Close()
		if err := q.SetInput([]byte{0x5a, 0}, 0, true); err != nil {
			t.Fatal(err)
		}
		_, err = q.Next()
		q.Close()
		expected := error(tlv.ErrInvalidTag)
		if panicCallback {
			expected = tlv.ErrInvalidValue
		}
		if !errors.Is(err, expected) {
			t.Fatalf("tag error %v", err)
		}
	}
	options.Tags = nil
	options.Resolver = func(namespace, name string) ([]byte, error) {
		if namespace != "app" || name != "payload" {
			t.Fatalf("scope %q:%q", namespace, name)
		}
		return []byte{0x5a}, nil
	}
	p, err := tlv.CompileQuery("//app:payload", options)
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
	calls = 0
	options.Resolver = func(string, string) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte{0x5a}, nil
		}
		return []byte{0x5b}, nil
	}
	if p, err := tlv.CompileQuery("//app:payload", options); !errors.Is(err, tlv.ErrInvalidValue) {
		if p != nil {
			p.Close()
		}
		t.Fatalf("same-size resolver drift: %v", err)
	}
}

func TestQueryDefinitionsFixedFormatsAndSourceFeeds(t *testing.T) {
	format, err := tlv.NewFixed(tlv.FixedConfig{TagSize: 2, LengthSize: 1, ByteOrder: tlv.BigEndian, Order: tlv.LTV, LengthScope: tlv.TagAndValueLength})
	if err != nil {
		t.Fatal(err)
	}
	scopes := []tlv.QueryDefinitionScope{{Namespace: "a", Definitions: []tlv.QueryDefinition{{Tag: []byte{0x12, 0x34}, Name: "payload"}}},
		{Namespace: "b", Definitions: []tlv.QueryDefinition{{Tag: []byte{0x12, 0x34}, Name: "payload"}}}}
	options := tlv.ProgramOptions{Format: format, Optimize: true, Resolver: tlv.QueryDefinitions(scopes)}
	scopes[0].Definitions[0].Tag[0] = 0xff
	if p, err := tlv.CompileQuery("//payload", options); !errors.Is(err, tlv.ErrInvalidArg) {
		if p != nil {
			p.Close()
		}
		t.Fatalf("ambiguity %v", err)
	}
	p, err := tlv.CompileQuery("//a:payload[@offset=9 and @hlen=3]", options)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	q, err := p.Execution(tlv.DefaultQueryLimits(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	wire := []byte{3, 0x12, 0x34, 0x56}
	if _, err := q.FeedEncoded(wire, 0, 9); err != nil {
		t.Fatal(err)
	}
	wire[3] = 0
	if err := q.Finish(); err != nil {
		t.Fatal(err)
	}
	match, ordinal, err := q.NextResultWithOrdinal()
	if err != nil {
		t.Fatal(err)
	}
	if ordinal != 0 || !bytes.Equal(match.Element.Tag(), []byte{0x12, 0x34}) || !bytes.Equal(match.Element.Value(), []byte{0x56}) {
		t.Fatalf("match %#v ordinal %d", match, ordinal)
	}
	if _, _, err := q.NextResultWithOrdinal(); !errors.Is(err, tlv.ErrEnd) {
		t.Fatal(err)
	}
	context, err := tlv.CompileQuery("//a:payload", options)
	if err != nil {
		t.Fatal(err)
	}
	defer context.Close()
	assertion, err := tlv.CompileQuery("@len=1", options)
	if err != nil {
		t.Fatal(err)
	}
	defer assertion.Close()
	if err := tlv.ValidateQueryBuffer([]tlv.QueryRule{{Context: context, Assertion: assertion}}, []byte{3, 0x12, 0x34, 0x56}, format, tlv.DefaultQuerySchemaLimits()); err != nil {
		t.Fatal(err)
	}
	independentFormat, err := tlv.NewFixed(tlv.FixedConfig{TagSize: 2, LengthSize: 1, ByteOrder: tlv.BigEndian, Order: tlv.LTV, LengthScope: tlv.TagAndValueLength})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := tlv.Parse([]byte{3, 0x12, 0x34, 0x56}, independentFormat)
	if errors.Is(err, tlv.ErrUnsupported) {
		return // Buffer extensions remain available without the Document component.
	}
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	if err := tlv.ValidateQueryDocument([]tlv.QueryRule{{Context: context, Assertion: assertion}}, doc, tlv.DefaultQuerySchemaLimits()); err != nil {
		t.Fatal(err)
	}
	execution, err := context.Execution(tlv.DefaultQueryLimits(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer execution.Close()
	if err := execution.EvaluateDocument(doc, tlv.Node{}, -1); err != nil {
		t.Fatal(err)
	}
	if _, err := execution.NextDocument(); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 5; index++ {
		config := tlv.FixedConfig{TagSize: 2, LengthSize: 1, ByteOrder: tlv.BigEndian, Order: tlv.LTV, LengthScope: tlv.TagAndValueLength}
		switch index {
		case 0:
			config.TagSize = 3
		case 1:
			config.LengthSize = 2
		case 2:
			config.ByteOrder = tlv.LittleEndian
		case 3:
			config.Order = tlv.TLV
		case 4:
			config.LengthScope = tlv.ValueLength
		}
		different, err := tlv.NewFixed(config)
		if err != nil {
			t.Fatal(err)
		}
		other, err := tlv.Parse(nil, different)
		if err != nil {
			t.Fatal(err)
		}
		if err := execution.Reset(); err != nil {
			t.Fatal(err)
		}
		err = execution.EvaluateDocument(other, tlv.Node{}, 0)
		other.Close()
		if !errors.Is(err, tlv.ErrInvalidArg) {
			t.Fatalf("mismatched Fixed field %d: %v", index, err)
		}
	}
}

func TestDocumentSourceLocationsSupportGlobalAxes(t *testing.T) {
	format, err := tlv.Builtin(tlv.BER)
	if err != nil {
		t.Skip(err)
	}
	doc, err := tlv.ParseWithOptions([]byte{0x50, 0, 0x57, 1, 0xaa}, format,
		tlv.DocumentOptions{MaxDepth: 4, MaxElements: 4, RetainSourceLocations: true})
	if errors.Is(err, tlv.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	program, err := tlv.CompileQuery("//50/following::57[@offset=2 and @hlen=2]",
		tlv.ProgramOptions{Format: format, Optimize: true})
	if err != nil {
		t.Fatal(err)
	}
	defer program.Close()
	execution, err := program.Execution(tlv.DefaultQueryLimits(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer execution.Close()
	if err := execution.EvaluateDocument(doc, tlv.Node{}, -1); err != nil {
		t.Fatal(err)
	}
	node, err := execution.NextDocument()
	if err != nil || !node.Valid() {
		t.Fatalf("missing located node: %v", err)
	}
}

func TestQueryNativeEMVResolver(t *testing.T) {
	format, err := tlv.Builtin(tlv.EMV)
	if err != nil {
		t.Skip(err)
	}
	options := tlv.ProgramOptions{Format: format, Optimize: true, Resolver: tlv.QueryEMV()}
	for _, text := range []string{"//emv:PAN", "//pan"} {
		program, err := tlv.CompileQuery(text, options)
		if err != nil {
			t.Fatal(err)
		}
		program.Close()
	}
	if program, err := tlv.CompileQuery("//other:PAN", options); err == nil {
		program.Close()
		t.Fatal("unknown namespace accepted")
	}
}
