package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawnkit-core/source"
)

type projectReport struct {
	SchemaVersion int    `json:"schemaVersion"`
	Root          string `json:"root"`
	Manifest      string `json:"manifest"`
	Entry         string `json:"entry"`
	Profile       string `json:"profile"`
	Build         string `json:"build"`
	Runtime       string `json:"runtime"`
}

func runProject(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("project", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	output := flags.String("output", "human", "human or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if err := ctx.Err(); err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn project:", err)
		return ExitInternal
	}
	root, err := filepath.Abs(*projectDir)
	if err != nil {
		return ExitInternal
	}
	project, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn project:", err)
		return ExitFindings
	}
	selection := project.Selection()
	report := projectReport{
		SchemaVersion: 1,
		Root:          filepath.FromSlash(project.Root()),
		Manifest:      filepath.FromSlash(project.Workspace().ManifestPath),
		Entry:         filepath.FromSlash(project.Paths().Entry),
		Profile:       selection.ProfileID,
	}
	if selection.Build != nil {
		report.Build = selection.Build.Name
	}
	if selection.Runtime != nil {
		report.Runtime = selection.Runtime.Name
	}
	switch *output {
	case "json":
		if err := writeJSON(stdout, report); err != nil {
			return ExitInternal
		}
	case "human":
		if _, err := fmt.Fprintf(
			stdout, "profile  %s\nbuild    %s\nruntime  %s\nentry    %s\n",
			displaySelection(report.Profile), displaySelection(report.Build),
			displaySelection(report.Runtime), displaySelection(report.Entry),
		); err != nil {
			return ExitInternal
		}
	default:
		return ExitUsage
	}
	return ExitOK
}

func displaySelection(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}
