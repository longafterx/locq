package locq_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/iotest"

	"github.com/longafterx/locq"
)

func TestAnalyzeDefaultsAndDedup(t *testing.T) {
	t.Parallel()
	dir := writeAPIFixture(t, map[string]string{
		"a.go": "package sample\n// comment\n\n", "b.go": "package sample\n// comment\n\n",
	})
	for _, tc := range []struct {
		name  string
		opts  *locq.Options
		files int32
	}{
		{name: "nil", files: 2},
		{name: "zero value", opts: &locq.Options{}, files: 2},
		{name: "single worker", opts: &locq.Options{Workers: 1}, files: 2},
		{name: "maximum workers", opts: &locq.Options{Workers: locq.MaxWorkers}, files: 2},
		{name: "dedup single worker", opts: &locq.Options{Workers: 1, Dedup: true}, files: 1},
		{name: "dedup automatic", opts: &locq.Options{Dedup: true}, files: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := locq.Analyze([]string{dir}, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			total := result.Total
			counts := []int32{total.Total, total.Code, total.Comments, total.Blanks}
			if !reflect.DeepEqual(counts, []int32{tc.files, tc.files, tc.files, tc.files}) {
				t.Fatalf("total=%+v, want %d files and one line of each kind per file", total, tc.files)
			}
			if len(result.Files) != int(tc.files) || result.Files[filepath.Join(dir, "a.go")] == nil {
				t.Fatalf("unexpected retained files: %v", result.Files)
			}
		})
	}
}

func TestAnalyzeEmptyPaths(t *testing.T) {
	t.Parallel()
	result, err := locq.Analyze(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	hasFiles := len(result.Files) != 0 || len(result.Languages) != 0
	if result.Total.Total != 0 || hasFiles {
		t.Fatalf("empty paths must not scan the current directory: %+v", result)
	}
}

func TestAnalyzeGlobRoots(t *testing.T) {
	t.Parallel()
	dir := writeAPIFixture(t, map[string]string{
		"target/generated.go":        "package sample\n",
		"src/target/generated.go":    "package sample\n",
		"src/generated/generated.go": "package sample\n",
		"tests/generated/test.go":    "package sample\n",
		"src/main.go":                "package sample\n",
	})
	for _, tc := range []struct {
		name    string
		paths   []string
		pattern string
		want    []string
	}{
		{
			name: "anchored directory", paths: []string{dir}, pattern: "/target/",
			want: []string{"src/generated/generated.go", "src/main.go", "src/target/generated.go", "tests/generated/test.go"},
		},
		{
			name: "independent roots", paths: []string{filepath.Join(dir, "src"), filepath.Join(dir, "tests")},
			pattern: "/generated/", want: []string{"src/main.go", "src/target/generated.go"},
		},
		{
			name: "root directory name", paths: []string{filepath.Join(dir, "target")},
			pattern: "target/", want: []string{},
		},
		{
			name: "dot root name", paths: []string{filepath.Join(dir, "target") + string(os.PathSeparator) + "."},
			pattern: "target/", want: []string{},
		},
		{
			name: "explicit file parent", paths: []string{filepath.Join(dir, "target/generated.go")},
			pattern: "target/", want: []string{},
		},
		{
			name: "explicit filename", paths: []string{filepath.Join(dir, "src/main.go")},
			pattern: "/main.go", want: []string{},
		},
		{
			name: "outside ancestors ignored", paths: []string{filepath.Join(dir, "src/target")},
			pattern: "src/", want: []string{"src/target/generated.go"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			relativePaths := make([]string, 0, len(tc.paths))
			inputs := [][]string{tc.paths}
			for _, input := range tc.paths {
				relative, err := filepath.Rel(cwd, input)
				if err != nil {
					// Windows temporary directories may be on another volume.
					t.Logf("relative input unavailable: %v", err)
					break
				}
				relativePaths = append(relativePaths, relative)
			}
			if len(relativePaths) == len(tc.paths) {
				inputs = append(inputs, relativePaths)
			}
			for _, paths := range inputs {
				result, err := locq.Analyze(paths, &locq.Options{Exclude: []string{tc.pattern}})
				if err != nil {
					t.Fatal(err)
				}
				if got := apiRelativeFiles(t, dir, result); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("files=%q, want %q", got, tc.want)
				}
			}
		})
	}
}

func apiRelativeFiles(t *testing.T, root string, result *locq.Result) []string {
	t.Helper()
	files := make([]string, 0, len(result.Files))
	for path := range result.Files {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(root, absolute)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, filepath.ToSlash(rel))
	}
	sort.Strings(files)
	return files
}

