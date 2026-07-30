package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawn-project/toolchain"
	"github.com/pawnkit/pawnfmt"
	"github.com/pawnkit/pawnkit-core/source"
)

func TestHelpAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), nil, &stdout, &stderr, "test"); code != ExitOK {
		t.Fatalf("help code = %d", code)
	}
	if !strings.Contains(stdout.String(), "pawn check") || !strings.Contains(stdout.String(), "pawn fmt") ||
		!strings.Contains(stdout.String(), "pawn lint") || !strings.Contains(stdout.String(), "pawn test") ||
		!strings.Contains(stdout.String(), "pawn doctor") || !strings.Contains(stdout.String(), "pawn audit") {
		t.Fatalf("help = %q", stdout.String())
	}
	if code := Run(context.Background(), []string{"unknown"}, &stdout, &stderr, "test"); code != ExitUsage {
		t.Fatalf("unknown code = %d", code)
	}
}

func TestToolCommandsRunFromProject(t *testing.T) {
	project := t.TempDir()
	type call struct {
		name    string
		project string
		args    []string
	}
	var calls []call
	previous := executeTool
	executeTool = func(_ context.Context, name, project string, args []string, _, _ io.Writer) int {
		calls = append(calls, call{name: name, project: project, args: slices.Clone(args)})
		return ExitOK
	}
	t.Cleanup(func() {
		executeTool = previous
	})

	tests := []struct {
		args     []string
		wantName string
		wantArgs []string
	}{
		{[]string{"fmt", "--project", project}, "pawnfmt", []string{"--write"}},
		{[]string{"fmt", "--project", project, "--check"}, "pawnfmt", []string{"--check"}},
		{[]string{"lint", "--project", project, "--profile", "strict"}, "pawnlint", []string{"--profile", "strict"}},
		{[]string{"test", "--project", project, "--run", "sample"}, "pawntest", []string{"--run", "sample"}},
	}
	for _, test := range tests {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), test.args, &stdout, &stderr, "test"); code != ExitOK {
			t.Fatalf("%v code=%d stderr=%s", test.args, code, stderr.String())
		}
	}
	if len(calls) != len(tests) {
		t.Fatalf("calls = %v", calls)
	}
	for index, test := range tests {
		got := calls[index]
		if got.name != test.wantName || got.project != project || !slices.Equal(got.args, test.wantArgs) {
			t.Errorf("call %d = %#v, want name=%q project=%q args=%v", index, got, test.wantName, project, test.wantArgs)
		}
	}
}

func TestToolCommandReportsMissingExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"lint"}, &stdout, &stderr, "test"); code != ExitInternal {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "pawnlint was not found on PATH") {
		t.Fatalf("stderr=%s", stderr.String())
	}
}

func TestCompilerCandidatesPreferOpenMPCompiler(t *testing.T) {
	got := compilerCandidates("openmp")
	if len(got) != 2 || got[0] != "openmp-pawncc" || got[1] != "pawncc" {
		t.Fatalf("compilerCandidates(openmp) = %v", got)
	}
	got = compilerCandidates("samp-037")
	if len(got) != 1 || got[0] != "pawncc" {
		t.Fatalf("compilerCandidates(samp-037) = %v", got)
	}
}

type compilerDownload []byte

func (content compilerDownload) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(content)), nil
}

type compilerLookup struct {
	path string
	err  error
}

func (lookup compilerLookup) LookPath(string) (string, error) {
	return lookup.path, lookup.err
}

func TestResolveCompilerUsesPinnedCacheBeforePath(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "pawn.json"), []byte(`{
		"entry":"main.pwn",
		"preset":"openmp",
		"build":{"compiler":{"user":"openmultiplayer","repo":"compiler","version":"3.10.11"}}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.pwn"), []byte("main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, projectDir, projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(t.TempDir(), "toolchains")
	info, err := toolchain.NewResolver(
		toolchain.OSCacheFS{}, cacheDir, compilerDownload("compiler"), nil,
	).Update(context.Background(), toolchain.ResolveOptions{
		Vendor:      toolchain.VendorOpenMultiplayer,
		Version:     "3.10.11",
		DownloadURL: "https://example.test/compiler",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveCompiler(
		context.Background(), loaded, cacheDir, toolchain.OSCacheFS{},
		compilerLookup{err: errors.New("PATH must not be used")},
	)
	if err != nil || got != info.Path {
		t.Fatalf("resolveCompiler = %q, %v; want %q", got, err, info.Path)
	}
}

func TestResolveCompilerFallsBackToPathWhenCacheIsEmpty(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "pawn.json"), []byte(`{
		"entry":"main.pwn",
		"preset":"samp",
		"build":{"compiler":{"user":"pawn-lang","repo":"compiler","version":"3.10.10"}}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.pwn"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, projectDir, projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveCompiler(
		context.Background(), loaded, filepath.Join(t.TempDir(), "toolchains"),
		toolchain.OSCacheFS{}, compilerLookup{path: "/tools/pawncc"},
	)
	if err != nil || got != "/tools/pawncc" {
		t.Fatalf("resolveCompiler = %q, %v", got, err)
	}
}

func TestRestoreAcceptsLockedLocalDependency(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "include"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), []byte("main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "pawn.json"),
		[]byte(`{"entry":"main.pwn","experimental":{"build_file":false}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "pawn.lock"),
		[]byte(`{
			"version": 1,
			"generated": "2026-07-30T00:00:00Z",
			"sampctl_version": "1.14.1",
			"dependencies": {
				"local:include": {
					"constraint": "local:include",
					"resolved": "local:include",
					"commit": "0000000",
					"user": "local",
					"repo": "include",
					"local": "include"
				}
			}
		}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(
		context.Background(),
		[]string{"restore", "--project", dir, "--format", "json"},
		&stdout,
		&stderr,
		"test",
	)
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "local"`) {
		t.Fatalf("output = %s", stdout.String())
	}
}

func TestToolchainReportsPinnedCLI(t *testing.T) {
	releaseSet := filepath.Join(t.TempDir(), "release-set.json")
	content := `{"schemaVersion":1,"id":"tested","components":[{"name":"pawn","version":"v1.5.0"}]}`
	if err := os.WriteFile(releaseSet, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"toolchain", "--release-set", releaseSet, "--output", "json"}, &stdout, &stderr, "1.5.0")
	if code != ExitOK || !strings.Contains(stdout.String(), `"status": "matched"`) {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
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

func TestProjectReportsResolvedSelection(t *testing.T) {
	dir := t.TempDir()
	manifest := `{
		"entry": "main.pwn",
		"preset": "openmp",
		"builds": [{"name": "development"}],
		"runtimes": [{"name": "local"}]
	}`
	if err := os.WriteFile(filepath.Join(dir, "pawn.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.pwn"), []byte("main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"project", "--project", dir, "--output", "json"}, &stdout, &stderr, "test")
	if code != ExitOK {
		t.Fatalf("code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var report projectReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || report.Profile != "openmp" || report.Build != "development" ||
		report.Runtime != "local" || report.Entry != filepath.Join(dir, "main.pwn") {
		t.Fatalf("project report = %#v", report)
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
