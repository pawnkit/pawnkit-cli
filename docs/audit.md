# Dependency and runtime audit

`pawn audit` works from `pawn.lock`. It checks package and compiler checksums,
source transport, platform selection, and local artifact architecture. Source
credentials are redacted.

The report separates confirmed facts from inferences and heuristics. It runs
offline, so a clean result does not mean a dependency is safe.

CycloneDX and SPDX output is available when the lockfile contains enough package
metadata. Generate either format with `--sbom` and `--sbom-output`.
