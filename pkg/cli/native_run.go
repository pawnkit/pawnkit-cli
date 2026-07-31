package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

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
	runtimeVersion, sessionOptions, err := nativeRuntimeSelection(loaded)
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
		sessionOptions,
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

func nativeRuntimeSelection(project *projectmodel.Project) (string, runtimeartifact.SessionOptions, error) {
	selection := project.Selection()
	if selection.ProfileID != "openmp" &&
		(selection.Runtime == nil || (selection.Runtime.Mode != "openmp" && selection.Runtime.Mode != "openmp-server")) {
		return "", runtimeartifact.SessionOptions{}, errors.New("native run currently supports the openmp profile")
	}
	version := defaultRuntimeVersion
	options := runtimeartifact.SessionOptions{}
	if selection.Runtime != nil {
		if selection.Runtime.Version != "" {
			version = selection.Runtime.Version
		}
		if len(selection.Runtime.Plugins) != 0 || len(selection.Runtime.Filterscripts) != 0 ||
			len(selection.Runtime.Gamemodes) != 0 {
			return "", runtimeartifact.SessionOptions{}, errors.New("native run does not yet stage plugins, filterscripts, or extra gamemodes")
		}
		if selection.Runtime.Endpoint != "" || selection.Runtime.NoSign != "" ||
			selection.Runtime.ConnectionCookies != nil || len(selection.Runtime.Extra) != 0 {
			return "", runtimeartifact.SessionOptions{}, errors.New("native run contains runtime settings without an open.mp mapping")
		}
		sleep, err := runtimeSleep(selection.Runtime.Sleep)
		if err != nil {
			return "", runtimeartifact.SessionOptions{}, err
		}
		options = runtimeartifact.SessionOptions{
			Name: selection.Runtime.Hostname, Language: selection.Runtime.Language,
			Website: selection.Runtime.WebURL, Password: selection.Runtime.Password,
			Announce: selection.Runtime.Announce, EnableQuery: selection.Runtime.Query,
			MaxPlayers: selection.Runtime.MaxPlayers, MaxBots: selection.Runtime.MaxNPC,
			Sleep: sleep, Port: selection.Runtime.Port, Bind: selection.Runtime.Bind,
			LANMode: selection.Runtime.LANMode, OnFootRate: selection.Runtime.OnFootRate,
			InVehicleRate: selection.Runtime.InCarRate, AimingRate: selection.Runtime.WeaponRate,
			StreamRate: selection.Runtime.StreamRate, StreamRadius: selection.Runtime.StreamDistance,
			PlayerTimeout: selection.Runtime.PlayerTimeout, AcksLimit: selection.Runtime.AckLimit,
			MessagesLimit: selection.Runtime.MessagesLimit, MessageHoleLimit: selection.Runtime.MessageHoleLimit,
			MinConnectionTime: selection.Runtime.MinConnectionTime, ConnectionSeed: selection.Runtime.ConnectionSeed,
			GameMode: selection.Runtime.GameModeText, MapName: selection.Runtime.MapName,
			LagCompMode: selection.Runtime.LagCompMode, RCON: selection.Runtime.RCON,
			RCONPassword: selection.Runtime.RCONPassword, LogQueries: selection.Runtime.LogQueries,
			LogChat: selection.Runtime.ChatLogging, LogTimestamps: selection.Runtime.Timestamp,
			LogTimestampFormat: selection.Runtime.LogTimeFormat, LogDatabase: selection.Runtime.DBLogging,
			LogDatabaseQueries: selection.Runtime.DBLogQueries, LogCookies: selection.Runtime.CookieLogging,
		}
	}
	return version, options, nil
}

func runtimeSleep(value any) (float64, error) {
	if value == nil {
		return 0, nil
	}
	var result float64
	switch value := value.(type) {
	case string:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, errors.New("runtime sleep must be a number")
		}
		result = parsed
	case float64:
		result = value
	case float32:
		result = float64(value)
	case int:
		result = float64(value)
	case int64:
		result = float64(value)
	default:
		return 0, errors.New("runtime sleep must be a number")
	}
	if result < 0 || math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, errors.New("runtime sleep must be a finite non-negative number")
	}
	return result, nil
}
