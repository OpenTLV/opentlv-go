# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- Add standalone Go PR/Fast, Main and Full CI with native platform/linking checks, preserved race/cgo and Query coverage, isolated build outputs, per-check evidence and a fail-closed quality gate. (#3)
- Transfer the baseline Go implementation, tests, examples and guides with per-file provenance, and add a standalone build helper using pinned Core sources or an explicit local override with effective dependency evidence. (#1, #2; OpenTLV/opentlv-workspace#40)
- Record the original Core source baseline, migration ownership and MIT attribution before importing the Go binding. (OpenTLV/opentlv#685)
