# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- Integrate Go-owned manuals and examples with the documentation portal through independent source selection, a versioned example catalog and verified Main documentation notifications. (#6)
- Normalize the standalone Go layout with public consumer tests beside the root package, enforced private cgo boundaries, and documented repository ownership while preserving public imports and API behavior. (#5)
- Establish the canonical `github.com/OpenTLV/opentlv-go` module, independent binding version and tag policy, and external module ZIP consumption checks while preserving Core-reporting `Version()` and MIT attribution. (#4)
- Add standalone Go PR/Fast, Main and Full CI with native platform/linking checks, preserved race/cgo and Query coverage, isolated build outputs, per-check evidence and a fail-closed quality gate. (#3)
- Transfer the baseline Go implementation, tests, examples and guides with per-file provenance, and add a standalone build helper using pinned Core sources or an explicit local override with effective dependency evidence. (#1, #2; OpenTLV/opentlv-workspace#40)
- Record the original Core source baseline, migration ownership and MIT attribution before importing the Go binding. (OpenTLV/opentlv#685)
