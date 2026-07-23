package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }
func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type initManifest struct {
	Entry        string           `json:"entry"`
	Preset       string           `json:"preset"`
	Experimental initExperimental `json:"experimental"`
	PawnKit      initPawnKit      `json:"pawnkit"`
}

type initExperimental struct {
	BuildFile bool `json:"build_file"`
}

type initPawnKit struct {
	SchemaVersion int      `json:"schemaVersion"`
	Profile       string   `json:"profile"`
	IncludePaths  []string `json:"includePaths,omitempty"`
}

func runInit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectDir := flags.String("project", ".", "project directory")
	entry := flags.String("entry", "", "project entry file")
	target := flags.String("target", "openmp", "openmp or samp")
	dryRun := flags.Bool("dry-run", false, "print the manifest without writing it")
	var includes stringList
	flags.Var(&includes, "include", "include directory (repeatable)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitUsage
	}
	if *target != "openmp" && *target != "samp" {
		_, _ = fmt.Fprintln(stderr, "pawn init: target must be openmp or samp")
		return ExitUsage
	}
	if err := ctx.Err(); err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn init:", err)
		return ExitInternal
	}
	root, err := filepath.Abs(*projectDir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn init:", err)
		return ExitInternal
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		_, _ = fmt.Fprintf(stderr, "pawn init: project directory is unavailable: %s\n", root)
		return ExitUsage
	}
	for _, name := range []string{"pawn.json", "pawn.yaml", "pawn.yml"} {
		if _, statErr := os.Stat(filepath.Join(root, name)); statErr == nil {
			_, _ = fmt.Fprintf(stderr, "pawn init: %s already exists\n", name)
			return ExitUsage
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			_, _ = fmt.Fprintln(stderr, "pawn init:", statErr)
			return ExitInternal
		}
	}
	resolvedEntry, err := initEntry(root, *entry)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn init:", err)
		return ExitUsage
	}
	cleanIncludes, err := initIncludes(root, includes)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn init:", err)
		return ExitUsage
	}
	manifest := initManifest{
		Entry: resolvedEntry, Preset: *target,
		Experimental: initExperimental{BuildFile: false},
		PawnKit:      initPawnKit{SchemaVersion: 1, Profile: *target, IncludePaths: cleanIncludes},
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ExitInternal
	}
	content = append(content, '\n')
	if *dryRun {
		_, err = stdout.Write(content)
	} else {
		err = writeManifest(filepath.Join(root, "pawn.json"), content)
		if err == nil {
			_, err = fmt.Fprintf(stdout, "pawn init: wrote %s\n", filepath.Join(root, "pawn.json"))
		}
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "pawn init:", err)
		return ExitInternal
	}
	return ExitOK
}

func initEntry(root, value string) (string, error) {
	if value == "" {
		candidates, err := findPawnEntries(root)
		if err != nil {
			return "", err
		}
		if len(candidates) != 1 {
			return "", fmt.Errorf("found %d possible entry files; pass --entry", len(candidates))
		}
		return candidates[0], nil
	}
	entry, err := projectRelativePath(root, value)
	if err != nil {
		return "", fmt.Errorf("entry: %w", err)
	}
	if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(entry))); statErr != nil || info.IsDir() {
		return "", fmt.Errorf("entry does not exist: %s", value)
	}
	if strings.ToLower(filepath.Ext(entry)) != ".pwn" {
		return "", fmt.Errorf("entry must be a .pwn file: %s", value)
	}
	return entry, nil
}

func findPawnEntries(root string) ([]string, error) {
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != root {
			switch entry.Name() {
			case ".git", "dependencies", "vendor", "generated", "node_modules":
				return filepath.SkipDir
			}
		}
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(entry.Name()), ".pwn") {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			entries = append(entries, filepath.ToSlash(relative))
			if len(entries) > 1000 {
				return errors.New("too many Pawn files to discover an entry")
			}
		}
		return nil
	})
	sort.Strings(entries)
	return entries, err
}

func initIncludes(root string, values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		relative, err := projectRelativePath(root, value)
		if err != nil {
			return nil, fmt.Errorf("include %q: %w", value, err)
		}
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("include directory does not exist: %s", value)
		}
		if _, exists := seen[relative]; !exists {
			seen[relative] = struct{}{}
			result = append(result, relative)
		}
	}
	return result, nil
}

func projectRelativePath(root, value string) (string, error) {
	path := value
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("path is outside the project")
	}
	return filepath.ToSlash(filepath.Clean(relative)), nil
}

func writeManifest(path string, content []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".pawn.json-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	if err = file.Chmod(0o644); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(temporary)
		}
	}()
	if _, err = file.Write(content); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		return err
	}
	ok = true
	return nil
}
