# Go module packaging and versioning

## Module identity

| Property | Value |
| --- | --- |
| Module / repository | `github.com/OpenTLV/opentlv-go` / `https://github.com/OpenTLV/opentlv-go` |
| Public package | `opentlv` |
| Minimum Go | 1.22 |
| Binding source version | `opentlv.BindingVersion`, currently `0.1.0` (first release, tag `v0.1.0`) |
| License | MIT; original Marek Cingel attribution in `LICENSE` |
| Native dependency | Core `v0.8.0` source commit in `core-source.json` |

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
first Go release is `0.1.0`, prepared against Core `v0.8.0` (#31); it is not a
Core version. Migration development used the unreleased `0.1.0-dev`; there were
no Go repository tags or GitHub releases when checked on 2026-10-28. Before a
release, update this constant, `docs-examples.json`, `ci/release-notes.md` and
the changelog together. The tag is `v` plus that version, at the repository root
(for example `v0.1.0` or `v0.1.0-rc.1`), without a `bindings/go/` prefix. The
release workflow publishes stable `vMAJOR.MINOR.PATCH` tags only.

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
untagged commit resolves to a pseudo-version, whatever the source constant says.
Neither value proves release publication or binary provenance.

## External consumption

Use the canonical import in your application:

```go
import opentlv "github.com/OpenTLV/opentlv-go"
```

Select the release tag with `go get github.com/OpenTLV/opentlv-go@v0.1.0`; the
public Go module proxy and checksum database serve and authenticate it. For an
unreleased change, select an exact **merged commit containing the canonical
module path** with `go get github.com/OpenTLV/opentlv-go@<commit>`; Go records
its pseudo-version.
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

The supported source-build baseline is exactly Core `v0.8.0`, commit
`345c77acb29266036df5f27d04ce315bb61853cc`, recorded with its release version in
`core-source.json`. The build helper requires that version field; the release
workflow additionally requires the Core tag to resolve to that commit and to be
a published, non-prerelease Core release. No wider Core version range or stable
native ABI is claimed.

The pin moved from the migration baseline
`cb3caed5b09706ce11ecfae839397fd4f99d0025` for the first release (#31). Between
both commits Core changed no public header. Its native changes are internal
Query program and executor workspace layout accessors with unchanged image
format, public API and ABI (OpenTLV/opentlv#707, OpenTLV/opentlv#709); it also
removed the WASM build option and the migrated Go, WASM and JavaScript sources.
The Query conformance runner and corpus used by Full are unchanged. The reviewed
generic API inventory is unchanged (166 referenced, zero missing and 131
excluded symbols), and the generated diagnostic snapshots are identical. Reader, Writer,
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

The binding intentionally uses source-built Core rather than a verified release
binary. The local ZIP has synthetic version/time metadata solely for testing;
it is not an official release artifact. The published module is the Git tag,
served by the public Go module proxy; the release workflow checks that the proxy
and checksum database serve exactly the tagged tree.

## GitHub release

The manual `Go Release` workflow (`.github/workflows/go-release.yml`) accepts an
unused `vMAJOR.MINOR.PATCH` tag matching `BindingVersion`, `docs-examples.json`
and `ci/release-notes.md`. Changes stay under `Unreleased` during development;
close the changelog under the matching version without the tag's `v` prefix and
the release date before dispatching the release (for example,
`## [0.1.0] - 2026-10-28`). The versioned release notes are the publication text
and are included in the hashed CI definitions.

1. Review and merge the release preparation and the closed changelog. Wait for
   successful Go Main CI on that exact commit. Dispatch `Go Release` on `main`
   with `tag: v0.1.0`.
2. The workflow rejects a foreign, dirty or non-main checkout, a changed main
   tip, an existing tag or release, wrong versions, an open changelog, missing
   Main evidence or a Core lock that is not the published Core release. It then
   runs [Full CI](development/ci.md#workflows-and-coverage): all six native
   configurations, each with the external-consumer package check, and the
   attempt-matched Go quality gate.
3. Assembly revalidates every Full record against the current commit, run,
   attempt, Core lock and CI definition hashes, then attests three assets:
   `release-manifest.json` (request, Main run, Core release and all Full
   records), `release-notes.md` and `SHA256SUMS`. There are no binaries.
4. Publication rechecks the request, Full evidence, checksums, attestations and
   unused tag status, then creates the tag and a draft release. Only after the
   uploaded asset names, sizes, SHA-256 digests, notes and tag commit match does
   it publish the draft and verify the public result. `go-published-<attempt>`
   records the URL, source, Core and asset identities.
5. The final job downloads `github.com/OpenTLV/opentlv-go@<tag>` through
   `proxy.golang.org` with `sum.golang.org` verification in a fresh module
   cache. It requires the module path, version and origin commit to match and
   the module files to equal the tagged tree byte for byte, retrying while the
   proxy fetches the new tag. `go-module-<attempt>` records the `h1:` sums.
6. Record the published URL and exact commit, Main and Release/Full run links
   in [issue #31](https://github.com/OpenTLV/opentlv-go/issues/31) and
   [Core epic #718](https://github.com/OpenTLV/opentlv/issues/718). Close the
   tracking only after publication and module verification.

If a failure occurs after creating a tag or draft, inspect that state before
retrying; the workflow deliberately refuses to overwrite either. Never move or
delete a published tag: the checksum database permanently records its content,
so a correction needs a new version. Earlier attempts' artifacts cannot satisfy
a new attempt. A new main commit or Main run requires a new release attempt.
