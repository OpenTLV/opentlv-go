# Go module packaging and versioning

## Module identity

| Property | Value |
| --- | --- |
| Module / repository | `github.com/OpenTLV/opentlv-go` / `https://github.com/OpenTLV/opentlv-go` |
| Public package | `opentlv` |
| Minimum Go | 1.22 |
| Binding source version | `opentlv.BindingVersion`, currently `0.1.0-dev` |
| License | MIT; original Marek Cingel attribution in `LICENSE` |
| Native dependency | Immutable Core source commit in `core-source.json` |

The root `go.mod` is the canonical module identity. There are no third-party Go
dependencies, `replace` directives or required Go workspace. No `go.sum` is needed
in this module today. All C/cgo implementation remains private in `internal/capi`.
The module contains the bridge sources; it does not bundle the Core library.

The module ZIP carries the root `LICENSE` with original attribution,
`LICENSES/MIT.txt`, `REUSE.toml` and `THIRD_PARTY.md`. The package check rejects
missing licensing inputs and tests exact notice bytes in the ZIP. See
[dependencies and attribution](../THIRD_PARTY.md) for the Core and toolchain review
and notice handling when distributing linked binaries. Shared community policies
remain in `OpenTLV/.github`; they do not supply a module license.

## Independent versions and tags

`BindingVersion` identifies the Go binding source independently of Core. The
initial `0.1.0-dev` is an unreleased development version, not a Core version or a
claim that a public tag exists. Before a release, update this constant and the
changelog together. The tag is `v` plus that version, at the repository root
(for example `v0.1.0` or `v0.1.0-rc.1`), without a `bindings/go/` prefix.

While experimental (v0), incompatible API, minimum-Go or native linking-contract
changes increment the minor version; compatible fixes increment the patch.
Core lock updates require a new binding version and compatibility validation;
they never automatically copy Core's version. After v1, incompatible changes
require a major version and v2+ requires a matching `/v2` (etc.) module/import
suffix. Follow the [Go module version rules](https://go.dev/ref/mod#versions).

`Version()` retains its original meaning: the version string of the linked C
library, including a deliberately selected local Core. Its value is unrelated to
`BindingVersion`. The exact selected Go dependency version comes from
`go list -m github.com/OpenTLV/opentlv-go` or `runtime/debug.ReadBuildInfo`; an
untagged commit resolves to a pseudo-version even while the source constant says
`0.1.0-dev`. Neither value proves release publication or binary provenance.

## External consumption

Use the canonical import in your application:

```go
import opentlv "github.com/OpenTLV/opentlv-go"
```

Before a first release, select an exact **merged commit containing the canonical
module path** with `go get github.com/OpenTLV/opentlv-go@<commit>`. Go records its
pseudo-version. After publication, select the intended `vX.Y.Z` tag instead.
Do not use the old Core subtree path and the new path together: Go treats their
types and packages as distinct. There is no forwarding compatibility module.

For local development before merge, create a separate application directory,
run `go mod init example.test/app`, and add a local replacement explicitly:

```sh
go mod edit -require=github.com/OpenTLV/opentlv-go@v0.1.0-dev
go mod edit -replace=github.com/OpenTLV/opentlv-go=/absolute/path/to/opentlv-go
```

Build or run that application using the binding checkout's helper:

```sh
python3 /absolute/path/to/opentlv-go/scripts/go.py --working-dir /absolute/path/to/app -- run .
```

The helper builds the pinned Core and supplies matching native paths for the
external command. Keep the helper checkout aligned with the selected binding
revision. A dependency's Python helper is not executed automatically by `go get`
or `go build`. Direct Go commands require `CGO_ENABLED=1`, a matching C compiler,
`CGO_CPPFLAGS` with Core public and generated include directories, and
`CGO_LDFLAGS` with the matching library directory. Static builds additionally
define `TLV_STATIC_DEFINE`; shared builds need the runtime library search path.
See the [development guide](development/go.md) and
[cgo documentation](https://pkg.go.dev/cmd/cgo).

## Core compatibility

The supported source-build baseline is exactly commit
`cb3caed5b09706ce11ecfae839397fd4f99d0025`, recorded in `core-source.json`.
No wider Core version range or stable native ABI is claimed. Reader, Writer,
Query, Schema and Codec must be enabled. Minimal builds may disable ASN.1,
Bluetooth, LLDP, DHCP, NFC and Document; corresponding public APIs retain their
existing unavailable-feature errors. Use matching headers, generated config and
library from the same build and target. Windows requires GCC/MinGW compatible
with cgo; an MSVC library is not supported by this setup. No pure-Go fallback is
provided. Explicit local Core overrides are development evidence only.

## Packaging validation and migration limits

```sh
python3 scripts/go.py -- package-check
```

This builds the selected Core, checks module/version/license metadata, creates a
deterministic local module ZIP from repository sources, and serves it through a
temporary file proxy. A separate consumer with a fresh module cache downloads,
builds and runs the package without a `replace` directive or Go workspace. It
checks the resolved module version and a Reader/Writer round trip. The ZIP hash
and result are logged; the candidate and temporary consumer are removed afterward.
The local proxy disables the checksum database only for this private test. It
does not modify user Go settings. PR/Fast, Main and every Full configuration run
this check against their own Core configuration.

Migration intentionally uses source-built Core rather than a verified release
binary. The local ZIP has synthetic version/time metadata solely for testing;
it is not an official release artifact. Public proxy retrieval, a published tag,
checksum database registration and release readiness remain unclaimed. Publishing
requires a separate approved release with successful hosted validation of the
exact revision. This migration creates no tag or release.

## Release process handoff

The independently dispatched `go-full.yml` workflow is the technical release
quality gate. Before authorizing a module tag, select the exact committed binding
version and Core source lock, run Full on that commit, and require all six native
configurations plus the attempt-matched Go quality gate. Each configuration
includes the external-consumer package check. Retain the commit, run, attempt
and artifact digests with the release decision; PR/Main or historical Full
results cannot replace that candidate's Full evidence.

The current `0.1.0-dev` metadata supports migration development and validation.
A release owner separately approves a publishable version, changelog, tag and
publication. Migration completion establishes that this process can be followed;
it does not grant publication permission or claim public proxy availability.
