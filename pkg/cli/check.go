package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawnfmt"
	"github.com/pawnkit/pawnkit-cli/pkg/adapter"
	"github.com/pawnkit/pawnkit-cli/pkg/capability"
	"github.com/pawnkit/pawnkit-cli/pkg/workflow"
	"github.com/pawnkit/pawnkit-core/source"
	"github.com/pawnkit/pawnlint/pkg/analyzer"
)

type checkReport struct {
	SchemaVersion int               `json:"schemaVersion"`
	Tasks         []workflow.Result `json:"tasks"`
}

func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	only := flags.String("only", "", "comma-separated tasks")
	skip := flags.String("skip", "", "comma-separated tasks")
	failFast := flags.Bool("fail-fast", false, "stop after the first failure")
	jobs := flags.Int("jobs", 1, "maximum concurrent tasks")
	output := flags.String("output", "human", "human, json, or sarif")
	buildTool := flags.String("build-tool", "", "negotiated build executable")
	testTool := flags.String("test-tool", "", "negotiated test executable")
	changedFiles := flags.String("changed-files", "", "comma-separated project files")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *jobs < 1 {
		return ExitUsage
	}
	root, err := filepath.Abs(*projectDir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn check:", err)
		return ExitInternal
	}
	changed, err := resolveChangedFiles(root, *changedFiles)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn check:", err)
		return ExitUsage
	}
	state := &checkState{root: root, changed: changed}
	tasks := []workflow.Task{
		{Name: "project", Run: state.loadProject},
		{Name: "format", DependsOn: []string{"project"}, Run: state.checkFormat},
		{Name: "lint", DependsOn: []string{"project"}, Run: state.checkLint},
	}
	if *buildTool != "" {
		tasks = append(tasks, workflow.Task{Name: "build", DependsOn: []string{"project", "lint"}, Run: externalTask(*buildTool, "build", root)})
	}
	if *testTool != "" {
		dependencies := []string{"project", "lint"}
		if *buildTool != "" {
			dependencies = append(dependencies, "build")
		}
		tasks = append(tasks, workflow.Task{Name: "test", DependsOn: dependencies, Run: externalTask(*testTool, "test", root)})
	}
	onlySet, skipSet := nameSet(*only), nameSet(*skip)
	if unknown := unknownTask(tasks, onlySet, skipSet); unknown != "" {
		_, _ = fmt.Fprintf(stderr, "pawn check: unknown task %q\n", unknown)
		return ExitUsage
	}
	results, err := workflow.Run(ctx, tasks, workflow.Options{Only: onlySet, Skip: skipSet, FailFast: *failFast, Parallelism: *jobs})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn check:", err)
		return ExitInternal
	}
	switch *output {
	case "json":
		if err := writeJSON(stdout, checkReport{SchemaVersion: 1, Tasks: results}); err != nil {
			return ExitInternal
		}
	case "sarif":
		if err := writeSARIF(stdout, results); err != nil {
			return ExitInternal
		}
	case "human":
		for _, result := range results {
			if _, err := fmt.Fprintf(stdout, "%-8s %-7s %s\n", result.Name, result.Status, result.Message); err != nil {
				return ExitInternal
			}
			for _, finding := range result.Findings {
				if _, err := fmt.Fprintf(stdout, "  %s:%d:%d: %s: %s [%s]\n", finding.Path, finding.Line, finding.Column, finding.Severity, finding.Message, finding.RuleID); err != nil {
					return ExitInternal
				}
			}
		}
	default:
		_, _ = fmt.Fprintln(stderr, "pawn check: --output must be human, json, or sarif")
		return ExitUsage
	}
	for _, result := range results {
		if result.Status == "failed" {
			return ExitFindings
		}
	}
	return ExitOK
}

func externalTask(executable, command, project string) func(context.Context) workflow.Result {
	return func(ctx context.Context) workflow.Result {
		result, document, err := adapter.Run(ctx, capability.OSRunner{}, executable, command, project)
		if err != nil {
			return failed(executable, err)
		}
		return workflow.Result{Status: result.Status, Tool: document.Tool, Message: result.Message}
	}
}

