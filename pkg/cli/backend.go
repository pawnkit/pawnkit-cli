package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	projectbackend "github.com/pawnkit/pawn-project/backend"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/profile"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawnkit-cli/pkg/backendrunner"
	"github.com/pawnkit/pawnkit-cli/pkg/capability"
	"github.com/pawnkit/pawnkit-cli/pkg/directbackend"
	"github.com/pawnkit/pawnkit-core/source"
)

func runBackend(ctx context.Context, operation projectbackend.Operation, args []string, stdout, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet(string(operation), flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	profile := flags.String("profile", "", "project profile")
	buildName := flags.String("build", "", "named build")
	runtimeName := flags.String("runtime", "", "named runtime")
	executable := flags.String("backend", "", "RFC 0012 backend executable")
	compiler := flags.String("compiler", "", "absolute or project-relative pawncc path")
	artifact := flags.String("artifact", "", "absolute or project-relative output path")
	format := flags.String("format", "human", "human or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if operation != projectbackend.Build && *compiler != "" {
		_, _ = fmt.Fprintln(stderr, "pawn", operation, ": --compiler is only valid for build")
		return ExitUsage
	}
	if operation != projectbackend.Build && *executable == "" {
		_, _ = fmt.Fprintln(stderr, "pawn", operation, ": --backend is required")
		return ExitUsage
	}
	if operation == projectbackend.Build && *executable == "" && *compiler == "" {
		_, _ = fmt.Fprintln(stderr, "pawn build: --compiler or --backend is required")
		return ExitUsage
	}

	root, err := filepath.Abs(*projectDir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn", operation, ":", err)
		return ExitInternal
	}
	loaded, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{
		Profile: profileOptions(*profile, *buildName, *runtimeName),
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn", operation, ":", err)
		return ExitInternal
	}
	if len(loaded.Diagnostics()) != 0 {
		_, _ = fmt.Fprintf(stderr, "pawn %s: project has %d diagnostic(s)\n", operation, len(loaded.Diagnostics()))
		return ExitFindings
	}

	outputPath := resolvedPath(root, *artifact)
	var compilerInfo *projectbackend.Compiler
	if *compiler != "" {
		compilerPath := resolvedPath(root, *compiler)
		compilerInfo = &projectbackend.Compiler{Path: compilerPath}
	}
	request, err := loaded.BackendRequest(operation, projectbackend.RequestOptions{
		Compiler: compilerInfo,
		Output:   outputPath,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn", operation, ":", err)
		return ExitFindings
	}

	var result projectbackend.Result
	if *executable != "" {
		result, err = backendrunner.Run(ctx, capability.OSRunner{}, *executable, request)
	} else {
		result, err = directbackend.Execute(ctx, request, version)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn", operation, ":", err)
		return ExitInternal
	}
	switch *format {
	case "json":
		if err := writeJSON(stdout, result); err != nil {
			return ExitInternal
		}
	case "human":
		if err := writeBackendResult(stdout, result); err != nil {
			return ExitInternal
		}
	default:
		_, _ = fmt.Fprintln(stderr, "pawn", operation, ": --format must be human or json")
		return ExitUsage
	}
	if result.Status == "passed" {
		return ExitOK
	}
	return ExitFindings
}

func profileOptions(profileID, buildName, runtimeName string) profile.Options {
	return profile.Options{ProfileOverride: profileID, BuildName: buildName, RuntimeName: runtimeName}
}

func resolvedPath(root, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return filepath.Clean(path)
}

func writeBackendResult(output io.Writer, result projectbackend.Result) error {
	_, err := fmt.Fprintf(output, "%s %s %s\n", result.Backend.Name, result.Backend.Version, result.Status)
	if err != nil {
		return err
	}
	for _, artifact := range result.Artifacts {
		if _, err := fmt.Fprintf(output, "  %s (%d bytes)\n", artifact.Path, artifact.Size); err != nil {
			return err
		}
	}
	if result.Process != nil {
		if result.Process.Stdout != "" {
			if _, err := fmt.Fprint(output, result.Process.Stdout); err != nil {
				return err
			}
		}
		if result.Process.Stderr != "" {
			if _, err := fmt.Fprint(output, result.Process.Stderr); err != nil {
				return err
			}
		}
	}
	return nil
}
