package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	projectmodel "github.com/pawnkit/pawn-project/project"
	projecttoolchain "github.com/pawnkit/pawn-project/toolchain"
	"github.com/pawnkit/pawnkit-core/source"
)

type installServices struct {
	sourceInstaller dependency.Installer
	downloader      dependency.ResourceDownloader
	provider        dependency.ReleaseProvider
	writeLock       func(string, []byte) error
}

func runInstall(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
) int {
	downloader := projecttoolchain.HTTPDownloader{}
	return runInstallWith(ctx, args, stdout, stderr, installServices{
		sourceInstaller: dependency.GitInstaller{},
		downloader:      downloader,
		provider: githubReleaseProvider{
			token: os.Getenv("GITHUB_TOKEN"),
		},
		writeLock: replaceLockfile,
	})
}

func runInstallWith(
	ctx context.Context,
	args []string,
	stdout, stderr io.Writer,
	services installServices,
) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	target := flags.String("target", runtime.GOOS+"-"+runtime.GOARCH, "resource target")
	runtimeVersion := flags.String("runtime-version", "", "resource runtime version")
	format := flags.String("format", "human", "human or json")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if *format != "human" && *format != "json" {
		_, _ = fmt.Fprintln(stderr, "pawn install: --format must be human or json")
		return ExitUsage
	}

	root, err := filepath.Abs(*projectDir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}
	loaded, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}
	lock := loaded.Lockfile()
	if lock == nil {
		_, _ = fmt.Fprintln(stderr, "pawn install: pawn.lock is missing or invalid")
		return ExitFindings
	}
	dependencies, err := dependency.NewRestorer(fsx.OS{}, services.sourceInstaller).
		Restore(ctx, loaded.Root(), lock)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}
	resources, err := dependency.NewResourceResolver(
		fsx.OS{},
		services.downloader,
		services.provider,
	).Resolve(ctx, loaded.Root(), *target, *runtimeVersion, lock)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}

	lockContent, err := os.ReadFile(lock.SourcePath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}
	updated, err := lockfile.MarshalSampctlResources(lockContent, lock, resources)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}
	if err := services.writeLock(lock.SourcePath, updated); err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}

	updatedLock := *lock
	updatedLock.Resources = resources
	resourceResults, err := dependency.NewResourceRestorer(
		dependency.OSResourceFS{},
		services.downloader,
	).Restore(ctx, loaded.Root(), *target, &updatedLock)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn install:", err)
		return ExitInternal
	}
	return writeInstallResult(stdout, *format, dependencies, resourceResults)
}

func writeInstallResult(
	output io.Writer,
	format string,
	dependencies []dependency.Result,
	resources []dependency.ResourceResult,
) int {
	if format == "json" {
		if err := writeJSON(output, restoreReport{
			SchemaVersion: 1,
			Dependencies:  dependencies,
			Resources:     resources,
		}); err != nil {
			return ExitInternal
		}
		return ExitOK
	}
	for _, result := range dependencies {
		if _, err := fmt.Fprintf(output, "%-10s %s\n", result.Status, result.Name); err != nil {
			return ExitInternal
		}
	}
	for _, result := range resources {
		if _, err := fmt.Fprintf(
			output,
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
	return ExitOK
}

func replaceLockfile(path string, content []byte) (err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("checking lockfile: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("lockfile is not a regular file")
	}
	directory := filepath.Dir(path)
	staged, err := os.CreateTemp(directory, ".pawn-lock-*")
	if err != nil {
		return fmt.Errorf("creating staged lockfile: %w", err)
	}
	stagedPath := staged.Name()
	defer func() {
		_ = staged.Close()
		_ = os.Remove(stagedPath)
	}()
	if err := staged.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("setting staged lockfile permissions: %w", err)
	}
	if _, err := staged.Write(content); err != nil {
		return fmt.Errorf("writing staged lockfile: %w", err)
	}
	if err := staged.Sync(); err != nil {
		return fmt.Errorf("syncing staged lockfile: %w", err)
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("closing staged lockfile: %w", err)
	}

	backup, err := os.CreateTemp(directory, ".pawn-lock-backup-*")
	if err != nil {
		return fmt.Errorf("creating lockfile backup path: %w", err)
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		return fmt.Errorf("closing lockfile backup path: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("preparing lockfile backup path: %w", err)
	}
	defer func() { _ = os.Remove(backupPath) }()

	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("preserving lockfile: %w", err)
	}
	if err := os.Rename(stagedPath, path); err != nil {
		if rollbackErr := os.Rename(backupPath, path); rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("installing lockfile: %w", err),
				fmt.Errorf("restoring lockfile: %w", rollbackErr),
			)
		}
		return fmt.Errorf("installing lockfile: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("removing lockfile backup: %w", err)
	}
	return nil
}
