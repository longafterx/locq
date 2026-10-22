package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAnalyzeAnalyzeEmptyInput(t *testing.T) {
	for _, workers := range []int{0, 1, 2, 8} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			t.Parallel()
			opts := &Options{Workers: workers}
			result, err := Analyze(nil, opts)
			if err != nil {
				t.Fatal(err)
			}
			hasFiles := len(result.Files) != 0 || len(result.Languages) != 0
			if result.Total.Total != 0 || hasFiles {
				t.Fatalf("expected empty result: %+v", result)
			}
		})
	}
}

func TestAnalyzeAnalyzeWorkersProducesEquivalentResults(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	pythonFile := filepath.Join(dir, "script.py")
	if err := os.WriteFile(goFile, []byte("package sample\n// comment\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pythonFile, []byte("# comment\nprint(\"x\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, workers := range []int{0, 1, 2, 8} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			result := analyzeDirectory(t, dir, &Options{Workers: workers})
			assertAnalysisCounts(t, result.Languages, result.Files)
		})
	}
}

func TestAnalyzeAnalyzeWorkersPreservesDuplicateBehavior(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("nil options count both files", func(t *testing.T) {
		result := analyzeDirectory(t, dir, nil)
		validTotal := result.Total.Total == 2 && result.Total.Code == 2
		if !validTotal || len(result.Files) != 2 {
			t.Fatalf("nil options should count both files: %+v", result.Total)
		}
	})

	for _, skip := range []bool{false, true} {
		wantPaths := []string{filepath.Join(dir, "a.go")}
		if skip {
			wantPaths = append(wantPaths, filepath.Join(dir, "b.go"))
		}
		for _, workers := range []int{0, 1, 2, 8} {
			t.Run(fmt.Sprintf("workers=%d/skip=%t", workers, skip), func(t *testing.T) {
				result := analyzeDirectory(t, dir, &Options{Workers: workers, Dedup: !skip})
				wantCount := int32(len(wantPaths))
				if result.Total.Total != wantCount || result.Total.Code != wantCount {
					t.Fatalf("total=%+v, want %d files and code lines", result.Total, wantCount)
				}
				if got := result.Languages["Go"].Files; !reflect.DeepEqual(got, wantPaths) {
					t.Fatalf("retained files=%q, want %q", got, wantPaths)
				}
				if len(result.Files) != len(wantPaths) {
					t.Fatalf("analyzed file count=%d, want %d", len(result.Files), len(wantPaths))
				}
				for _, path := range wantPaths {
					if result.Files[path] == nil || result.Files[path].Code != 1 {
						t.Fatalf("file %q was not counted correctly: %+v", path, result.Files[path])
					}
				}
			})
		}
	}
}

func analyzeDirectory(t *testing.T, dir string, opts *Options) *Result {
	t.Helper()
	result, err := Analyze([]string{dir}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertAnalysisCounts(t *testing.T, languages map[string]*Language, files map[string]*ClocFile) {
	t.Helper()
	if len(files) != 2 {
		t.Fatalf("analyzed file count = %d, want 2", len(files))
	}
	if got := languages["Go"]; got.Code != 2 || got.Comments != 1 || got.Blanks != 1 {
		t.Fatalf("Go counts = (%d, %d, %d), want (2, 1, 1)", got.Code, got.Comments, got.Blanks)
	}
	if got := languages["Python"]; got.Code != 1 || got.Comments != 1 || got.Blanks != 0 {
		t.Fatalf("Python counts = (%d, %d, %d), want (1, 1, 0)", got.Code, got.Comments, got.Blanks)
	}
}
