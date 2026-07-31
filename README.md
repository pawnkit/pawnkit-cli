# pawnkit-cli

[![Maturity: preview](https://img.shields.io/badge/maturity-preview-blue)](.pawnkit/support.json)

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
pawn fmt [--project DIR] [--check]
pawn lint [--project DIR]
pawn test [--project DIR]
pawn doctor [--project DIR] [--output FORMAT]
pawn project [--project DIR] [--output FORMAT]
pawn audit [--project DIR] [--output FORMAT]
pawn init [--project DIR] [--entry FILE] [--target openmp|samp] [--include DIR]
pawn restore [--project DIR]
pawn build [--project DIR] [--compiler PATH | --backend EXECUTABLE]
pawn run [--project DIR] [--compiler PATH | --backend EXECUTABLE]
pawn runtime install [--version VERSION] [--target OS-ARCH]
pawn version
```

Check the installed tools against a pinned tested release set:

```sh
pawn toolchain --release-set toolchain.json
```

`PAWN_RELEASE_SET` can provide the same file to `pawn toolchain` and
`pawn doctor`. Overrides remain usable, but PawnKit reports them as untested.

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

The focused commands use the installed standalone tools:

```sh
pawn fmt --check
pawn fmt
pawn lint
pawn test
```

Put `pawnfmt`, `pawnlint`, and `pawntest` on `PATH`. Each command runs from the
project directory, so local tool configuration still applies.

Use `--build-tool` or `--test-tool` to add an external backend. The executable
must support PawnKit capability negotiation. `pawn check` never downloads or
updates tools.

`pawn build` uses the project's pinned cached compiler when available, then
checks `PATH`. If neither has the compiler, it installs the exact reviewed
artifact for the current platform. Pass `--compiler` to select another binary.
`pawn restore` installs dependency sources at the commits recorded in
`pawn.lock`. When the lock contains RFC 0021 resource records, it also verifies
and installs the exact plugin, component, filterscript, and include assets for
the current host. Pass `--target OS-ARCH` to select another recorded target, or
`--backend` to use an RFC 0012 backend.

`pawn run` builds an open.mp project, installs its verified server runtime when
needed, and starts it in an isolated session. The runtime cache is not modified
while the server runs. Supported sampctl runtime settings are translated to
open.mp configuration. Projects with plugins, filterscripts, extra gamemodes,
or unmapped legacy settings still need an RFC 0012 backend.

Pass `--backend` to use an RFC 0012 backend executable. Requests contain the
selected profile, paths, defines, and compiler rather than asking the backend
to rediscover the project.

Install the reviewed open.mp server runtime for the current host:

```sh
pawn runtime install
```

The command verifies the pinned index, archive, and server executable before
placing it in PawnKit's runtime cache. Linux and Windows hosts are supported.

`pawn doctor` looks for common project problems such as missing entry files,
unpinned dependencies, path-case collisions, and credentials in configuration
files. It reports possible fixes but does not change the project.

`pawn project` prints the resolved profile, build, runtime, and entry point.
Use JSON output when an editor or script needs that selection.

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
