package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	projectbackend "github.com/pawnkit/pawn-project/backend"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawn-project/toolchain"
	"github.com/pawnkit/pawnkit-cli/pkg/directbackend"
	"github.com/pawnkit/pawnserver/runtimeartifact"
)

const defaultRuntimeVersion = "1.5.8.3079"

func runNative(
	ctx context.Context,
	loaded *projectmodel.Project,
	root, compilerPath, outputPath string,
	stdout, stderr io.Writer,
	version string,
) int {
	runtimeVersion, port, err := nativeRuntimeSelection(loaded)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn run:", err)
		return ExitFindings
	}
	cacheDir, cacheErr := toolchain.DefaultCacheDir()
	if cacheErr != nil {
		cacheDir = ""
	}
	if compilerPath == "" {
		compilerPath, err = resolveCompiler(
			ctx, loaded, cacheDir, toolchain.OSCacheFS{}, toolchain.OSPathLookup{},
			compilerAcquisition{
				indexURL:      compilerIndexURL,
				indexChecksum: compilerIndexSum,
				target:        runtime.GOOS + "-" + runtime.GOARCH,
				downloader:    toolchain.HTTPDownloader{},
			},
		)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "pawn run: resolving compiler:", err)
			return ExitInternal
		}
	}
	buildRequest, err := loaded.BackendRequest(projectbackend.Build, projectbackend.RequestOptions{
		Compiler: &projectbackend.Compiler{Path: resolvedPath(root, compilerPath)},
		Output:   outputPath,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn run:", err)
		return ExitFindings
	}
	buildResult, err := directbackend.Execute(ctx, buildRequest, version)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn run:", err)
		return ExitInternal
	}
	if err := writeBackendResult(stdout, buildResult); err != nil {
		return ExitInternal
	}
	if buildResult.Status != "passed" {
		return ExitFindings
	}

	installed, err := acquireRuntime(ctx, runtimeInstaller{
		indexURL:      runtimeIndexURL,
		indexChecksum: runtimeIndexSum,
		downloader:    toolchain.HTTPDownloader{},
		cacheDir:      runtimeartifact.DefaultCacheDir,
		install:       runtimeartifact.Install,
	}, "openmultiplayer", runtimeVersion, runtime.GOOS+"-"+runtime.GOARCH, "", false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn run: resolving runtime:", err)
		return ExitInternal
	}
	sessionDir, err := os.MkdirTemp("", "pawn-run-*")
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn run:", err)
		return ExitInternal
	}
	defer func() { _ = os.RemoveAll(sessionDir) }()
	session, err := runtimeartifact.PrepareSession(
		installed.directory,
		buildRequest.Output,
		filepath.Join(sessionDir, "server"),
		runtimeartifact.SessionOptions{Port: port},
	)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn run:", err)
		return ExitInternal
	}
	err = runtimeartifact.RunSession(ctx, installed.executable, session, runtimeartifact.ProcessOptions{
		Stdin: os.Stdin, Stdout: stdout, Stderr: stderr,
	})
	if ctx.Err() != nil {
		return ExitOK
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn run:", err)
		return ExitFindings
	}
	return ExitOK
}

func nativeRuntimeSelection(project *projectmodel.Project) (string, int, error) {
	selection := project.Selection()
	if selection.ProfileID != "openmp" &&
		(selection.Runtime == nil || (selection.Runtime.Mode != "openmp" && selection.Runtime.Mode != "openmp-server")) {
		return "", 0, errors.New("native run currently supports the openmp profile")
	}
	version := defaultRuntimeVersion
	port := 0
	if selection.Runtime != nil {
		if selection.Runtime.Version != "" {
			version = selection.Runtime.Version
		}
		port = selection.Runtime.Port
		if len(selection.Runtime.Plugins) != 0 || len(selection.Runtime.Filterscripts) != 0 {
			return "", 0, errors.New("native run does not yet stage plugins or filterscripts")
		}
	}
	return version, port, nil
}
