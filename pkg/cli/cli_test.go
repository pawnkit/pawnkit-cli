package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pawnkit/pawnfmt"
)

func TestHelpAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), nil, &stdout, &stderr, "test"); code != ExitOK {
		t.Fatalf("help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "pawn check") || !strings.Contains(stdout.String(), "pawn doctor") || !strings.Contains(stdout.String(), "pawn audit") {
		t.Fatalf("help = %q", stdout.String())
	}
	if code := Run(context.Background(), []string{"unknown"}, &stdout, &stderr, "test"); code != ExitUsage {
		t.Fatalf("unknown code = %d", code)
	}
}

func TestInitDiscoversProjectAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "include"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), []byte("main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"init", "--project", dir, "--target", "samp", "--include", "include"}
	if code := Run(context.Background(), args, &stdout, &stderr, "test"); code != ExitOK {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	content, err := os.ReadFile(filepath.Join(dir, "pawn.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "entry": "main.pwn",
  "preset": "samp",
  "experimental": {
    "build_file": false
  },
  "pawnkit": {
    "schemaVersion": 1,
    "profile": "samp",
    "includePaths": [
      "include"
    ]
  }
}
`
	if string(content) != want {
		t.Fatalf("manifest:\n%s\nwant:\n%s", content, want)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), args, &stdout, &stderr, "test"); code != ExitUsage || !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("overwrite code=%d stderr=%q", code, stderr.String())
	}
}

func TestInitRequiresUnambiguousContainedPaths(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"one.pwn", "two.pwn"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"init", "--project", dir}, &stdout, &stderr, "test"); code != ExitUsage || !strings.Contains(stderr.String(), "pass --entry") {
		t.Fatalf("ambiguous code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	if code := Run(context.Background(), []string{"init", "--project", dir, "--entry", "../outside.pwn"}, &stdout, &stderr, "test"); code != ExitUsage || !strings.Contains(stderr.String(), "outside") {
		t.Fatalf("outside code=%d stderr=%q", code, stderr.String())
	}
}

func TestAuditOfflineDisclaimer(t *testing.T) {
	dir := testProject(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"audit", "--project", dir}, &stdout, &stderr, "test")
	if code != ExitFindings || !strings.Contains(stdout.String(), "does not mean") || strings.Contains(stdout.String(), "safe\n") {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestAuditWritesSBOMTransactionally(t *testing.T) {
	dir := testProject(t)
	if err := os.WriteFile(filepath.Join(dir, "pawn.lock"), []byte(`{"schemaVersion":1,"packages":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "bom.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"audit", "--project", dir, "--sbom", "cyclonedx", "--sbom-output", destination}, &stdout, &stderr, "test")
	if code != ExitOK {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	content, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(content), `"bomFormat": "CycloneDX"`) {
		t.Fatalf("content=%s error=%v", content, err)
	}
}

func TestAuditRejectsUnavailableOnlineMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"audit", "--offline=false"}, &stdout, &stderr, "test")
	if code != ExitUsage || !strings.Contains(stderr.String(), "not configured") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestCheckAndDoctorJSON(t *testing.T) {
	dir := testProject(t)
	for _, command := range [][]string{
		{"check", "--project", dir, "--only", "project", "--output", "json"},
		{"doctor", "--project", dir, "--output", "json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), command, &stdout, &stderr, "test"); code != ExitOK {
			t.Fatalf("%v code=%d stderr=%s", command, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), `"schemaVersion": 1`) {
			t.Fatalf("%v output=%s", command, stdout.String())
		}
	}
}

func TestCheckFindsFormatting(t *testing.T) {
	dir := testProject(t)
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), []byte("main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", dir, "--only", "project,format"}, &stdout, &stderr, "test")
	if code != ExitFindings || !strings.Contains(stdout.String(), "not formatted") {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestCheckRejectsUnknownTask(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--only", "missing"}, &stdout, &stderr, "test")
	if code != ExitUsage || !strings.Contains(stderr.String(), "unknown task") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestCheckChangedFiles(t *testing.T) {
	dir := testProject(t)
	formatted, err := pawnfmt.Format([]byte("stock Helper() {}\n"), pawnfmt.Options{TabSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper.inc"), formatted, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", dir, "--changed-files", "helper.inc", "--only", "project,format"}, &stdout, &stderr, "test")
	if code != ExitOK || !strings.Contains(stdout.String(), "1 source file") {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestCheckRejectsChangedFileOutsideProject(t *testing.T) {
	dir := testProject(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", dir, "--changed-files", "../outside.pwn"}, &stdout, &stderr, "test")
	if code != ExitUsage || !strings.Contains(stderr.String(), "outside the project") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestCheckSARIF(t *testing.T) {
	dir := testProject(t)
	formatted, err := pawnfmt.Format([]byte("main() { new value; value(); }\n"), pawnfmt.Options{TabSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), formatted, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", dir, "--only", "lint", "--output", "sarif"}, &stdout, &stderr, "test")
	if code != ExitFindings || !strings.Contains(stdout.String(), `"version": "2.1.0"`) || !strings.Contains(stdout.String(), "non-callable-symbol") {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestCheckHumanOutputIncludesFinding(t *testing.T) {
	dir := testProject(t)
	formatted, err := pawnfmt.Format([]byte("main() { new value; value(); }\n"), pawnfmt.Options{TabSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), formatted, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", dir, "--only", "lint"}, &stdout, &stderr, "test")
	if code != ExitFindings || !strings.Contains(stdout.String(), "main.pwn:") || !strings.Contains(stdout.String(), "[non-callable-symbol]") {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestCheckAcceptsCompilerConstantsAndVoidFunctions(t *testing.T) {
	dir := testProject(t)
	source := []byte("#if cellbits == 32\nvoid:Reset() {}\n#endif\nmain() { Reset(); }\n")
	formatted, err := pawnfmt.Format(source, pawnfmt.Options{TabSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), formatted, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", dir, "--only", "lint"}, &stdout, &stderr, "test")
	if code != ExitOK {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func testProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pawn.json"), []byte(`{"entry":"main.pwn","experimental":{"build_file":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	formatted, err := pawnfmt.Format([]byte("main() {}\n"), pawnfmt.Options{TabSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), formatted, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
