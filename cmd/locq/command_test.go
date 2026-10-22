package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"

	"github.com/longafterx/locq"
	"github.com/spf13/cobra"
)

func TestAutomaticWorkersSupportsLargeRuntimeLimit(t *testing.T) {
	previous := runtime.GOMAXPROCS(locq.MaxWorkers + 1)
	defer runtime.GOMAXPROCS(previous)

	command := newRootCommand()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"-o", "json", t.TempDir()})
	if err := command.Execute(); err != nil {
		t.Fatalf("automatic selection treated as an excessive explicit count: %v", err)
	}
	if stderr.Len() != 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout=%q stderr=%q, want JSON without diagnostics", stdout.String(), stderr.String())
	}
}

func TestNegativeWorkerOptionIsRejectedBeforeVersionExit(t *testing.T) {
	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"--workers=-1", "--version"})
	err := command.Execute()
	if err == nil {
		t.Fatal("expected worker validation error")
	}
	if !strings.Contains(err.Error(), "--workers must be zero (automatic) or between 1 and 64") {
		t.Fatalf("error = %q, want worker validation error", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected output before handling error: %q", output.String())
	}
}

func TestCmdOptionsDefaultWorkersUsesAutomaticSelection(t *testing.T) {
	command := newRootCommand()
	if err := command.ParseFlags([]string{"."}); err != nil {
		t.Fatal(err)
	}
	if command.Flags().Changed("workers") {
		t.Fatal("workers must remain omitted for automatic selection")
	}
}

func TestRootCommandParsing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		paths   []string
		workers int
	}{
		{name: "equals", args: []string{"--workers=8", "src", "test"}, paths: []string{"src", "test"}, workers: 8},
		{name: "decimal leading zero", args: []string{"--workers=08", "src"}, paths: []string{"src"}, workers: 8},
		{name: "space", args: []string{"--workers", "4", "src"}, paths: []string{"src"}, workers: 4},
		{name: "short space", args: []string{"-w", "8", "src"}, paths: []string{"src"}, workers: 8},
		{name: "short equals", args: []string{"-w=8", "src"}, paths: []string{"src"}, workers: 8},
		{name: "short attached", args: []string{"-w08", "src"}, paths: []string{"src"}, workers: 8},
		{name: "mixed last value", args: []string{"--workers=4", "src", "-w8"}, paths: []string{"src"}, workers: 8},
		{name: "interspersed", args: []string{"src", "--workers=2", "test"}, paths: []string{"src", "test"}, workers: 2},
		{name: "terminator", args: []string{"--workers=1", "--", "--version"}, paths: []string{"--version"}, workers: 1},
		{name: "command-like paths", args: []string{"help", "completion"}, paths: []string{"help", "completion"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newRootCommand()
			command.SetArgs(tc.args)
			called := false
			command.RunE = func(cmd *cobra.Command, paths []string) error {
				called = true
				if !reflect.DeepEqual(paths, tc.paths) {
					t.Fatalf("paths = %v, want %v", paths, tc.paths)
				}
				workers, err := cmd.Flags().GetInt("workers")
				if err != nil {
					t.Fatal(err)
				}
				if workers != tc.workers {
					t.Fatalf("workers = %d, want %d", workers, tc.workers)
				}
				return nil
			}
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("path was treated as a subcommand")
			}
		})
	}
}

func TestRootCommandErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown flag", args: []string{"--unknown"}, want: "unknown flag"},
		{
			name: "conflicting dedup options",
			args: []string{"--dedup", "--skip-duplicated", "."},
			want: "unknown flag: --skip-duplicated",
		},
		{
			name: "conflicting explicit false options",
			args: []string{"--skip-duplicated=false", "--dedup=false", "."},
			want: "unknown flag: --skip-duplicated",
		},
		{name: "invalid dedup boolean", args: []string{"--dedup=invalid", "."}, want: "invalid argument"},
		{name: "missing value", args: []string{"--workers"}, want: "needs an argument"},
		{name: "invalid integer", args: []string{"--workers=many"}, want: "invalid argument"},
		{name: "hexadecimal integer", args: []string{"--workers=0x8"}, want: "invalid argument"},
		{name: "too many workers", args: []string{"--workers=65", "--show-lang"}, want: "zero (automatic) or between 1 and 64"},
		{name: "short negative workers", args: []string{"-w", "-1", "-V"}, want: "zero (automatic) or between 1 and 64"},
		{name: "short excessive workers", args: []string{"-w65", "-L"}, want: "zero (automatic) or between 1 and 64"},
		{name: "short missing value", args: []string{"-w"}, want: "needs an argument"},
		{name: "sort before version", args: []string{"--sort=invalid", "--version"}, want: "must be one of"},
		{name: "invalid repeated sort", args: []string{"--sort=invalid", "--sort=name", "."}, want: "must be one of"},
		{name: "sort conflict", args: []string{"--by-file", "--sort=files", "."}, want: "cannot be used"},
		{name: "short sort conflict", args: []string{"-f", "-s", "files", "."}, want: "cannot be used"},
		{name: "short sort before version", args: []string{"-sunknown", "-V"}, want: "must be one of"},
		{name: "match glob", args: []string{"--include=[", "."}, want: "invalid --include:"},
		{name: "not-match glob", args: []string{"--exclude=[", "."}, want: "invalid --exclude:"},
		{name: "match-d glob", args: []string{"--include=[", "."}, want: "invalid --include:"},
		{name: "not-match-d glob", args: []string{"--exclude=[", "."}, want: "invalid --exclude:"},
		{name: "removed fullpath", args: []string{"--fullpath", "."}, want: "unknown flag: --fullpath"},
		{name: "exclude glob", args: []string{"--exclude=[", "."}, want: "invalid --exclude: pattern 1"},
		{name: "empty glob", args: []string{"--exclude=", "."}, want: "invalid --exclude: pattern 1"},
		{
			name: "invalid second glob",
			args: []string{"--exclude=valid/", "--exclude={dist,target", "."},
			want: "invalid --exclude: pattern 2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newRootCommand()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(tc.args)
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if output.Len() != 0 {
				t.Fatalf("error must be returned without printing: %q", output.String())
			}
		})
	}
}

