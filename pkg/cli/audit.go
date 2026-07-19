package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawnkit-cli/pkg/audit"
	"github.com/pawnkit/pawnkit-cli/pkg/sbom"
	"github.com/pawnkit/pawnkit-core/source"
)

func runAudit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	output := flags.String("output", "human", "human or json")
	offline := flags.Bool("offline", true, "disable network metadata")
	sbomFormat := flags.String("sbom", "", "cyclonedx or spdx")
	sbomOutput := flags.String("sbom-output", "", "SBOM output path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if !*offline {
		_, _ = fmt.Fprintln(stderr, "pawn audit: online metadata provider is not configured; use --offline")
		return ExitUsage
	}
	if err := ctx.Err(); err != nil {
		return ExitInternal
	}
	root, err := filepath.Abs(*projectDir)
	if err != nil {
		return ExitInternal
	}
	project, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn audit:", err)
		return ExitFindings
	}
	report := audit.Inspect(project, audit.Options{Offline: *offline, Source: "pawn.lock"})
	if (*sbomFormat == "") != (*sbomOutput == "") {
		_, _ = fmt.Fprintln(stderr, "pawn audit: --sbom and --sbom-output must be used together")
		return ExitUsage
	}
	if *sbomFormat != "" {
		document, err := sbom.Generate(*sbomFormat, project.Lockfile())
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "pawn audit:", err)
			return ExitFindings
		}
		if err := writeJSONFile(*sbomOutput, document); err != nil {
			_, _ = fmt.Fprintln(stderr, "pawn audit:", err)
			return ExitInternal
		}
	}
	switch *output {
	case "json":
		if err := writeJSON(stdout, report); err != nil {
			return ExitInternal
		}
	case "human":
		if _, err := fmt.Fprintln(stdout, report.Disclaimer); err != nil {
			return ExitInternal
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

func writeJSONFile(destination string, value any) error {
	directory := filepath.Dir(destination)
	temporary, err := os.CreateTemp(directory, ".pawn-sbom-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	if err := writeJSON(temporary, value); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	keep = true
	return nil
}
