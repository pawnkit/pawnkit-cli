package adapter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pawnkit/pawnkit-cli/pkg/adapter"
)

type runner struct {
	calls int
}

func (r *runner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls++
	if args[0] == "capabilities" {
		return []byte(`{"protocolVersion":1,"tool":"pawntest","version":"1.0.0","commands":["test"]}`), nil
	}
	if args[0] != "test" {
		return nil, errors.New("unexpected command")
	}
	return []byte(`{"schemaVersion":1,"status":"passed","message":"2 tests passed"}`), nil
}

func TestRun(t *testing.T) {
	fake := &runner{}
	result, document, err := adapter.Run(context.Background(), fake, "pawntest", "test", "/project")
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 || result.Status != "passed" || document.Tool != "pawntest" {
		t.Fatalf("calls=%d result=%+v document=%+v", fake.calls, result, document)
	}
}

func TestRunRejectsUnsupportedCommand(t *testing.T) {
	fake := &runner{}
	if _, _, err := adapter.Run(context.Background(), fake, "pawntest", "build", "/project"); err == nil {
		t.Fatal("unsupported command accepted")
	}
	if fake.calls != 1 {
		t.Fatalf("calls = %d", fake.calls)
	}
}
