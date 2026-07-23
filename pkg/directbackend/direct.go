// Package directbackend builds a resolved request with pawncc.
package directbackend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	projectbackend "github.com/pawnkit/pawn-project/backend"
	"github.com/pawnkit/pawnkit-core/protocol"
)

const outputLimit = 1 << 20

func Execute(ctx context.Context, request projectbackend.Request, version string) (projectbackend.Result, error) {
	if request.Operation != projectbackend.Build || request.Compiler == nil {
		return projectbackend.Result{}, errors.New("direct backend requires a build request and compiler")
	}
	if err := os.MkdirAll(filepath.Dir(request.Output), 0o750); err != nil {
		return projectbackend.Result{}, fmt.Errorf("direct backend: creating output directory: %w", err)
	}
	if err := os.Remove(request.Output); err != nil && !errors.Is(err, os.ErrNotExist) {
		return projectbackend.Result{}, fmt.Errorf("direct backend: removing old output: %w", err)
	}

	stdout := &boundedBuffer{limit: outputLimit}
	stderr := &boundedBuffer{limit: outputLimit}
	command := exec.CommandContext(ctx, request.Compiler.Path, compilerArguments(request)...) //nolint:gosec // The resolved request selects the compiler.
	command.Dir = request.ProjectRoot
	command.Stdout = stdout
	command.Stderr = stderr
	runErr := command.Run()

	status := "passed"
	if ctx.Err() != nil {
		status = "cancelled"
	} else if runErr != nil {
		var exitError *exec.ExitError
		if !errors.As(runErr, &exitError) {
			return projectbackend.Result{}, fmt.Errorf("direct backend: starting compiler: %w", runErr)
		}
		status = "failed"
	}
	exitCode := 0
	if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
	}
	result := projectbackend.Result{
		Kind: "result", SchemaVersion: projectbackend.SchemaVersion, Status: status,
		Backend:   projectbackend.Identity{Name: "pawn-direct", Version: version},
		Artifacts: []projectbackend.Artifact{}, Diagnostics: []protocol.Diagnostic{},
		Process: &projectbackend.Process{
			ExitCode: &exitCode, Stdout: stdout.String(), Stderr: stderr.String(),
			Truncated: stdout.truncated || stderr.truncated,
		},
	}
	if status == "passed" {
		artifact, err := artifact(request.Output)
		if err != nil {
			return projectbackend.Result{}, err
		}
		result.Artifacts = append(result.Artifacts, artifact)
	}
	return result, nil
}

func compilerArguments(request projectbackend.Request) []string {
	arguments := []string{request.Entry}
	arguments = append(arguments, request.Arguments...)
	arguments = append(arguments, "-D"+request.ProjectRoot)
	for _, includePath := range request.IncludePaths {
		arguments = append(arguments, "-i"+includePath)
	}
	names := make([]string, 0, len(request.Defines))
	for name := range request.Defines {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		arguments = append(arguments, name+"="+request.Defines[name])
	}
	arguments = append(arguments, "-o"+request.Output)
	return arguments
}

func artifact(path string) (projectbackend.Artifact, error) {
	file, err := os.Open(path) //nolint:gosec // The request names the build output.
	if err != nil {
		return projectbackend.Artifact{}, fmt.Errorf("direct backend: opening output: %w", err)
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return projectbackend.Artifact{}, fmt.Errorf("direct backend: hashing output: %w", err)
	}
	return projectbackend.Artifact{
		Path: path, MediaType: "application/vnd.pawn.amx", Size: size,
		SHA256: hex.EncodeToString(digest.Sum(nil)),
	}, nil
}

type boundedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	written := len(value)
	remaining := buffer.limit - buffer.Len()
	if remaining <= 0 {
		buffer.truncated = true
		return written, nil
	}
	if len(value) > remaining {
		_, _ = buffer.Buffer.Write(value[:remaining])
		buffer.truncated = true
		return written, nil
	}
	_, _ = buffer.Buffer.Write(value)
	return written, nil
}
