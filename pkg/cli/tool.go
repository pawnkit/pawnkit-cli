package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

func runFormat(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	project, forwarded, ok := toolArgs("fmt", args, stderr)
	if !ok {
		return ExitUsage
	}
	if !hasAny(forwarded, "--write", "-w", "--check", "--diff", "--stdin", "--print-config", "--init-config") {
		forwarded = append([]string{"--write"}, forwarded...)
	}
	return runTool(ctx, "pawnfmt", project, forwarded, stdout, stderr)
}

func runLint(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	project, forwarded, ok := toolArgs("lint", args, stderr)
	if !ok {
		return ExitUsage
	}
	return runTool(ctx, "pawnlint", project, forwarded, stdout, stderr)
}

func runTest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	project, forwarded, ok := toolArgs("test", args, stderr)
	if !ok {
		return ExitUsage
	}
	return runTool(ctx, "pawntest", project, forwarded, stdout, stderr)
}

func toolArgs(name string, args []string, stderr io.Writer) (string, []string, bool) {
	project := "."
	forwarded := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		switch {
		case args[index] == "--project":
			if index+1 == len(args) {
				_, _ = fmt.Fprintf(stderr, "pawn %s: --project requires a directory\n", name)
				return "", nil, false
			}
			index++
			project = args[index]
		case strings.HasPrefix(args[index], "--project="):
			project = strings.TrimPrefix(args[index], "--project=")
			if project == "" {
				_, _ = fmt.Fprintf(stderr, "pawn %s: --project requires a directory\n", name)
				return "", nil, false
			}
		default:
			forwarded = append(forwarded, args[index])
		}
	}
	root, err := filepath.Abs(project)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "pawn %s: %v\n", name, err)
		return "", nil, false
	}
	return root, forwarded, true
}

func runTool(ctx context.Context, name, project string, args []string, stdout, stderr io.Writer) int {
	return executeTool(ctx, name, project, args, stdout, stderr)
}

var executeTool = executeExternalTool

func executeExternalTool(ctx context.Context, name, project string, args []string, stdout, stderr io.Writer) int {
	path, err := exec.LookPath(name)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "pawn: %s was not found on PATH\n", name)
		return ExitInternal
	}
	command := exec.CommandContext(ctx, path, args...) //nolint:gosec // PATH selects the installed PawnKit tool.
	command.Dir = project
	command.Stdout = stdout
	command.Stderr = stderr
	command.Stdin = os.Stdin
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return ExitFindings
		}
		_, _ = fmt.Fprintf(stderr, "pawn: running %s: %v\n", name, err)
		return ExitInternal
	}
	return ExitOK
}

func hasAny(args []string, values ...string) bool {
	for _, arg := range args {
		if slices.Contains(values, arg) {
			return true
		}
	}
	return false
}
