// Package cli implements the pawn command.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	ExitOK       = 0
	ExitFindings = 1
	ExitUsage    = 2
	ExitInternal = 3
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, version string) int {
	if len(args) == 0 {
		if err := writeHelp(stdout); err != nil {
			return ExitInternal
		}
		return ExitOK
	}
	switch args[0] {
	case "help", "-h", "--help":
		if err := writeHelp(stdout); err != nil {
			return ExitInternal
		}
		return ExitOK
	case "version", "--version":
		if _, err := fmt.Fprintln(stdout, "pawn", version); err != nil {
			return ExitInternal
		}
		return ExitOK
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout, stderr)
	case "audit":
		return runAudit(ctx, args[1:], stdout, stderr)
	case "init":
		return runInit(ctx, args[1:], stdout, stderr)
	case "restore":
		return runBackend(ctx, "restore", args[1:], stdout, stderr, version)
	case "build":
		return runBackend(ctx, "build", args[1:], stdout, stderr, version)
	case "run":
		return runBackend(ctx, "run", args[1:], stdout, stderr, version)
	default:
		_, _ = fmt.Fprintf(stderr, "pawn: unknown command %q\n", args[0])
		return ExitUsage
	}
}

func writeHelp(output io.Writer) error {
	_, err := fmt.Fprint(output, `pawn - unified PawnKit workflow

Usage:
  pawn check [--project DIR] [--only TASKS] [--skip TASKS] [--fail-fast] [--output human|json|sarif]
  pawn doctor [--project DIR] [--output human|json]
  pawn audit [--project DIR] [--offline] [--output human|json]
  pawn init [--project DIR] [--entry FILE] [--target openmp|samp] [--include DIR] [--dry-run]
  pawn restore [--project DIR] --backend EXECUTABLE [--format human|json]
  pawn build [--project DIR] (--compiler PATH | --backend EXECUTABLE) [--artifact FILE] [--format human|json]
  pawn run [--project DIR] --backend EXECUTABLE [--artifact FILE] [--format human|json]
  pawn version
  pawn help

Available check tasks: project, format, lint, plus negotiated build and test adapters
`)
	return err
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func nameSet(value string) map[string]bool {
	result := map[string]bool{}
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result[item] = true
		}
	}
	return result
}
