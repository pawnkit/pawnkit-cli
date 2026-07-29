package toolchain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type fakeRunner struct {
	paths   map[string]string
	outputs map[string][]byte
}

func (f fakeRunner) LookPath(name string) (string, error) {
	path, ok := f.paths[name]
	if !ok {
		return "", errors.New("not found")
	}
	return path, nil
}

func (f fakeRunner) Output(_ context.Context, name string, _ ...string) ([]byte, error) {
	return f.outputs[name], nil
}

func TestLoadAndInspect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.json")
	data := `{"schemaVersion":1,"id":"tested","components":[
		{"name":"pawn","version":"v1.4.2"},
		{"name":"pawnfmt","version":"v1.4.4"},
		{"name":"pawnlint","version":"v1.7.23"}
	]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	report := Inspect(context.Background(), set, "1.4.2", fakeRunner{
		paths:   map[string]string{"pawnfmt": "/bin/pawnfmt"},
		outputs: map[string][]byte{"/bin/pawnfmt": []byte("pawnfmt v1.4.4 (abc)")},
	})
	if len(report.Tools) != 3 || report.Tools[0].Status != "matched" ||
		report.Tools[1].Status != "matched" || report.Tools[2].Status != "missing" {
		t.Fatalf("report = %#v", report)
	}
	if report.Compatible() {
		t.Fatal("report with a missing tool is compatible")
	}
}

func TestLoadAcceptsCurrentReleaseSetSchemas(t *testing.T) {
	t.Parallel()

	for _, version := range []int{1, 2, 3} {
		path := filepath.Join(t.TempDir(), "set.json")
		data := fmt.Sprintf(`{"schemaVersion":%d,"id":"tested","components":[{"name":"pawn","version":"v1.5.1"}]}`, version)
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err != nil {
			t.Fatalf("schema v%d: %v", version, err)
		}
	}
}

func TestLoadRejectsUnsupportedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":4,"id":"next","components":[{"name":"pawn","version":"v1"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unsupported schema was accepted")
	}
}
