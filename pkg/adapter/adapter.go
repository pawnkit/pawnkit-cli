// Package adapter runs negotiated external PawnKit tools.
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/pawnkit/pawnkit-cli/pkg/capability"
)

type Result struct {
	SchemaVersion int    `json:"schemaVersion"`
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
}

func Run(ctx context.Context, runner capability.Runner, executable, command, project string) (Result, capability.Document, error) {
	document, err := capability.Probe(ctx, runner, executable)
	if err != nil {
		return Result{}, capability.Document{}, err
	}
	if !document.Supports(command) {
		return Result{}, document, fmt.Errorf("adapter: %s %s does not support %s", document.Tool, document.Version, command)
	}
	output, err := runner.Run(ctx, executable, command, "--project", project, "--output", "json")
	if err != nil {
		return Result{}, document, fmt.Errorf("adapter: %s: %w", document.Tool, err)
	}
	if len(output) > 1<<20 {
		return Result{}, document, errors.New("adapter: response exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	var result Result
	if err := decoder.Decode(&result); err != nil {
		return Result{}, document, fmt.Errorf("adapter: decoding %s result: %w", document.Tool, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Result{}, document, fmt.Errorf("adapter: invalid trailing data from %s", document.Tool)
	}
	if result.SchemaVersion != 1 || result.Status != "passed" && result.Status != "failed" {
		return Result{}, document, fmt.Errorf("adapter: invalid %s result", document.Tool)
	}
	return result, document, nil
}
