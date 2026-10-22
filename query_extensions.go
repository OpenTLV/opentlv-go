// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel
package opentlv

import (
	"bytes"
	"errors"
	"github.com/marekcingel/OpenTLV/bindings/go/internal/capi"
)

// QueryTagAdapter supplies semantic Tag capabilities with a nonzero stable image
// compatibility ID. Class and Number are independently optional. Callbacks own
// copied Tag input and must support concurrent independent executions. Native
// StatusError identities propagate; other errors and panics become InvalidValue.
type QueryTagAdapter struct {
	ID            uint32
	Class, Number func([]byte) (int64, error)
}

// QueryResolver resolves a scoped symbolic name during compilation only. Empty
// namespace means unqualified lookup. It must return identical bytes in both
// compilation passes. Returned bytes are copied immediately. Native StatusError
// identities propagate; other errors and panics become InvalidValue.
type QueryResolver func(namespace, name string) ([]byte, error)

// QueryEMV resolves native EMV base-dictionary symbols in the emv namespace or
// without a namespace, including PAN/pan. A library without EMV support returns
// ErrUnsupported during compilation.
func QueryEMV() QueryResolver {
	return func(namespace, name string) ([]byte, error) {
		tag, code := capi.ResolveEMV(namespace, name)
		if code != capi.OK {
			return nil, StatusError{code: code}
		}
		return tag, nil
	}
}

// QueryDefinition contains generic descriptive identifier metadata.
type QueryDefinition struct {
	Tag  []byte
	Name string
}

// QueryDefinitionScope identifies an explicit namespace of descriptive labels.
type QueryDefinitionScope struct {
	Namespace   string
	Definitions []QueryDefinition
}

// QueryDefinitions snapshots immutable definitions and delegates lookup to C.
// Unqualified names search every scope; ambiguity remains ErrInvalidArg even
// when multiple matching labels contain identical Tag bytes.
func QueryDefinitions(scopes []QueryDefinitionScope) QueryResolver {
	var definitions []capi.QueryDefinition
	for _, scope := range scopes {
		for _, definition := range scope.Definitions {
			definitions = append(definitions, capi.QueryDefinition{Namespace: scope.Namespace,
				Name: definition.Name, Tag: bytes.Clone(definition.Tag)})
		}
	}
	return func(namespace, name string) ([]byte, error) {
		tag, code := capi.ResolveDefinitions(definitions, namespace, name)
		if code != capi.OK {
			return nil, StatusError{code: code}
		}
		return tag, nil
	}
}

func queryCallbackCode(err error) capi.Code {
	if err == nil {
		return capi.OK
	}
	var native interface{ Code() int }
	if errors.As(err, &native) {
		return capi.Code(native.Code())
	}
	return capi.InvalidValue
}