func TestAnalyzeOptionsCanBeReusedConcurrently(t *testing.T) {
	t.Parallel()
	dir := writeAPIFixture(t, map[string]string{"main.go": "package main\n", "main.py": "print(1)\n"})
	opts := &locq.Options{
		IncludeLangs: []string{"Go"}, ExcludeLangs: []string{"Plain Text"}, Include: []string{"*.go"},
	}
	want := *opts
	want.IncludeLangs = append([]string{}, opts.IncludeLangs...)
	want.ExcludeLangs = append([]string{}, opts.ExcludeLangs...)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := locq.Analyze([]string{dir}, opts)
			if err != nil {
				t.Error(err)
				return
			}
			if result.Total.Total != 1 || result.Languages["Go"] == nil {
				t.Errorf("unexpected result: %+v", result)
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(*opts, want) {
		t.Fatalf("caller options mutated: %+v", opts)
	}
}

func TestAnalyzeDiagnosticFailure(t *testing.T) {
	t.Parallel()
	_, err := locq.Analyze([]string{filepath.Join(t.TempDir(), "missing")}, &locq.Options{
		Diagnostics: failingWriter{},
	})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error=%v, want diagnostic writer error", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func writeAPIFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAnalyzeGlobSelections(t *testing.T) {
	t.Parallel()
	root := writeAPIFixture(t, map[string]string{
		"main.go": "package main\n", "target/a.go": "package generated\n", "src/target/b.go": "package generated\n",
		"gcc.target/a.go": "package keep\n", "distribution/a.go": "package keep\n", "my-dist/a.go": "package keep\n",
		"src/main.go": "package main\n", "src/skip_test.go": "package test\n", "src/nested/mod.py": "print(1)\n",
		"tests/main.go": "package test\n", "scripts/run.py": "print(1)\n", ".hidden/main.go": "package hidden\n",
	})
	for _, tc := range []struct {
		name string
		opts locq.Options
		want []string
	}{
		{
			name: "exact excluded directories",
			opts: locq.Options{Exclude: []string{"{dist,node_modules,target}/", "*_test.go"}},
			want: []string{
				".hidden/main.go",
				"distribution/a.go",
				"gcc.target/a.go",
				"main.go",
				"my-dist/a.go",
				"scripts/run.py",
				"src/main.go",
				"src/nested/mod.py",
				"tests/main.go",
			},
		},
		{
			name: "nested inclusion remains reachable",
			opts: locq.Options{Include: []string{"nested/"}},
			want: []string{"src/nested/mod.py"},
		},
		{
			name: "directory descendants",
			opts: locq.Options{Include: []string{"/src/"}, Exclude: []string{"target/", "*_test.go"}},
			want: []string{"src/main.go", "src/nested/mod.py"},
		},
		{
			name: "include patterns are ORed",
			opts: locq.Options{Include: []string{"/src/main.go", "scripts/"}},
			want: []string{"scripts/run.py", "src/main.go"},
		},
		{
			name: "file and directory same pattern",
			opts: locq.Options{Include: []string{"main.go"}},
			want: []string{".hidden/main.go", "main.go", "src/main.go", "tests/main.go"},
		},
		{name: "anchored file", opts: locq.Options{Include: []string{"/main.go"}}, want: []string{"main.go"}},
		{name: "exclusion wins", opts: locq.Options{Include: []string{"src/"}, Exclude: []string{"src/"}}, want: []string{}},
		{
			name: "language inclusion",
			opts: locq.Options{IncludeLangs: []string{"Python"}},
			want: []string{"scripts/run.py", "src/nested/mod.py"},
		},
		{
			name: "language exclusion wins",
			opts: locq.Options{IncludeLangs: []string{"Go"}, ExcludeLangs: []string{"Go"}},
			want: []string{},
		},
		{
			name: "suffix exclusion is literal glob",
			opts: locq.Options{Include: []string{"src/"}, Exclude: []string{"*.go"}},
			want: []string{"src/nested/mod.py"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, workers := range []int{1, 4} {
				tc.opts.Workers = workers
				result, err := locq.Analyze([]string{root}, &tc.opts)
				if err != nil {
					t.Fatal(err)
				}
				if got := apiRelativeFiles(t, root, result); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("files=%q want=%q", got, tc.want)
				}
			}
		})
	}
}

func TestAnalyzeIncludeRoots(t *testing.T) {
	root := writeAPIFixture(t, map[string]string{"src/main.go": "package main\n", "tests/main.go": "package test\n"})
	for _, tc := range []struct {
		paths   []string
		include []string
		want    []string
	}{
		{[]string{filepath.Join(root, "src")}, []string{"src/"}, []string{"src/main.go"}},
		{[]string{filepath.Join(root, "src/main.go")}, []string{"src/"}, []string{"src/main.go"}},
		{[]string{filepath.Join(root, "src/main.go")}, []string{"/main.go"}, []string{"src/main.go"}},
		{[]string{filepath.Join(root, "src/main.go")}, []string{"/src/"}, []string{}},
		{
			[]string{filepath.Join(root, "src"), filepath.Join(root, "tests")},
			[]string{"/main.go"},
			[]string{"src/main.go", "tests/main.go"},
		},
	} {
		result, err := locq.Analyze(tc.paths, &locq.Options{Include: tc.include})
		if err != nil {
			t.Fatal(err)
		}
		if got := apiRelativeFiles(t, root, result); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf(
				"include=%q paths=%q files=%q want=%q",
				tc.include,
				tc.paths,
				got,
				tc.want,
			)
		}
	}
}

