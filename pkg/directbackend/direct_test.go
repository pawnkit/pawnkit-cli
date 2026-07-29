package directbackend

import (
	"path/filepath"
	"slices"
	"testing"

	projectbackend "github.com/pawnkit/pawn-project/backend"
)

func TestCompilerArgumentsUseResolvedOrder(t *testing.T) {
	request := projectbackend.Request{
		ProjectRoot: "/project",
		Entry:       "/project/main.pwn",
		Output:      "/project/build/main.amx",
		IncludePaths: []string{
			"/project/include",
			"/project/dependencies/lib",
		},
		Defines:   map[string]string{"SECOND": "2", "FIRST": "1"},
		Arguments: []string{"-d3"},
	}
	got := compilerArguments(request)
	want := []string{
		"/project/main.pwn", "-d3", "-D/project",
		"-i/project/include", "-i/project/dependencies/lib",
		"FIRST=1", "SECOND=2", "-o/project/build/main.amx",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("arguments = %v, want %v", got, want)
	}
}

func TestCompilerDiagnosticsParsePawnCCOutput(t *testing.T) {
	root := t.TempDir()
	output := "gamemodes/main.pwn(42) : error 017: undefined symbol \"Player\"\n" +
		"gamemodes/main.pwn(50 -- 52) : warning 203: symbol is never used: \"value\"\n" +
		"Pawn compiler 3.10.11\n"
	diagnostics := compilerDiagnostics(root, output, output)
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].Code != "pawncc-017" || diagnostics[0].Severity != "error" ||
		diagnostics[0].Primary.Range == nil || diagnostics[0].Primary.Range.Start.Line != 41 {
		t.Fatalf("error diagnostic = %#v", diagnostics[0])
	}
	wantURI := "file://" + filepath.ToSlash(filepath.Join(root, "gamemodes", "main.pwn"))
	if diagnostics[0].Primary.URI != wantURI {
		t.Fatalf("URI = %q, want %q", diagnostics[0].Primary.URI, wantURI)
	}
	if diagnostics[1].Code != "pawncc-203" || diagnostics[1].Severity != "warning" {
		t.Fatalf("warning diagnostic = %#v", diagnostics[1])
	}
}

func TestBoundedBufferTruncatesWithoutStoppingWriter(t *testing.T) {
	buffer := &boundedBuffer{limit: 4}
	n, err := buffer.Write([]byte("abcdef"))
	if err != nil || n != 6 || buffer.String() != "abcd" || !buffer.truncated {
		t.Fatalf("write = %d, %v; buffer = %q, truncated = %v", n, err, buffer.String(), buffer.truncated)
	}
}
