// Package toolchain checks installed tools against a tested release set.
package toolchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const maxReleaseSetSize = 1 << 20

type Component struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ReleaseSet struct {
	SchemaVersion int         `json:"schemaVersion"`
	ID            string      `json:"id"`
	Components    []Component `json:"components"`
}

type Tool struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Actual   string `json:"actual,omitempty"`
	Path     string `json:"path,omitempty"`
	Status   string `json:"status"`
}

type Report struct {
	SchemaVersion int    `json:"schemaVersion"`
	ReleaseSet    string `json:"releaseSet"`
	Tools         []Tool `json:"tools"`
}

type Runner interface {
	LookPath(string) (string, error)
	Output(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (ExecRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // LookPath resolved the executable.
}

func Load(path string) (ReleaseSet, error) {
	file, err := os.Open(path) //nolint:gosec // The user selects this file.
	if err != nil {
		return ReleaseSet{}, fmt.Errorf("open release set: %w", err)
	}

	data, err := io.ReadAll(io.LimitReader(file, maxReleaseSetSize+1))
	closeErr := file.Close()
	if err != nil {
		return ReleaseSet{}, fmt.Errorf("read release set: %w", err)
	}
	if closeErr != nil {
		return ReleaseSet{}, fmt.Errorf("close release set: %w", closeErr)
	}
	if len(data) > maxReleaseSetSize {
		return ReleaseSet{}, errors.New("release set exceeds 1 MiB")
	}
	var set ReleaseSet
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&set); err != nil {
		return ReleaseSet{}, fmt.Errorf("decode release set: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ReleaseSet{}, errors.New("release set contains multiple JSON values")
	}
	if set.SchemaVersion != 1 {
		return ReleaseSet{}, fmt.Errorf("unsupported release-set schema version %d", set.SchemaVersion)
	}
	if set.ID == "" || len(set.Components) == 0 {
		return ReleaseSet{}, errors.New("release set is missing its id or components")
	}
	return set, nil
}

func Inspect(ctx context.Context, set ReleaseSet, selfVersion string, runner Runner) Report {
	report := Report{SchemaVersion: 1, ReleaseSet: set.ID}
	for _, component := range set.Components {
		if !knownTool(component.Name) {
			continue
		}
		current := Tool{Name: component.Name, Expected: component.Version}
		if component.Name == "pawn" {
			current.Actual = normalizeVersion(selfVersion)
			current.Status = compareVersion(current.Actual, current.Expected)
			report.Tools = append(report.Tools, current)
			continue
		}
		path, err := runner.LookPath(component.Name)
		if err != nil {
			current.Status = "missing"
			report.Tools = append(report.Tools, current)
			continue
		}
		current.Path = path
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		output, err := runner.Output(probeCtx, path, "--version")
		cancel()
		if err != nil {
			current.Status = "unavailable"
		} else {
			current.Actual = extractVersion(output)
			current.Status = compareVersion(current.Actual, current.Expected)
		}
		report.Tools = append(report.Tools, current)
	}
	return report
}

func (r Report) Compatible() bool {
	if len(r.Tools) == 0 {
		return false
	}
	for _, tool := range r.Tools {
		if tool.Status != "matched" {
			return false
		}
	}
	return true
}

func knownTool(name string) bool {
	switch name {
	case "pawn", "pawnfmt", "pawnlint", "pawnlsp", "pawntest":
		return true
	default:
		return false
	}
}

var versionPattern = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)

func extractVersion(output []byte) string {
	return normalizeVersion(versionPattern.FindString(string(bytes.TrimSpace(output))))
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "dev" || version == "(devel)" {
		return version
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return version
}

func compareVersion(actual, expected string) string {
	if actual == "" || actual == "dev" || actual == "(devel)" {
		return "unavailable"
	}
	if actual == normalizeVersion(expected) {
		return "matched"
	}
	return "mismatch"
}
