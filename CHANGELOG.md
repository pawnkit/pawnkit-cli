# Changelog

Notable changes are recorded here.

## 1.2.1 - 2026-07-24

### Changed

- Updated the lint dependency to v1.1.5, pulling in an analysis release that
  removes duplicate work and a quadratic scan from large-file analysis.

## 1.2.0 - 2026-07-23

### Added

- Added `pawn restore`, `pawn build`, and `pawn run` with resolved build-backend
  requests.
- Added direct `pawncc` builds with bounded compiler output and artifact hashes.

## 1.1.3 - 2026-07-23

### Changed

- Used `pawn-project` to create and encode `pawn.json`.

## 1.1.2 - 2026-07-23

### Changed

- Updated project, formatter, and linter dependencies.

## 1.1.1 - 2026-07-23

### Fixed

- Stopped suggesting an update command that the CLI does not provide.

## 1.1.0 - 2026-07-23

### Added

- Added `pawn init` for creating a checked PawnKit project manifest.

## 1.0.3 - 2026-07-23

### Fixed

- Updated formatting and linting dependencies.

## 1.0.2 - 2026-07-23

### Fixed

- Kept project health checks consistent on Windows.

## 1.0.1 - 2026-07-22

### Fixed

- Updated project discovery, formatting, and linting dependencies.

## 1.0.0 - 2026-07-19

### Added

- `pawn check` for project, formatting, lint, build, and test tasks.
- `pawn doctor` for local project health checks.
- `pawn audit` with CycloneDX and SPDX output.
- Human, JSON, and SARIF reports.
- GoReleaser archives, checksums, SBOMs, and build provenance.
