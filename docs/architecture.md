# Architecture

The CLI orchestrates specialized PawnKit libraries and commands. It does not
contain a parser, formatter, linter, project loader, or dependency installer.

```text
pawn check
  -> pawn-project
  -> pawnfmt
  -> pawnlint

pawn fmt, pawn lint, pawn test
  -> installed standalone tool

pawn restore
  -> pawn-project/dependency

pawn install
  -> pawn-project/dependency
  -> GitHub API or local Git client
```

The workflow package selects tasks, resolves their dependencies, and keeps
results in a stable order. Commands handle presentation, exit codes, and
cancellation.

Focused commands run the matching tool from the project root. This preserves
its configuration and ignore rules.

Dependency restoration is implemented by `pawn-project`. Build backends
receive resolved RFC 0012 requests from `pawn-project`.
`pawn build` checks the project's verified compiler cache, then `PATH`, then
the checksum-pinned PawnKit compiler index.
`pawn build --compiler` uses the bundled direct compiler backend. External
backends may handle restore, build, and run without reloading the manifest.
Native restore installs locked source commits first, then verifies and commits
all RFC 0021 resources for the selected host target.
Install resolves missing records from restored package manifests. The CLI owns
GitHub API transport, selects the Git transport for other HTTPS hosts, and
replaces lockfiles recoverably. Selection, inspection, validation, and
installation remain in `pawn-project`.

## Contracts

- Exit `0`: success.
- Exit `1`: findings.
- Exit `2`: invalid usage.
- Exit `3`: internal or environmental failure.
- JSON reports include a schema version.
- Underlying tools are named in every task result.
- Discovery and doctor never execute project code.
- Check never performs silent network updates.

External tools declare their protocol version and supported commands through
`capabilities --output json`. Responses are limited to 1 MiB.

Audit findings say whether they are confirmed, inferred, or heuristic. Reports
never print credential values.
