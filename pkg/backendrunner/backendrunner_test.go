package backendrunner

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	projectbackend "github.com/pawnkit/pawn-project/backend"
	"github.com/pawnkit/pawnkit-core/protocol"
)

type fakeRunner struct {
	calls int
}

func (runner *fakeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	runner.calls++
	if args[0] == "capabilities" {
		return []byte(`{
			"kind":"capabilities",
			"protocolVersion":1,
			"name":"fixture",
			"version":"1.0.0",
			"operations":["build"],
			"profiles":["openmp"]
		}`), nil
	}
	inputPath := args[2]
	outputPath := args[4]
	body, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}
	var request projectbackend.Request
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	result := projectbackend.Result{
		Kind: "result", SchemaVersion: 1, Status: "passed",
		Backend:   projectbackend.Identity{Name: "fixture", Version: "1.0.0"},
		Artifacts: []projectbackend.Artifact{}, Diagnostics: []protocol.Diagnostic{},
	}
	body, err = json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return nil, os.WriteFile(outputPath, body, 0o600)
}

func TestRunNegotiatesAndExecutes(t *testing.T) {
	runner := &fakeRunner{}
	result, err := Run(context.Background(), runner, "fixture", projectbackend.Request{
		Kind: "request", SchemaVersion: 1, Operation: projectbackend.Build,
		ProjectRoot: "/project", Profile: "openmp", Target: "openmp",
		Entry: "/project/main.pwn", Output: "/project/main.amx",
		IncludePaths: []string{}, Defines: map[string]string{}, Arguments: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" || runner.calls != 2 {
		t.Fatalf("result = %+v, calls = %d", result, runner.calls)
	}
}

func TestRunRejectsUnsupportedProfile(t *testing.T) {
	runner := &fakeRunner{}
	_, err := Run(context.Background(), runner, "fixture", projectbackend.Request{
		Operation: projectbackend.Build,
		Profile:   "samp-037",
	})
	if err == nil {
		t.Fatal("unsupported profile was accepted")
	}
	if runner.calls != 1 {
		t.Fatalf("calls = %d", runner.calls)
	}
}
