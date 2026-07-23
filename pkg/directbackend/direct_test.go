package directbackend

import (
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

func TestBoundedBufferTruncatesWithoutStoppingWriter(t *testing.T) {
	buffer := &boundedBuffer{limit: 4}
	n, err := buffer.Write([]byte("abcdef"))
	if err != nil || n != 6 || buffer.String() != "abcd" || !buffer.truncated {
		t.Fatalf("write = %d, %v; buffer = %q, truncated = %v", n, err, buffer.String(), buffer.truncated)
	}
}
