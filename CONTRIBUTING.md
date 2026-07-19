# Contributing

PawnKit is maintained by volunteers, so reviews may take a little time.

The `pawn` command coordinates other PawnKit tools. Small command, reporting,
and integration fixes are welcome.

Run these checks before opening a pull request:

```sh
task check
```

Do not duplicate formatter, linter, test, or project logic in the CLI. Keep
machine-readable output versioned and add a fixture when its shape changes.
