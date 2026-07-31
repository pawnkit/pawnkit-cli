# Changelog

## 1.16.1 - 2026-07-31

- Resolve release resources from ordinary sampctl dependency entries.

## 1.16.0 - 2026-07-31

- Add `pawn install` for resolving, locking, and installing package resources.
- Support authenticated GitHub release lookups with rate-limit guidance.

## 1.15.0 - 2026-07-31

- Restore checksum-pinned package resources for an exact host target.
- Report restored resources in human and JSON output.

## 1.14.0 - 2026-07-31

- Apply supported sampctl runtime settings when starting open.mp.
- Reject runtime settings that native sessions cannot yet honour.
- Stop the native server when the command receives an interrupt.

## 1.13.0 - 2026-07-31

- Build and run open.mp projects with the verified native server runtime.
- Keep native run files outside the verified runtime cache.

## 1.12.0 - 2026-07-30

- Add `pawn runtime install` for verified open.mp server archives.

## 1.11.1 - 2026-07-30

- Build nested sampctl projects whose entry or output remains inside a
  containing Pawn workspace.
- Apply top-level sampctl build defaults to named builds.

## 1.11.0 - 2026-07-30

- Install the Pawn 3.10.8 compiler pinned by older sampctl projects.
- Load shared compiler libraries from reviewed archive layouts.

## 1.10.1 - 2026-07-30

- Fix the managed compiler test to use PawnKit's canonical path format on
  Windows.

## 1.10.0 - 2026-07-30

- Install the project's exact compiler from the reviewed PawnKit index when it
  is not cached or available on `PATH`.

## 1.9.0 - 2026-07-30

- Use the compiler pinned by the project when it is available in the verified
  toolchain cache.

## 1.8.2 - 2026-07-30

- Make tool command tests independent of Windows shell argument handling.

## 1.8.1 - 2026-07-30

- Use pawn-project v0.3.10 so downloaded compilers retain execute permission.

## 1.8.0 - 2026-07-30

- Use a local Pawn compiler from `PATH` when `pawn build` has no explicit backend.

## 1.7.2 - 2026-07-30

- Read sampctl resource manifests through pawn-project v0.3.8.

## 1.7.1 - 2026-07-30

- Restore every sampctl dependency scheme and verify locked source integrity.

## 1.7.0 - 2026-07-30

- Restore locked source and include dependencies through pawn-project.

## 1.6.3 - 2026-07-30

- Read sampctl 1.14 lockfiles through pawn-project v0.3.4.

## 1.6.2 - 2026-07-30

- Fix the Windows tool-command test fixture to preserve separate arguments.

## 1.6.1 - 2026-07-30

- Ignore downloaded dependencies and tool caches when `pawn doctor` scans for
  secrets and path collisions.

## 1.6.0 - 2026-07-30

- Add `pawn fmt`, `pawn lint`, and `pawn test` as focused entry points for the
  installed tools.

## 1.5.1 - 2026-07-29

- Read tested release sets through schema v3.

## 1.5.0 - 2026-07-29

- Add `pawn toolchain` and optional doctor checks for pinned tested release
  sets.

Notable changes are recorded here.

## 1.4.2 - 2026-07-29

### Fixed

- Fixed the Windows test for compiler diagnostic file URIs.

## 1.4.1 - 2026-07-29

### Fixed

- Include pawncc errors and warnings in JSON build diagnostics.

## 1.4.0 - 2026-07-29

### Added

- Added `pawn project` for reading the resolved project selection.

## 1.3.1 - 2026-07-25

### Changed

- Added the repository support record with CI validation.

## 1.3.0 - 2026-07-25

### Changed

- Use build-backend schema v2 and diagnostic schema v2.

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
