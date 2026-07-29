package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/pawnkit/pawnkit-cli/pkg/toolchain"
)

func runToolchain(ctx context.Context, args []string, stdout, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet("toolchain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	releaseSet := flags.String("release-set", os.Getenv("PAWN_RELEASE_SET"), "tested release-set file")
	output := flags.String("output", "human", "human or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if *releaseSet == "" {
		_, _ = fmt.Fprintln(stderr, "pawn toolchain: pass --release-set or set PAWN_RELEASE_SET")
		return ExitUsage
	}
	set, err := toolchain.Load(*releaseSet)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn toolchain:", err)
		return ExitUsage
	}
	report := toolchain.Inspect(ctx, set, version, toolchain.ExecRunner{})
	switch *output {
	case "json":
		if err := writeJSON(stdout, report); err != nil {
			return ExitInternal
		}
	case "human":
		if _, err := fmt.Fprintln(stdout, "Tested release set:", report.ReleaseSet); err != nil {
			return ExitInternal
		}
		for _, tool := range report.Tools {
			actual := tool.Actual
			if actual == "" {
				actual = "-"
			}
			if _, err := fmt.Fprintf(stdout, "%-10s %-11s installed %-12s expected %s\n", tool.Name, tool.Status, actual, tool.Expected); err != nil {
				return ExitInternal
			}
		}
	default:
		return ExitUsage
	}
	if !report.Compatible() {
		return ExitFindings
	}
	return ExitOK
}