func TestRootCommandDeduplication(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, count := range []int{1, 8} {
		t.Run("workers="+strconv.Itoa(count), func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				args  []string
				files int32
			}{
				{name: "default counts copies", args: []string{}, files: 2},
				{name: "dedup", args: []string{"--dedup"}, files: 1},
				{name: "dedup true", args: []string{"--dedup=true"}, files: 1},
				{name: "dedup false", args: []string{"--dedup=false"}, files: 2},
				{
					name:  "last dedup value wins",
					args:  []string{"--dedup", "--dedup=false"},
					files: 2,
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					command := newRootCommand()
					var stdout, stderr bytes.Buffer
					command.SetOut(&stdout)
					command.SetErr(&stderr)
					args := []string{"--output-type=json", "--workers=" + strconv.Itoa(count), dir}
					command.SetArgs(append(args, tc.args...))
					if err := command.Execute(); err != nil {
						t.Fatal(err)
					}
					if stderr.Len() != 0 {
						t.Fatalf("unexpected diagnostics: %s", stderr.String())
					}
					var result locq.JSONLanguagesResult
					if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if result.Total.FilesCount != tc.files || result.Total.Code != tc.files {
						t.Fatalf("total = %+v, want %d files and code lines", result.Total, tc.files)
					}
				})
			}
		})
	}
}

func TestRootCommandInformation(t *testing.T) {
	info, _ := debug.ReadBuildInfo()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "no paths", args: []string{}, want: "Usage:"},
		{name: "help", args: []string{"--help"}, want: "--workers"},
		{name: "short help", args: []string{"-h"}, want: "Usage:"},
		{name: "version", args: []string{"--version"}, want: versionString(Version, GitCommit, info)},
		{name: "short version", args: []string{"-V"}, want: versionString(Version, GitCommit, info)},
		{name: "languages", args: []string{"--show-lang"}, want: "Go"},
		{name: "short languages", args: []string{"-L"}, want: "Go"},
		{name: "help ignores glob syntax", args: []string{"--exclude=[", "--help"}, want: "Usage:"},
		{name: "languages ignore glob syntax", args: []string{"--exclude=[", "--show-lang"}, want: "Go"},
		{name: "automatic worker", args: []string{"--workers=0", "--version"}, want: versionString(Version, GitCommit, info)},
		{name: "one worker", args: []string{"--workers=1", "--version"}, want: versionString(Version, GitCommit, info)},
		{name: "64 workers", args: []string{"--workers=64", "--version"}, want: versionString(Version, GitCommit, info)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newRootCommand()
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs(tc.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), tc.want) || stderr.Len() != 0 {
				t.Fatalf("stdout = %q, stderr = %q, want %q", stdout.String(), stderr.String(), tc.want)
			}
		})
	}
}

func TestRootCommandHelpExamples(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "long help", args: []string{"--help"}},
		{name: "short help", args: []string{"-h"}},
		{name: "no arguments", args: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newRootCommand()
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs(tc.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected diagnostics: %s", stderr.String())
			}
			for _, obsolete := range []string{"(regex)", "substring match", "--fullpath"} {
				if strings.Contains(stdout.String(), obsolete) {
					t.Errorf("help still advertises %q", obsolete)
				}
			}
			previousSection := -1
			for _, section := range []string{"Usage:", "Flags:", "Examples:"} {
				position := strings.Index(stdout.String(), section)
				if position <= previousSection {
					t.Fatalf("help sections must appear in order: Usage, Flags, Examples; got %q", stdout.String())
				}
				previousSection = position
			}
			if !strings.HasSuffix(stdout.String(), "  locq --dedup .\n") {
				t.Fatal("examples must be the last help section")
			}
			for _, want := range []string{
				"Examples:",
				"  locq .\n",
				"  locq src tests\n",
				"  locq --exclude '{dist,node_modules,target}/' .\n",
				"  locq --include '*.go' --exclude '*_test.go' .\n",
				"  locq --include 'src/' --exclude '{dist,node_modules,target}/' .\n",
				"  locq -l Go,Python .\n",
				"  locq -f -s code .\n",
				"  locq -o json . > ../counts.json\n",
				"  locq --dedup .\n",
			} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("help output is missing %q", want)
				}
			}
		})
	}
}
