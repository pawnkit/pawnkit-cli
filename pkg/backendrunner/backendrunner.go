// Package backendrunner invokes build backends through RFC 0012.
package backendrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	projectbackend "github.com/pawnkit/pawn-project/backend"
	"github.com/pawnkit/pawnkit-cli/pkg/capability"
)

const maxMessageBytes = 8 << 20

func Run(
	ctx context.Context,
	runner capability.Runner,
	executable string,
	request projectbackend.Request,
) (projectbackend.Result, error) {
	capabilities, err := Probe(ctx, runner, executable)
	if err != nil {
		return projectbackend.Result{}, err
	}
	if !slices.Contains(capabilities.Operations, request.Operation) {
		return projectbackend.Result{}, fmt.Errorf(
			"backend %s %s does not support %s",
			capabilities.Name, capabilities.Version, request.Operation,
		)
	}
	if len(capabilities.Profiles) != 0 && !slices.Contains(capabilities.Profiles, request.Profile) {
		return projectbackend.Result{}, fmt.Errorf(
			"backend %s %s does not support profile %s",
			capabilities.Name, capabilities.Version, request.Profile,
		)
	}

	dir, err := os.MkdirTemp("", "pawn-backend-")
	if err != nil {
		return projectbackend.Result{}, fmt.Errorf("backend: creating request directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	inputPath := filepath.Join(dir, "request.json")
	outputPath := filepath.Join(dir, "result.json")
	body, err := json.Marshal(request)
	if err != nil {
		return projectbackend.Result{}, fmt.Errorf("backend: encoding request: %w", err)
	}
	if err := os.WriteFile(inputPath, body, 0o600); err != nil {
		return projectbackend.Result{}, fmt.Errorf("backend: writing request: %w", err)
	}
	if _, err := runner.Run(ctx, executable, "execute", "--input", inputPath, "--output", outputPath); err != nil {
		return projectbackend.Result{}, fmt.Errorf("backend: execute: %w", err)
	}
	resultBody, err := readBounded(outputPath)
	if err != nil {
		return projectbackend.Result{}, err
	}
	var result projectbackend.Result
	if err := decodeStrict(resultBody, &result); err != nil {
		return projectbackend.Result{}, fmt.Errorf("backend: decoding result: %w", err)
	}
	if result.Kind != "result" || result.SchemaVersion != projectbackend.SchemaVersion ||
		result.Backend.Name == "" || result.Backend.Version == "" ||
		result.Artifacts == nil || result.Diagnostics == nil ||
		result.Status != "passed" && result.Status != "failed" && result.Status != "cancelled" {
		return projectbackend.Result{}, errors.New("backend: invalid result")
	}
	return result, nil
}

func Probe(ctx context.Context, runner capability.Runner, executable string) (projectbackend.Capabilities, error) {
	if runner == nil || executable == "" {
		return projectbackend.Capabilities{}, errors.New("backend: executable is required")
	}
	body, err := runner.Run(ctx, executable, "capabilities", "--output", "json")
	if err != nil {
		return projectbackend.Capabilities{}, fmt.Errorf("backend: capabilities: %w", err)
	}
	if len(body) > maxMessageBytes {
		return projectbackend.Capabilities{}, errors.New("backend: capabilities exceed 8 MiB")
	}
	var document projectbackend.Capabilities
	if err := decodeStrict(body, &document); err != nil {
		return projectbackend.Capabilities{}, fmt.Errorf("backend: decoding capabilities: %w", err)
	}
	if document.Kind != "capabilities" ||
		document.ProtocolVersion != projectbackend.ProtocolVersion ||
		document.Name == "" || document.Version == "" {
		return projectbackend.Capabilities{}, errors.New("backend: incompatible capabilities")
	}
	return document, nil
}

func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path) //nolint:gosec // The path is in an owner-only temporary directory.
	if err != nil {
		return nil, fmt.Errorf("backend: reading result: %w", err)
	}
	defer func() { _ = file.Close() }()
	body, err := io.ReadAll(io.LimitReader(file, maxMessageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("backend: reading result: %w", err)
	}
	if len(body) > maxMessageBytes {
		return nil, errors.New("backend: result exceeds 8 MiB")
	}
	return body, nil
}

func decodeStrict(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("unexpected trailing data")
	}
	return nil
}
