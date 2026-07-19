package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	clidoctor "github.com/pawnkit/pawnkit-cli/pkg/doctor"
	"github.com/pawnkit/pawnkit-core/source"
)

func runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	output := flags.String("output", "human", "human or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if err := ctx.Err(); err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn doctor:", err)
		return ExitInternal
	}
	root, err := filepath.Abs(*projectDir)
	if err != nil {
		return ExitInternal
	}
	project, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn doctor:", err)
		return ExitFindings
	}
	report := clidoctor.Inspect(project, fsx.OS{})
	switch *output {
	case "json":
		if err := writeJSON(stdout, report); err != nil {
			return ExitInternal
		}
	case "human":
		if len(report.Findings) == 0 {
			if _, err := fmt.Fprintln(stdout, "doctor   passed  no findings"); err != nil {
				return ExitInternal
			}
		}
		for _, finding := range report.Findings {
			if _, err := fmt.Fprintf(stdout, "%-28s %-7s %s\n", finding.ID, finding.Severity, finding.Message); err != nil {
				return ExitInternal
			}
		}
	default:
		return ExitUsage
	}
	if len(report.Findings) != 0 {
		return ExitFindings
	}
	return ExitOK
}
