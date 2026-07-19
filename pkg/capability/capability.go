// Package capability negotiates optional PawnKit tool adapters.
package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
)

const ProtocolVersion = 1

var (
	ErrUnavailable  = errors.New("capability: tool unavailable")
	ErrIncompatible = errors.New("capability: incompatible protocol")
)

type Document struct {
	ProtocolVersion int      `json:"protocolVersion"`
	Tool            string   `json:"tool"`
	Version         string   `json:"version"`
	Commands        []string `json:"commands"`
}

func (d Document) Supports(command string) bool {
	return slices.Contains(d.Commands, command)
}

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // The user selects the tool.
	var output cappedBuffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.Bytes(), err
}

type cappedBuffer struct {
	bytes.Buffer
}

func (b *cappedBuffer) Write(value []byte) (int, error) {
	remaining := (1 << 20) - b.Len()
	if remaining <= 0 {
		return 0, errors.New("capability: response exceeds 1 MiB")
	}
	if len(value) > remaining {
		_, _ = b.Buffer.Write(value[:remaining])
		return remaining, errors.New("capability: response exceeds 1 MiB")
	}
	return b.Buffer.Write(value)
}

func Probe(ctx context.Context, runner Runner, executable string) (Document, error) {
	if runner == nil || executable == "" {
		return Document{}, ErrUnavailable
	}
	output, err := runner.Run(ctx, executable, "capabilities", "--output", "json")
	if err != nil {
		return Document{}, fmt.Errorf("%w: %s: %w", ErrUnavailable, executable, err)
	}
	if len(output) > 1<<20 {
		return Document{}, errors.New("capability: response exceeds 1 MiB")
	}
	var document Document
	if err := json.Unmarshal(output, &document); err != nil {
		return Document{}, fmt.Errorf("capability: decoding %s response: %w", executable, err)
	}
	if document.ProtocolVersion != ProtocolVersion || document.Tool == "" || document.Version == "" {
		return Document{}, fmt.Errorf("%w: %s protocol %d", ErrIncompatible, executable, document.ProtocolVersion)
	}
	return document, nil
}
