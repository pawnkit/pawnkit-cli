package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"

	"github.com/pawnkit/pawn-project/toolchain"
	"github.com/pawnkit/pawnserver/runtimeartifact"
)

const (
	runtimeIndexURL = "https://pawnkit.dev/runtime-indexes/openmp-1.5.8.3079-linux-windows.json"
	runtimeIndexSum = "sha256:57502c7a6fd440b49842df3bf1a3a61611fc6584cb7418f30c7dfcedd363a7cc"
)

type runtimeInstaller struct {
	indexURL      string
	indexChecksum string
	downloader    toolchain.Downloader
	cacheDir      func() (string, error)
	install       func(runtimeartifact.Artifact, io.Reader, string) error
}

type runtimeInstallResult struct {
	Vendor     string `json:"vendor"`
	Version    string `json:"version"`
	Target     string `json:"target"`
	Directory  string `json:"directory"`
	Executable string `json:"executable"`
}

type installedRuntime struct {
	artifact   runtimeartifact.Artifact
	directory  string
	executable string
	downloaded bool
}

func runRuntime(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runRuntimeWith(ctx, args, stdout, stderr, runtimeInstaller{
		indexURL:      runtimeIndexURL,
		indexChecksum: runtimeIndexSum,
		downloader:    toolchain.HTTPDownloader{},
		cacheDir:      runtimeartifact.DefaultCacheDir,
		install:       runtimeartifact.Install,
	})
}

func runRuntimeWith(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	installer runtimeInstaller,
) int {
	if len(args) == 0 || args[0] != "install" {
		_, _ = fmt.Fprintln(stderr, "pawn runtime: expected install")
		return ExitUsage
	}
	flags := flag.NewFlagSet("runtime install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	vendor := flags.String("vendor", "openmultiplayer", "runtime vendor")
	version := flags.String("version", "1.5.8.3079", "runtime version")
	target := flags.String("target", runtime.GOOS+"-"+runtime.GOARCH, "host target")
	destination := flags.String("destination", "", "install directory")
	format := flags.String("format", "human", "human or json")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if *format != "human" && *format != "json" {
		_, _ = fmt.Fprintln(stderr, "pawn runtime install: --format must be human or json")
		return ExitUsage
	}

	var err error
	if *destination != "" {
		*destination, err = filepath.Abs(*destination)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "pawn runtime install:", err)
			return ExitInternal
		}
	}
	installed, err := acquireRuntime(ctx, installer, *vendor, *version, *target, *destination, true)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn runtime install:", err)
		return ExitInternal
	}
	result := runtimeInstallResult{
		Vendor:     *vendor,
		Version:    *version,
		Target:     *target,
		Directory:  installed.directory,
		Executable: installed.executable,
	}
	switch *format {
	case "human":
		_, err = fmt.Fprintln(stdout, installed.executable)
	case "json":
		err = writeJSON(stdout, result)
	}
	if err != nil {
		return ExitInternal
	}
	return ExitOK
}

func acquireRuntime(
	ctx context.Context,
	installer runtimeInstaller,
	vendor, version, target, destination string,
	force bool,
) (installedRuntime, error) {
	indexReader, err := installer.downloader.Download(ctx, installer.indexURL)
	if err != nil {
		return installedRuntime{}, err
	}
	defer func() { _ = indexReader.Close() }()
	index, err := runtimeartifact.LoadIndex(indexReader, installer.indexChecksum)
	if err != nil {
		return installedRuntime{}, err
	}
	artifact, err := index.Select(vendor, version, target)
	if err != nil {
		return installedRuntime{}, err
	}
	if destination == "" {
		cacheDir, cacheErr := installer.cacheDir()
		if cacheErr != nil {
			return installedRuntime{}, cacheErr
		}
		destination, err = runtimeartifact.CacheDestination(cacheDir, vendor, version, target)
		if err != nil {
			return installedRuntime{}, err
		}
	}
	if !force {
		if executable, verifyErr := runtimeartifact.VerifyInstalled(artifact, destination); verifyErr == nil {
			return installedRuntime{
				artifact: artifact, directory: destination, executable: executable,
			}, nil
		}
	}
	archive, err := installer.downloader.Download(ctx, artifact.Archive.URL)
	if err != nil {
		return installedRuntime{}, err
	}
	defer func() { _ = archive.Close() }()
	if err := installer.install(artifact, archive, destination); err != nil {
		return installedRuntime{}, err
	}
	executable, err := runtimeartifact.VerifyInstalled(artifact, destination)
	if err != nil {
		return installedRuntime{}, err
	}
	return installedRuntime{
		artifact: artifact, directory: destination, executable: executable, downloaded: true,
	}, nil
}
