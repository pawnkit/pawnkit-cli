package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"

	projectbackend "github.com/pawnkit/pawn-project/backend"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/profile"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawn-project/toolchain"
	"github.com/pawnkit/pawnkit-cli/pkg/backendrunner"
	"github.com/pawnkit/pawnkit-cli/pkg/capability"
	"github.com/pawnkit/pawnkit-cli/pkg/directbackend"
	"github.com/pawnkit/pawnkit-core/source"
)

const (
	compilerIndexURL = "https://pawnkit.dev/compiler-indexes/pawn-3.10.10-openmp-3.10.11.json"
	compilerIndexSum = "sha256:ef4c1aa64ce3be544e1517ba0a7c3457c5dd4d7b00fbdce1bf7248b93aba5792"
)

type compilerAcquisition struct {
	indexURL      string
	indexChecksum string
	target        string
	downloader    toolchain.Downloader
}

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
	if operation == projectbackend.Build && *executable == "" && *compiler == "" {
		cacheDir, cacheErr := toolchain.DefaultCacheDir()
		if cacheErr != nil {
			cacheDir = ""
		}
		*compiler, err = resolveCompiler(
			ctx, loaded, cacheDir, toolchain.OSCacheFS{}, toolchain.OSPathLookup{},
			compilerAcquisition{
				indexURL:      compilerIndexURL,
				indexChecksum: compilerIndexSum,
				target:        runtime.GOOS + "-" + runtime.GOARCH,
				downloader:    toolchain.HTTPDownloader{},
			},
		)
		if err != nil {
			if errors.Is(err, toolchain.ErrNotFound) {
				_, _ = fmt.Fprintln(stderr, "pawn build: pawncc was not found; pass --compiler or add it to PATH")
			} else {
				_, _ = fmt.Fprintln(stderr, "pawn build: resolving compiler:", err)
			}
			return ExitInternal
		}
	}
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

func resolveCompiler(
	ctx context.Context,
	project *projectmodel.Project,
	cacheDir string,
	cacheFS toolchain.CacheFS,
	lookup toolchain.PathLookup,
	acquisition compilerAcquisition,
) (string, error) {
	coordinate, pinned := project.CompilerCoordinate()
	if pinned && cacheDir != "" {
		info, err := toolchain.NewResolver(cacheFS, cacheDir, nil, nil).Resolve(ctx, toolchain.ResolveOptions{
			Vendor:           coordinate.Vendor,
			Version:          coordinate.Version,
			ExpectedChecksum: coordinate.ExpectedChecksum,
			Offline:          true,
		})
		if err == nil {
			return info.Path, nil
		}
		if !errors.Is(err, toolchain.ErrOffline) {
			return "", err
		}
	}
	local, err := toolchain.FindCompiler(lookup, compilerCandidates(project.Selection().ProfileID)...)
	if err == nil || !errors.Is(err, toolchain.ErrNotFound) || !pinned || cacheDir == "" ||
		acquisition.indexURL == "" || acquisition.downloader == nil {
		return local, err
	}
	indexReader, err := acquisition.downloader.Download(ctx, acquisition.indexURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = indexReader.Close() }()
	index, err := toolchain.LoadIndex(indexReader, acquisition.indexChecksum)
	if err != nil {
		return "", err
	}
	artifact, err := index.Select(coordinate.Vendor, coordinate.Version, acquisition.target)
	if err != nil {
		return "", err
	}
	info, err := toolchain.NewResolver(cacheFS, cacheDir, acquisition.downloader, nil).
		Resolve(ctx, artifact.ResolveOptions())
	if err != nil {
		return "", err
	}
	return info.Path, nil
}

func compilerCandidates(profileID string) []string {
	if profileID == "openmp" {
		return []string{"openmp-pawncc", "pawncc"}
	}
	return []string{"pawncc"}
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
