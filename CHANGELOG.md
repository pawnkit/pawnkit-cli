# Changelog

## 1.34.31 - 2026-08-03

- Use PawnKit Actions v1.8.73 in CI.

## 1.34.30 - 2026-08-03

- Use PawnKit Actions v1.8.72 in CI.

## 1.34.29 - 2026-08-03

- Use pawnlint v1.8.53's cached shared diagnostic mappings.

## 1.34.28 - 2026-08-03

- Use pawnlint v1.8.52 for loop-scoped performance checks.

## 1.34.27 - 2026-08-03

- Use pawnlint v1.8.51 for shared loop indexes in performance checks.

## 1.34.26 - 2026-08-03

- Use pawnfmt v1.4.10 for the project workflow.

## 1.34.25 - 2026-08-03

- Use pawnlint v1.8.42 for cached project analysis.

## 1.34.24 - 2026-08-03

- Republish the CLI artifacts with complete release provenance.

## 1.34.23 - 2026-08-03

- Use pawnlint v1.8.41 for target-aware cached control-flow models.

## 1.34.22 - 2026-08-03

- Use pawnlint v1.8.39 for cached editor project models.

## 1.34.21 - 2026-08-03

- Use pawnlint 1.8.38 and pawn-analysis 0.30.16.

## 1.34.20 - 2026-08-03

- Use pawnlint 1.8.37 and pawn-analysis 0.30.15 for lower-overhead lint runs.

## 1.34.19 - 2026-08-03

- Use pawnfmt 1.4.9, pawnlint 1.8.36, and pawn-analysis 0.30.14.

## 1.34.18 - 2026-08-03

- Use pawnlint 1.8.35 and the retained-token analysis path.

## 1.34.17 - 2026-08-02

- Use pawnlint 1.8.34.

## 1.34.16 - 2026-08-02

- Use pawnfmt 1.4.8, pawnlint 1.8.33, and pawnserver 0.7.2.

## 1.34.15 - 2026-08-02

- Use pawnlint 1.8.32.

## 1.34.14 - 2026-08-02

- Use pawnlint 1.8.31.

## 1.34.13 - 2026-08-02

- Use pawnlint 1.8.30.

## 1.34.12 - 2026-08-02

- Use pawn-analysis 0.30.11 through the shared toolchain.

## 1.34.11 - 2026-08-02

- Use pawnlint 1.8.29.

## 1.34.10 - 2026-08-02

- Use pawnlint 1.8.29 and pawn-analysis 0.30.10.

## 1.34.9 - 2026-08-02

- Use pawnlint 1.8.28.

## 1.34.8 - 2026-08-02

- Use pawnfmt 1.4.7 and pawn-project 0.34.2.

## 1.34.7 - 2026-08-02

- Use pawnfmt 1.4.6.

## 1.34.6 - 2026-08-02

- Use pawnlint 1.8.27, pawn-analysis 0.30.9, and pawn-parser 1.5.8.

## 1.34.5 - 2026-08-02

- Use pawnlint 1.8.26 for faster recursive-call checks.

## 1.34.4 - 2026-08-02

- Use pawnlint 1.8.25 for faster statement-macro checks.

## 1.34.3 - 2026-08-01

- Use pawnlint 1.8.24 and pawn-parser 1.5.7.

## 1.34.2 - 2026-08-01

- Use pawnlint 1.8.23 and pawn-analysis 0.30.7 for PawnPlus tag aliases.

## 1.34.1 - 2026-08-01

- Use pawnlint 1.8.11 and pawn-analysis 0.30.3 for trivia-only edit reuse.

## 1.34.0 - 2026-08-01

- Update pawnlint to v1.8.10 and pawn-analysis to v0.30.2.

## 1.33.0 - 2026-08-01

- Stage project scriptfiles for isolated native server runs.

## 1.32.0 - 2026-08-01

- Stage verified plugins, components, and filterscripts for native open.mp runs.

## 1.31.0 - 2026-07-31

- Align `pawn check` with pawnfmt v1.4.5 and pawnlint v1.8.9.

## 1.30.1 - 2026-07-31

- Refresh clean dependency checkouts after lock updates.

## 1.30.0 - 2026-07-31

- Install guarded package dependency cycles deterministically.

## 1.29.1 - 2026-07-31

- Accept branch dependency references containing slashes.

## 1.29.0 - 2026-07-31

- Recover stale plugin paths when an archive has one matching filename.

## 1.28.1 - 2026-07-31

- Apply same-package dependency constraint overrides correctly.

## 1.28.0 - 2026-07-31

- Apply root dependency overrides during install and lock generation.

## 1.27.0 - 2026-07-31

- Keep direct project constraints authoritative over transitive requests.

## 1.26.0 - 2026-07-31

- Prefer direct dependency pins over unqualified transitive references.

## 1.25.0 - 2026-07-31

- Install leaf dependency repositories that have no package manifest.

## 1.24.0 - 2026-07-31

- Install credential-free HTTPS dependencies from Git hosts outside GitHub.

## 1.23.0 - 2026-07-31

- Resolve sampctl-compatible dependency ranges to deterministic GitHub tags.

## 1.22.0 - 2026-07-31

- Reuse verified dependency checkouts across projects.

## 1.21.0 - 2026-07-31

- Use canonical GitHub repository identities when resolving moved packages.

## 1.20.0 - 2026-07-31

- Add `pawn install --update` to refresh locked dependency revisions.

## 1.19.0 - 2026-07-31

- Reconcile changed manifest dependencies with existing lockfiles.
- Keep matching locked installs independent of provider APIs.

## 1.18.0 - 2026-07-31

- Create a sampctl-compatible lock from a manifest-only GitHub project.
- Reuse complete locked resource records without release lookups.
- Accept `GH_TOKEN` for authenticated GitHub resolution.

## 1.17.0 - 2026-07-31

- Reuse verified resource downloads while resolving and installing them.

## 1.16.2 - 2026-07-31

- Install resources from ordinary sampctl dependencies with broad asset patterns.

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
