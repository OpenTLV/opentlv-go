# OpenTLV Go v0.1.0

First standalone Go release: the module `github.com/OpenTLV/opentlv-go` is
versioned as **0.1.0** (`opentlv.BindingVersion`), independently of Core.
`opentlv.Version()` still reports the linked Core library.

- Builds against OpenTLV Core **v0.8.0** at
  `345c77acb29266036df5f27d04ce315bb61853cc`, recorded in `core-source.json`
  and built from that source commit by `scripts/go.py`. No wider Core range or
  stable native ABI is claimed.
- Includes the Reader/Writer, streaming tree and visitor, generic field and
  composed Format, Document, Query, Schema, generic Codec, structure codec,
  value constraint and deterministic Generator APIs with owned Go diagnostics.
- Requires Go 1.22 or newer with cgo, CMake, Ninja, Git, Python 3.9 or newer
  and a C compiler supported by cgo. Windows uses GCC/MinGW; MSVC libraries are
  not supported. There is no pure-Go fallback.
- Select it with `go get github.com/OpenTLV/opentlv-go@v0.1.0`. The module
  contains Go and cgo bridge sources only; build the matching Core with the
  helper as described in
  [packaging](https://github.com/OpenTLV/opentlv-go/blob/v0.1.0/docs/packaging.md).
  The GitHub Release carries the release notes, the Full CI evidence manifest
  and SHA-256 checksums; it contains no binaries.

The API is experimental (v0). Generic Core symbol references are reviewed
against the pin, but they do not prove functional parity, and Builtins-specific
parity remains outside this release.
