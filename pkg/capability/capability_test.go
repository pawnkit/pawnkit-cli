package capability_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pawnkit/pawnkit-cli/pkg/capability"
)

type runner struct {
	output []byte
	err    error
}

func (r runner) Run(context.Context, string, ...string) ([]byte, error) {
	return r.output, r.err
}

func TestProbe(t *testing.T) {
	document, err := capability.Probe(context.Background(), runner{output: []byte(`{"protocolVersion":1,"tool":"pawntest","version":"1.0.0","commands":["test"]}`)}, "pawntest")
	if err != nil {
		t.Fatal(err)
	}
	if !document.Supports("test") || document.Supports("build") {
		t.Fatalf("document = %+v", document)
	}
}

func TestProbeRejectsUnavailableAndIncompatible(t *testing.T) {
	if _, err := capability.Probe(context.Background(), nil, ""); !errors.Is(err, capability.ErrUnavailable) {
		t.Fatalf("unavailable error = %v", err)
	}
	_, err := capability.Probe(context.Background(), runner{output: []byte(`{"protocolVersion":2,"tool":"pawntest","version":"2"}`)}, "pawntest")
	if !errors.Is(err, capability.ErrIncompatible) {
		t.Fatalf("incompatible error = %v", err)
	}
}

func TestProbeBoundsOutput(t *testing.T) {
	output := make([]byte, 1<<20+1)
	if _, err := capability.Probe(context.Background(), runner{output: output}, "tool"); err == nil {
		t.Fatal("oversized response accepted")
	}
}
