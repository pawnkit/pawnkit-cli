package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	projectmodel "github.com/pawnkit/pawn-project/project"
	projecttoolchain "github.com/pawnkit/pawn-project/toolchain"
	"github.com/pawnkit/pawnkit-core/source"
)

type restoreReport struct {
	SchemaVersion int                         `json:"schemaVersion"`
	Dependencies  []dependency.Result         `json:"dependencies"`
	Resources     []dependency.ResourceResult `json:"resources,omitempty"`
}

func runRestore(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	version string,
) int {
	if hasBackendFlag(args) {
		return runBackend(ctx, "restore", args, stdout, stderr, version)
	}

	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	target := flags.String("target", runtime.GOOS+"-"+runtime.GOARCH, "resource target")
	format := flags.String("format", "human", "human or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}

	root, err := filepath.Abs(*projectDir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn restore:", err)
		return ExitInternal
	}
	loaded, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn restore:", err)
		return ExitInternal
	}
	lock := loaded.Lockfile()
	if lock == nil {
		_, _ = fmt.Fprintln(stderr, "pawn restore: pawn.lock is missing or invalid")
		return ExitFindings
	}

	results, err := dependency.NewRestorer(fsx.OS{}, dependency.GitInstaller{}).
		Restore(ctx, loaded.Root(), lock)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn restore:", err)
		return ExitInternal
	}
	resourceResults, err := restoreLockedResources(
		ctx,
		loaded.Root(),
		*target,
		lock,
		projecttoolchain.HTTPDownloader{},
	)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn restore:", err)
		return ExitInternal
	}

	switch *format {
	case "json":
		if err := writeJSON(stdout, restoreReport{
			SchemaVersion: 1,
			Dependencies:  results,
			Resources:     resourceResults,
		}); err != nil {
			return ExitInternal
		}
	case "human":
		for _, result := range results {
			if _, err := fmt.Fprintf(stdout, "%-10s %s\n", result.Status, result.Name); err != nil {
				return ExitInternal
			}
		}
		for _, result := range resourceResults {
			if _, err := fmt.Fprintf(
				stdout,
				"%-10s %s/%s (%s, %d files)\n",
				dependency.StatusInstalled,
				result.Package,
				result.Resource,
				result.Target,
				result.Files,
			); err != nil {
				return ExitInternal
			}
		}
	default:
		_, _ = fmt.Fprintln(stderr, "pawn restore: --format must be human or json")
		return ExitUsage
	}

	return ExitOK
}

func restoreLockedResources(
	ctx context.Context,
	root, target string,
	lock *lockfile.Lock,
	downloader dependency.ResourceDownloader,
) ([]dependency.ResourceResult, error) {
	if len(lock.Resources) == 0 {
		return nil, nil
	}
	return dependency.NewResourceRestorer(dependency.OSResourceFS{}, downloader).
		Restore(ctx, root, target, lock)
}

func hasBackendFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--backend" || strings.HasPrefix(arg, "--backend=") {
			return true
		}
	}
	return false
}
