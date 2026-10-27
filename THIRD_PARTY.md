# Dependencies and attribution

Original contributions by Marek Cingel retain his copyright under the root
[MIT license](LICENSE). OpenTLV is a GitHub organization, not a separate legal
copyright holder. Other contributors retain their own notices and years;
contributing under MIT does not assign copyright.

## Module and native dependency

The root `go.mod` declares no third-party Go modules. The public Go package,
private C/cgo bridge, tests and examples carry their source notices. The imported
Core baseline and subsequent adaptations are recorded in [MIGRATION.md](MIGRATION.md)
and `migration/source-inventory.json`; this review does not change original
source identities or checksums.

The native dependency is OpenTLV Core, selected at the exact revision in
`core-source.json`. It is fetched into ignored build output or selected explicitly
by the caller, and is not vendored in the Go module. Review that revision's
`LICENSE` and any file-specific notices when distributing Core or a binary linked
with it. Go module metadata alone does not describe a consumer's complete binary
dependency set. Preserve the applicable Go, Core, toolchain/runtime and additional
dependency notices in binary distributions.

`diagnostic_kind.go` and `diagnostic_vocabulary_test.go` are generated from Core's
diagnostic vocabulary by the Go-owned generator. Both retain their MIT and Marek
Cingel headers. Regenerate them through the documented command; do not replace
their ownership through a blanket REUSE annotation.

## Development dependencies

Go, Python, Git, a cgo-compatible C compiler, CMake and Ninja are build tools.
Pre-commit, REUSE, the pre-commit hooks and Markdownlint are contributor checks;
CI also installs actionlint and uses the actions declared in `.github/workflows/`.
These tools and their dependencies keep their upstream licenses. They are not
copied into the Go module or covered by its MIT declaration. Pinned contributor
versions live in `requirements-dev.txt` and `.pre-commit-config.yaml`.

No third-party source tree or binary dependency is currently vendored. Before
adding one, record its origin, exact version, license and redistribution notices;
preserve its source headers and add its license texts as needed. Do not extend
the original-author REUSE block to cover vendor directories or future files.

## Distributed notices

The module ZIP includes `LICENSE`, `LICENSES/MIT.txt`, `REUSE.toml` and this file.
The root license contains the original attribution; `LICENSES/MIT.txt` is the
standard license text used by REUSE. The package check requires these files and
tests their preservation in the candidate ZIP. Organization community defaults
remain linked guidance and are not substitutes for the module's license files.
