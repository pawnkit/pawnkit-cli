# Architecture

The CLI orchestrates specialized PawnKit libraries. It does not contain a
parser, formatter, linter, project loader, or package manager.

```text
pawn check
  -> pawn-project
  -> pawnfmt
  -> pawnlint
```

The workflow package selects tasks, resolves their dependencies, and keeps
results in a stable order. Commands handle presentation, exit codes, and
cancellation.

Build backends receive resolved RFC 0012 requests from `pawn-project`.
`pawn build --compiler` uses the bundled direct compiler backend. External
backends handle restore, build, and run without reloading the manifest.

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