func unknownTask(tasks []workflow.Task, sets ...map[string]bool) string {
	known := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		known[task.Name] = true
	}
	for _, set := range sets {
		for name := range set {
			if !known[name] {
				return name
			}
		}
	}
	return ""
}

type checkState struct {
	root    string
	project *projectmodel.Project
	changed []string
	sources []analyzer.Source
}

func (s *checkState) loadProject(context.Context) workflow.Result {
	project, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, s.root, projectmodel.Options{})
	if err != nil {
		return failed("pawn-project", err)
	}
	s.project = project
	if len(project.Diagnostics()) != 0 {
		return workflow.Result{Status: "failed", Tool: "pawn-project", Message: fmt.Sprintf("%d project diagnostic(s)", len(project.Diagnostics()))}
	}
	if project.Paths().Entry == "" {
		return workflow.Result{Status: "failed", Tool: "pawn-project", Message: "manifest has no entry"}
	}
	paths := s.changed
	if len(paths) == 0 {
		paths = []string{project.Paths().Entry}
	}
	for _, path := range paths {
		content, err := os.ReadFile(path) //nolint:gosec // Changed files are contained within the project.
		if err != nil {
			return failed("pawn-project", err)
		}
		s.sources = append(s.sources, analyzer.Source{Path: path, Content: content})
	}
	return workflow.Result{Status: "passed", Tool: "pawn-project", Message: fmt.Sprintf("%d source file(s)", len(s.sources))}
}

func (s *checkState) checkFormat(context.Context) workflow.Result {
	if s.project == nil {
		return workflow.Result{Status: "failed", Tool: "pawnfmt", Message: "project task did not pass"}
	}
	for _, source := range s.sources {
		formatted, err := pawnfmt.Format(source.Content, pawnfmt.Options{TabSize: 4})
		if err != nil {
			return failed("pawnfmt", err)
		}
		if !bytes.Equal(formatted, source.Content) {
			return workflow.Result{Status: "failed", Tool: "pawnfmt", Message: filepath.Base(source.Path) + " is not formatted"}
		}
	}
	return workflow.Result{Status: "passed", Tool: "pawnfmt", Message: fmt.Sprintf("%d source file(s) formatted", len(s.sources))}
}

func (s *checkState) checkLint(ctx context.Context) workflow.Result {
	if s.project == nil {
		return workflow.Result{Status: "failed", Tool: "pawnlint", Message: "project task did not pass"}
	}
	result, err := analyzer.Analyze(ctx, analyzer.Request{
		Sources:          s.sources,
		WorkingDirectory: s.root,
		IncludePaths:     s.project.Paths().IncludeRoots,
	})
	if err != nil {
		return failed("pawnlint", err)
	}
	if len(result.Diagnostics) != 0 {
		findings := make([]workflow.Finding, 0, len(result.Diagnostics))
		for _, diagnostic := range result.Diagnostics {
			findings = append(findings, workflow.Finding{
				RuleID: diagnostic.RuleID, Severity: diagnostic.Severity, Message: diagnostic.Message,
				Path: diagnostic.Path, Line: diagnostic.Range.Start.Line, Column: diagnostic.Range.Start.Column,
			})
		}
		return workflow.Result{Status: "failed", Tool: "pawnlint", Message: fmt.Sprintf("%d diagnostic(s)", len(result.Diagnostics)), Findings: findings}
	}
	return workflow.Result{Status: "passed", Tool: "pawnlint", Message: "no diagnostics"}
}

func resolveChangedFiles(root, value string) ([]string, error) {
	var files []string
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		candidate := item
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(root, candidate)
		}
		candidate = filepath.Clean(candidate)
		relative, err := filepath.Rel(root, candidate)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("changed file %q is outside the project", item)
		}
		files = append(files, candidate)
	}
	return files, nil
}

func failed(tool string, err error) workflow.Result {
	return workflow.Result{Status: "failed", Tool: tool, Message: err.Error()}
}
