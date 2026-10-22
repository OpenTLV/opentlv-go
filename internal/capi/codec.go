// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Marek Cingel

package capi

/*
#include <tlv/codec/values.h>
#include <tlv/codec/number.h>
#include <tlv/codec/text.h>
#include <tlv/codec/digits.h>
#include <tlv/codec/ipv4.h>

typedef struct { int kind, encoding, padding; size_t width; unsigned digits; } go_codec;
typedef struct { tlv_codec_diagnostic_t diagnostic; int code; uint64_t number; int64_t signed_number; const uint8_t* bytes; size_t size; } go_codec_result;
static go_codec_result go_convert(go_codec c, const uint8_t* input, size_t size,
 uint64_t number, int64_t signed_number, uint8_t* output, size_t capacity, int encode) {
 go_codec_result r = {0};
 tlv_codec_diagnostic_init(&r.diagnostic, encode ? (output ? TLV_CODEC_OP_ENCODE : TLV_CODEC_OP_MEASURE) : TLV_CODEC_OP_DECODE);
 tlv_number_codec_config_t nc = {(tlv_number_encoding_t)c.encoding,c.width,c.digits};
 tlv_text_codec_config_t tc = {(tlv_text_alphabet_t)c.encoding,c.width,c.padding};
 tlv_digits_codec_config_t dc = {c.width};
 tlv_codec_t codec;
 union { uint8_t u8; uint16_t u16; uint32_t u32; uint64_t u64; int64_t i64; tlv_value_t span; tlv_ipv4_t ip; tlv_ipv4_list_t ips; } v;
 void* value = &v; size_t object_size = 0;
 switch(c.kind) {
 case 1: codec=tlv_codec_uint8; v.u8=(uint8_t)number; object_size=sizeof(v.u8); if(encode && number>UINT8_MAX) goto invalid; break;
 case 2: case 4: codec=c.kind==2?tlv_codec_uint16_be:tlv_codec_uint16_le; v.u16=(uint16_t)number; object_size=sizeof(v.u16); if(encode && number>UINT16_MAX) goto invalid; break;
 case 3: case 5: codec=c.kind==3?tlv_codec_uint32_be:tlv_codec_uint32_le; v.u32=(uint32_t)number; object_size=sizeof(v.u32); if(encode && number>UINT32_MAX) goto invalid; break;
 case 6: codec=tlv_codec_int64_minimal_be; v.i64=signed_number; object_size=sizeof(v.i64); break;
 case 7: codec=tlv_number_codec(&nc); v.u64=number; object_size=sizeof(v.u64); break;
 case 8: case 9: codec=c.kind==8?tlv_codec_bytes:tlv_text_codec(&tc); v.span.data=input; v.span.size=size; object_size=sizeof(v.span); break;
 case 10: codec=tlv_digits_codec(&dc); value=encode?(void*)input:output; object_size=encode?size:capacity; break;
 case 11: codec=tlv_codec_ipv4; object_size=sizeof(v.ip); if(encode) { if(size!=4) goto invalid; for(size_t i=0;i<4;i++) v.ip.bytes[i]=input[i]; } break;
 case 12: codec=tlv_codec_ipv4_list; v.ips.raw.data=input; v.ips.raw.size=size; object_size=sizeof(v.ips); break;
 default: r.code=TLV_ERR_UNSUPPORTED; return r;
 }
 if(encode) { r.code=tlv_codec_encode(&codec,value,object_size,output,capacity,&r.size,&r.diagnostic); return r; }
 r.code=tlv_codec_decode(&codec,input,size,value,object_size,&r.diagnostic);
 if(r.code) return r;
 switch(c.kind) {
 case 1: r.number=v.u8; break;
 case 2: case 4: r.number=v.u16; break;
 case 3: case 5: r.number=v.u32; break;
 case 6: r.signed_number=v.i64; break;
 case 7: r.number=v.u64; break;
 case 8: case 9: r.bytes=v.span.data; r.size=(size_t)v.span.size; break;
 case 10: r.bytes=output; while(r.size<capacity && output[r.size]) r.size++; break;
 case 11: for(size_t i=0;i<4;i++) output[i]=v.ip.bytes[i]; r.bytes=output; r.size=4; break;
 case 12: r.bytes=v.ips.raw.data; r.size=(size_t)v.ips.raw.size; break;
 }
 return r;
invalid: r.code=TLV_ERR_INVALID_VALUE; return r;
}
*/
import "C"
import (
	"bytes"
	"runtime"
)

