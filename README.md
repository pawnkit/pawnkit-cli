# pawnkit-cli

`pawn` gives a Pawn project one command for the checks you run every day. It
loads the project once, then hands formatting, linting, and optional build or
test work to the appropriate PawnKit tool.

The individual tools still work on their own. The CLI coordinates them; it
does not hide or reimplement them.

## Install

```sh
go install github.com/pawnkit/pawnkit-cli/cmd/pawn@latest
```

## Current commands

```text
pawn check [--project DIR] [--only TASKS] [--skip TASKS] [--output FORMAT]
pawn doctor [--project DIR] [--output FORMAT]
pawn audit [--project DIR] [--output FORMAT]
pawn init [--project DIR] [--entry FILE] [--target openmp|samp] [--include DIR]
pawn version
```

Start a project with `pawn init`. It finds a single `.pwn` entry file and
writes `pawn.json` without replacing existing project configuration:

```sh
pawn init --target openmp --include include
```

Pass `--entry` when the project contains more than one possible entry file.
Use `--include` more than once for multiple include directories.

Run `pawn check` from a project directory to validate `pawn.json` and
`pawn.lock`, check formatting, and run pawnlint:

```sh
pawn check
pawn check --only project,lint
pawn check --output sarif > pawn.sarif
```

Use `--build-tool` or `--test-tool` to add an external backend. The executable
must support PawnKit capability negotiation. `pawn check` never downloads or
updates tools.

`pawn doctor` looks for common project problems such as missing entry files,
unpinned dependencies, path-case collisions, and credentials in configuration
files. It reports possible fixes but does not change the project.

`pawn audit` checks the local lockfile and platform artifacts. It can also write
a CycloneDX or SPDX SBOM:

```sh
pawn audit --sbom cyclonedx --sbom-output bom.json
```

Audit runs offline. It can confirm integrity problems in local metadata, but it
cannot tell you whether a dependency has a known vulnerability.

Human output is the default. Check also supports JSON and SARIF; doctor and
audit support JSON. JSON reports use `schemaVersion: 1`.

## Development

```sh
task check
```

This is a community project, and focused fixes are welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

## Licence

[MIT](LICENSE)
