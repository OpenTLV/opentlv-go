// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package opentlv

import "github.com/marekcingel/OpenTLV/bindings/go/internal/capi"

// ByteOrder determines the wire byte order of fixed numeric fields.
type ByteOrder int

const (
	// BigEndian places the most significant byte first.
	BigEndian ByteOrder = 1
	// LittleEndian places the least significant byte first.
	LittleEndian ByteOrder = 2
)

// ElementOrder determines the ordering of the wire fields.
type ElementOrder int

const (
	// TLV orders Tag, Length and Value.
	TLV ElementOrder = iota
	// LTV orders Length, Tag and Value.
	LTV
)

// LengthScope determines which bytes a fixed Length counts.
type LengthScope int

const (
	// ValueLength counts only Value bytes.
	ValueLength LengthScope = iota
	// TagAndValueLength counts Tag and Value bytes.
	TagAndValueLength
)

// FixedConfig configures generic fixed-width fields. Set ByteOrder explicitly.
// Zero TagSize represents an absent Tag where supported by the C engine.
type FixedConfig struct {
	TagSize, LengthSize int
	ByteOrder           ByteOrder
	Order               ElementOrder
	LengthScope         LengthScope
}

// FormatKind identifies a built-in preset; availability follows the linked library.
type FormatKind int

const (
	// BER selects definite-length BER.
	BER FormatKind = FormatKind(capi.BER)
	// BERIndefinite selects BER with indefinite-length support.
	BERIndefinite FormatKind = FormatKind(capi.BERIndefinite)
	// DER selects Distinguished Encoding Rules.
	DER FormatKind = FormatKind(capi.DER)
	// CER selects Canonical Encoding Rules.
	CER FormatKind = FormatKind(capi.CER)
	// EMV selects EMV BER-TLV framing.
	EMV FormatKind = FormatKind(capi.EMV)
	// LLDP selects LLDP framing.
	LLDP FormatKind = FormatKind(capi.LLDP)
	// BluetoothLTV selects Bluetooth advertising LTV framing.
	BluetoothLTV FormatKind = FormatKind(capi.BluetoothLTV)
	// DHCPv4 selects DHCPv4 option framing.
	DHCPv4 FormatKind = FormatKind(capi.DHCPv4)
	// NFCType2 selects NFC Type 2 data-area TLV framing.
	NFCType2 FormatKind = FormatKind(capi.NFCType2)
)

// Format is immutable configuration for the canonical C engine. Copies may be
// shared concurrently. It owns no native resources and requires no Close.
// Its zero value is invalid; use NewFixed or Builtin.
type Format struct {
	native capi.Format
	valid  bool
}

// Valid reports whether this Format was successfully constructed.
func (f Format) Valid() bool { return f.valid }

// StatusError preserves a native failure code as a Go error. It owns no C storage.
// Use errors.Is with the named Err values. Processing failures wrap this status
// in ParseError or WriteError; errors.As can still retrieve a StatusError.
type StatusError struct{ code capi.Code }

// Error returns the canonical native status description.
func (e StatusError) Error() string { return e.code.String() }

// Code returns the canonical C status number without exposing a C type.
func (e StatusError) Code() int { return int(e.code) }
func formatResult(f capi.Format, code capi.Code) (Format, error) {
	if code != capi.OK {
		return Format{}, StatusError{code: code}
	}
	return Format{native: f, valid: true}, nil
}

// NewFixed validates a snapshot of config through the canonical C engine.
// Later changes to config do not affect the returned Format.
func NewFixed(config FixedConfig) (Format, error) {
	f, code := capi.NewFixed(capi.FixedConfig{TagSize: config.TagSize, LengthSize: config.LengthSize, ByteOrder: int(config.ByteOrder), Order: int(config.Order), LengthScope: int(config.LengthScope)})
	return formatResult(f, code)
}

// Builtin selects a compiled-in preset. Unknown or disabled presets return an error
// and an invalid Format; there is no fallback to another format.
func Builtin(kind FormatKind) (Format, error) {
	f, code := capi.Builtin(capi.Kind(kind))
	return formatResult(f, code)
}