// CodecConfig selects a canonical C value descriptor and its call-local context.
type CodecConfig struct {
	Kind, Encoding, Width, Digits int
	Padding                       bool
}

// CodecMessage returns the canonical codec error description.
func CodecMessage(code int) string {
	return C.GoString(C.tlv_result_string(C.tlv_result_t(code)))
}
func (c CodecConfig) native() (C.go_codec, bool) {
	if c.Width < 0 || c.Digits < 0 || uint64(c.Digits) > uint64(^uint32(0)) || c.Encoding < 0 || c.Encoding > 2 {
		return C.go_codec{}, false
	}
	p := 0
	if c.Padding {
		p = 1
	}
	return C.go_codec{kind: C.int(c.Kind), encoding: C.int(c.Encoding), width: C.size_t(c.Width), digits: C.uint(c.Digits), padding: C.int(p)}, true
}

// Decode copies any borrowed native representation before returning.
func (c CodecConfig) Decode(input []byte) (uint64, int64, []byte, int, *CodecDetail) {
	n, ok := c.native()
	if !ok {
		return 0, 0, nil, int(InvalidArg), nil
	}
	capacity := 4
	if c.Kind == 10 {
		if len(input) > (int(^uint(0)>>1)-1)/2 {
			return 0, 0, nil, int(Overflow), nil
		}
		capacity = len(input)*2 + 1
	}
	output := make([]byte, capacity)
	r := C.go_convert(n, bytePointer(input), C.size_t(len(input)), 0, 0, bytePointer(output), C.size_t(len(output)), 0)
	defer runtime.KeepAlive(input)
	defer runtime.KeepAlive(output)
	if r.code != 0 {
		return 0, 0, nil, int(r.code), codecDetail(r.diagnostic.diagnostic, r.diagnostic.codec)
	}
	return uint64(r.number), int64(r.signed_number), bytes.Clone(nativeBytes(r.bytes, r.size)), 0, nil
}

// Encode validates and measures through C before allocating output.
func (c CodecConfig) Encode(number uint64, signed int64, input []byte) ([]byte, int, *CodecDetail) {
	n, ok := c.native()
	if !ok {
		return nil, int(InvalidArg), nil
	}
	// Digits requires a non-NULL object even for the empty string.
	if c.Kind == 10 && len(input) == 0 {
		input = make([]byte, 0, 1)
	}
	r := C.go_convert(n, bytePointer(input), C.size_t(len(input)), C.uint64_t(number), C.int64_t(signed), nil, 0, 1)
	defer runtime.KeepAlive(input)
	if r.code != 0 {
		return nil, int(r.code), codecDetail(r.diagnostic.diagnostic, r.diagnostic.codec)
	}
	if uint64(r.size) > uint64(^uint(0)>>1) {
		return nil, int(NativeSize), nil
	}
	output := make([]byte, int(r.size))
	r = C.go_convert(n, bytePointer(input), C.size_t(len(input)), C.uint64_t(number), C.int64_t(signed), bytePointer(output), C.size_t(len(output)), 1)
	runtime.KeepAlive(output)
	if r.code != 0 {
		return nil, int(r.code), codecDetail(r.diagnostic.diagnostic, r.diagnostic.codec)
	}
	return output, 0, nil
}