func TestOptionsValidationPrecedesIO(t *testing.T) {
	for _, tc := range []struct {
		opts  locq.Options
		field string
	}{
		{locq.Options{Workers: -1}, "Workers"}, {locq.Options{Workers: 65}, "Workers"},
		{locq.Options{Include: []string{"["}}, "Include"}, {locq.Options{Exclude: []string{"["}}, "Exclude"},
		{locq.Options{Exclude: []string{""}}, "Exclude"}, {locq.Options{Include: []string{"../*"}}, "Include"},
		{locq.Options{IncludeLangs: []string{"typo"}}, "IncludeLangs"}, {locq.Options{ExcludeLangs: []string{"typo"}}, "ExcludeLangs"},
		{locq.Options{Definitions: map[string]*locq.LanguageDefinition{"broken": nil}}, "Definitions"},
		{locq.Options{Extensions: map[string]string{"custom": "unknown"}}, "Extensions"},
	} {
		var output bytes.Buffer
		tc.opts.Diagnostics = &output
		result, err := locq.Analyze([]string{"nonexistent"}, &tc.opts)
		var optionErr *locq.OptionError
		if result != nil || !errors.As(err, &optionErr) || optionErr.Field != tc.field || output.Len() != 0 {
			t.Fatalf(
				"result=%v error=%v diagnostics=%q",
				result,
				err,
				output.String(),
			)
		}
	}
}

func TestPerCallLanguageConfiguration(t *testing.T) {
	root := writeAPIFixture(t, map[string]string{"a.custom": "# custom\ncode\n", "a.go": "// built-in\npackage main\n"})
	comments := []string{"#"}
	blocks := [][]string{{"{-", "-}"}}
	custom := locq.NewLanguageDefinition("Custom", comments, blocks)
	comments[0] = "!"
	blocks[0][0] = "!"
	opts := &locq.Options{
		Definitions: map[string]*locq.LanguageDefinition{"Custom": custom},
		Extensions:  map[string]string{"custom": "Custom"},
	}
	result, err := locq.Analyze([]string{root}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total.Total != 2 || result.Languages["Custom"].Comments != 1 || result.Languages["Go"].Comments != 1 {
		t.Fatalf("result=%+v", result)
	}
	defaults, err := locq.Analyze([]string{root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Total.Total != 1 || defaults.Languages["Custom"] != nil {
		t.Fatal("custom rules leaked into next call")
	}
	// Two independent callers can map the same suffix differently.
	var wg sync.WaitGroup
	for _, language := range []string{"Go", "Python"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := locq.Analyze(
				[]string{filepath.Join(root, "a.custom")},
				&locq.Options{Extensions: map[string]string{"custom": language}},
			)
			if err != nil || res.Languages[language] == nil {
				t.Errorf(
					"language=%s result=%v err=%v",
					language,
					res,
					err,
				)
			}
		}()
	}
	wg.Wait()
}

func TestReaderEventsAndErrors(t *testing.T) {
	language := locq.NewDefinedLanguages().Langs["Go"]
	source := "\xef\xbb\xbfpackage main\r\n  // comment  \n\nlast"
	var events []locq.LineEvent
	result, err := locq.AnalyzeReader(
		"sample.go",
		language,
		iotest.OneByteReader(strings.NewReader(source)),
		&locq.FileOptions{OnLine: func(e locq.LineEvent) { events = append(events, e) }},
	)
	want := []locq.LineEvent{
		{Number: 1, Kind: locq.CodeLine, Text: "package main"},
		{Number: 2, Kind: locq.CommentLine, Text: "// comment"},
		{Number: 3, Kind: locq.BlankLine, Text: ""},
		{Number: 4, Kind: locq.CodeLine, Text: "last"},
	}
	if err != nil || result.Code != 2 || !reflect.DeepEqual(events, want) {
		t.Fatalf(
			"result=%+v events=%+v err=%v",
			result,
			events,
			err,
		)
	}
	failure := errors.New("read failed")
	events = nil
	result, err = locq.AnalyzeReader(
		"bad.go",
		language,
		io.MultiReader(strings.NewReader("package main\npartial"), iotest.ErrReader(failure)),
		&locq.FileOptions{OnLine: func(e locq.LineEvent) { events = append(events, e) }},
	)
	if !errors.Is(err, failure) || result.Code != 1 || len(events) != 1 {
		t.Fatalf(
			"partial result=%+v events=%+v err=%v",
			result,
			events,
			err,
		)
	}
	_, err = locq.AnalyzeReader(
		"sample.go",
		language,
		strings.NewReader(source),
		&locq.FileOptions{Debug: true, Diagnostics: failingWriter{}},
	)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("diagnostics error=%v", err)
	}
	for _, definition := range []*locq.LanguageDefinition{
		nil,
		locq.NewLanguageDefinition("", nil, nil),
		locq.NewLanguageDefinition("bad", nil, [][]string{{"/*"}}),
	} {
		_, err = locq.AnalyzeReader(
			"sample",
			definition,
			strings.NewReader("x"),
			nil,
		)
		if err == nil {
			t.Fatalf("invalid definition accepted: %+v", definition)
		}
	}
}
