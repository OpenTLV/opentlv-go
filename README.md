# opentlv-go

Go bindings for OpenTLV — a universal library for parsing, encoding, and working
with TLV and TLV-like binary formats.

This repository is being prepared for the behavior-preserving extraction tracked
by [OpenTLV/opentlv#684](https://github.com/OpenTLV/opentlv/issues/684).
The existing implementation remains in Core at the recorded baseline; this
bootstrap repository does not yet contain a consumable Go module.

See [MIGRATION.md](MIGRATION.md) for the exact source revision, license attribution,
ownership boundaries and implementation handoff. Source transfer is tracked in
[Go #1](https://github.com/OpenTLV/opentlv-go/issues/1); standalone Core dependency
integration in [Go #2](https://github.com/OpenTLV/opentlv-go/issues/2).

The binding will own its public Go API, internal cgo bridge, Go tests, examples,
documentation and independent release metadata. Core continues to own native
semantics and comprehensive C tests.

Licensed under the [MIT License](LICENSE).
