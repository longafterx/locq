package core

import (
	"bytes"
	"crypto/md5"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type failingSourceReader struct{ err error }

func (r failingSourceReader) Read([]byte) (int, error) { return 0, r.err }

func TestAnalyzeReaderDebugRecords(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    []string
	}{
		{name: "empty"},
		{
			name:    "line numbers, comment state and escaped content",
			content: "package main\r\n// note\t\x1b[31m\n\n/* start\nmiddle\n*/\nvar x = 1",
			want: []string{
				`[CODE] file=FILE line=1 code=1 comment=0 blank=0 in_comment=false text="package main\r\n"`,
				`[COMM] file=FILE line=2 code=1 comment=1 blank=0 in_comment=false text="// note\t\x1b[31m\n"`,
				`[BLNK] file=FILE line=3 code=1 comment=1 blank=1 in_comment=false text="\n"`,
				`[COMM] file=FILE line=4 code=1 comment=2 blank=1 in_comment=true text="/* start\n"`,
				`[COMM] file=FILE line=5 code=1 comment=3 blank=1 in_comment=true text="middle\n"`,
				`[COMM] file=FILE line=6 code=1 comment=4 blank=1 in_comment=false text="*/\n"`,
				`[CODE] file=FILE line=7 code=2 comment=4 blank=1 in_comment=false text="var x = 1"`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			filename := "dir/\"file\n.go"
			var diagnostics bytes.Buffer
			opts := &FileOptions{Debug: true, Diagnostics: &diagnostics}
			AnalyzeReader(
				filename,
				NewDefinedLanguages().Langs["Go"],
				strings.NewReader(tc.content),
				opts,
			)
			want := "[FILE] file=" + strconv.Quote(filename) + "\n"
			for _, record := range tc.want {
				want += strings.ReplaceAll(record, "file=FILE", "file="+strconv.Quote(filename)) + "\n"
			}
			if got := diagnostics.String(); got != want {
				t.Fatalf("debug records:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestAnalyzeReaderReportsReadFailure(t *testing.T) {
	failure := errors.New("injected read failure")
	reader := io.MultiReader(strings.NewReader("package main\n"), failingSourceReader{err: failure})
	result, err := AnalyzeReader("sample.go", NewDefinedLanguages().Langs["Go"], reader, nil)
	if result.Code != 1 || !errors.Is(err, failure) || !strings.Contains(err.Error(), "sample.go") {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestAnalyzeFileReportsOpenFailure(t *testing.T) {
	result, err := AnalyzeFile(filepath.Join(t.TempDir(), "missing.go"), NewDefinedLanguages().Langs["Go"], nil)
	if result != nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestDetectAndAnalyzeFileDistinguishesFailureFromExclusion(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "unknown.xyz")
	if err := os.WriteFile(unknown, []byte("unrecognized\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	languages := NewDefinedLanguages()
	reader := newLineReader(nil)
	for _, skip := range []bool{false, true} {
		opts := testRunState(t, &Options{Dedup: !skip})
		missing := detectAndAnalyzeFile(fileCandidate{path: filepath.Join(dir, "missing.go")}, languages, opts, reader)
		if !errors.Is(missing.err, os.ErrNotExist) {
			t.Fatalf("missing file error=%v", missing.err)
		}
		ignored := detectAndAnalyzeFile(fileCandidate{path: unknown}, languages, opts, reader)
		if ignored.err != nil || !ignored.ignored {
			t.Fatalf("unknown file must be an exclusion, not an error: %+v", ignored)
		}
	}
}

func TestDetectAndAnalyzeFile(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "plain.go", content: "package main\n// comment\n\n"},
		{name: "override.go", content: "#!/usr/bin/env python\n# comment\nprint(1)\n"},
		{name: "script", content: "#!/bin/sh\necho hello\n"},
		{name: "unterminated.go", content: "#!/usr/bin/env python"},
		{name: "empty.go"},
		{name: "long.go", content: "//" + strings.Repeat("x", 20000) + "\npackage main"},
		{name: "ambiguous.ts", content: "export const value: number = 1;\n"},
		{name: "actor.mo", content: "actor { public query func greet() : async Text { \"hello\" } };\n"},
		{name: "Makefile", content: "all:\n\techo hello\n"},
		{name: "unknown.xyz", content: "hello\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			opts := testRunState(t, nil)
			opts.runConfig.Dedup = true
			langs := NewDefinedLanguages()
			reader := newLineReader(nil)
			ext, recognized := detectFileType(path, opts, func(all bool) ([]byte, error) {
				content := []byte(tc.content)
				if all {
					return content, nil
				}
				if end := bytes.IndexByte(content, '\n'); end >= 0 {
					return content[:end+1], nil
				}
				return content, io.EOF
			})
			key, known := builtInExtensions[ext]
			got := detectAndAnalyzeFile(fileCandidate{path: path}, langs, opts, reader)
			if !recognized || !known {
				if !got.ignored {
					t.Fatal("unknown language was accepted")
				}
				return
			}
			want, readErr := AnalyzeFile(path, langs.Langs[key], nil)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got.ignored || got.languageKey != key || !reflect.DeepEqual(got.clocFile, want) {
				t.Fatalf("combined result = %+v (%+v), want %s %+v", got, got.clocFile, key, want)
			}
			if got.digest != md5.Sum([]byte(tc.content)) {
				t.Fatal("digest must include the detection prefix exactly once")
			}
			opts.runConfig.Dedup = false
			withoutHash := detectAndAnalyzeFile(fileCandidate{path: path}, langs, opts, reader)
			if withoutHash.ignored || !reflect.DeepEqual(withoutHash.clocFile, want) {
				t.Fatalf("non-dedup result = %+v, want %+v", withoutHash.clocFile, want)
			}
			if withoutHash.digest != [md5.Size]byte{} {
				t.Fatal("non-dedup analysis must not compute a digest")
			}
			opts.ExcludeLangs[key] = struct{}{}
			if !detectAndAnalyzeFile(fileCandidate{path: path}, langs, opts, reader).ignored {
				t.Fatal("excluded language was accepted")
			}
			delete(opts.ExcludeLangs, key)
			opts.IncludeLangs["not-a-language"] = struct{}{}
			if !detectAndAnalyzeFile(fileCandidate{path: path}, langs, opts, reader).ignored {
				t.Fatal("language outside include filter was accepted")
			}
		})
	}
}

func TestDetectAndAnalyzeFileAmbiguousLanguages(t *testing.T) {
	for _, tt := range []struct {
		name     string
		filename string
		content  string
		language string
	}{
		{
			name: "TypeScript", filename: "sample.ts", language: "TypeScript",
			content: "export const value: number = 1;\n",
		},
		{
			name: "Objective-C", filename: "sample.m", language: "Objective-C",
			content: "#import <Foundation/Foundation.h>\n@implementation Example\n@end\n",
		},
		{
			name: "MATLAB", filename: "sample.m", language: "MATLAB",
			content: "function y = square(x)\ny = x.^2;\nend\n",
		},
		{
			name: "Verilog", filename: "sample.v", language: "Verilog",
			content: "module example(input wire clk, output reg q);\nalways @(posedge clk) q <= 1'b1;\nendmodule\n",
		},
		{
			name: "FSharp", filename: "sample.fs", language: "F#",
			content: "module Example\nlet square x = x * x\nprintfn \"%d\" (square 2)\n",
		},
		{
			name: "R", filename: "sample.r", language: "R",
			content: "square <- function(x) { x ^ 2 }\nprint(square(2))\n",
		},
		{
			name: "Motoko", filename: "sample.mo", language: "Motoko",
			content: "actor { public query func greet() : async Text { \"hello\" } };\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			got := detectAndAnalyzeFile(
				fileCandidate{path: path},
				NewDefinedLanguages(),
				testRunState(t, nil),
				newLineReader(nil),
			)
			if got.err != nil || got.ignored || got.languageKey != tt.language {
				t.Fatalf("language=%q ignored=%t err=%v; want %q", got.languageKey, got.ignored, got.err, tt.language)
			}
		})
	}
}

func TestAnalyzeFile4Python(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.py")

	if err := os.WriteFile(tmpfile, []byte(`#!/bin/python

class A:
	"""comment1
	comment2
	comment3
	"""
	pass
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("Python", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 4 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 3 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Python" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4PythonInvalid(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.py")

	if err := os.WriteFile(tmpfile, []byte(`#!/bin/python

class A:
	"""comment1
	comment2
	comment3"""
	pass
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("Python", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 3 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 3 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Python" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4PythonNoShebang(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.py")

	if err := os.WriteFile(tmpfile, []byte(`#!/bin/python
	world
	'''

	b = 1
	"""hello
	commen
	"""

	print a, b
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("Python", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 2 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 3 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 5 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Python" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4Go(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.go")

	if err := os.WriteFile(tmpfile, []byte(`package main

func main() {
	var n string /*
		comment
		comment
	*/
}
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewDefinedLanguages().Langs["Go"]
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 3 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 4 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Go" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4GoWithOnelineBlockComment(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.go")

	if err := os.WriteFile(tmpfile, []byte(`package main

func main() {
	st := "/*"
	a := 1
	en := "*/"
	/* comment */
}
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewDefinedLanguages().Langs["Go"]
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 1 { // cloc->3, tokei->1, gocloc->4
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 6 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Go" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4GoWithCommentInnerBlockComment(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.go")

	if err := os.WriteFile(tmpfile, []byte(`package main

func main() {
	// comment /*
	a := 1
	b := 2
}
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewDefinedLanguages().Langs["Go"]
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 1 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 5 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Go" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4GoWithNoComment(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.go")

	if err := os.WriteFile(tmpfile, []byte(`package main

	func main() {
		a := "/*                */"
		b := "//                  "
	}
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewDefinedLanguages().Langs["Go"]
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 0 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 5 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Go" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4ATSWithDoubleMultilineComments(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.java")

	if err := os.WriteFile(tmpfile, []byte(`/* com */
(* co *)

vo (*
com *)

vo /*
jife */

vo /* ff */
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("ATS", []string{"//"}, [][]string{{"(*", "*)"}, {"/*", "*/"}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 3 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 4 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 3 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "ATS" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4JavaWithCommentInCodeLine(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.java")

	if err := os.WriteFile(tmpfile, []byte(`public class Sample {
		public static void main(String args[]){
		int a; /* A takes care of counts */
		int b;
		int c;
		String d; /*Just adding comments */
		bool e; /*
		comment*/
		bool f; /*
		comment1
		comment2
		*/
		/*End of Main*/
		}
		}
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("Java", []string{"//"}, [][]string{{"/*", "*/"}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 0 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 5 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 10 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Java" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4Makefile(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "Makefile.am")

	if err := os.WriteFile(tmpfile, []byte(`# This is a simple Makefile with comments
	.PHONY: test build

	build:
		mkdir -p bin
		GO111MODULE=on go build -o ./bin/gocloc cmd/gocloc/main.go

	# Another comment
	update-package:
		GO111MODULE=on go get -u github.com/longafterx/gocloc

	run-example:
		GO111MODULE=on go run examples/languages.go
		GO111MODULE=on go run examples/files.go

	test:
		GO111MODULE=on go test -v
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("Makefile", []string{"#"}, [][]string{{"", ""}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 4 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 2 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 11 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Makefile" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeReader(t *testing.T) {
	buf := bytes.NewBuffer([]byte(`#!/bin/python

class A:
	"""comment1
	comment2
	comment3
	"""
	pass
`))

	language := NewLanguageDefinition("Python", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeReader("test.py", language, buf, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 4 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 3 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Python" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeReaderEvents(t *testing.T) {
	language := NewLanguageDefinition("Python", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}})
	var events []LineEvent
	_, err := AnalyzeReader("test.py", language, strings.NewReader("foo\n  \"\"\"bar\n\n"), &FileOptions{OnLine: func(e LineEvent) { events = append(events, e) }})
	want := []LineEvent{{Number: 1, Kind: CodeLine, Text: "foo"}, {Number: 2, Kind: CommentLine, Text: "\"\"\"bar"}, {Number: 3, Kind: BlankLine, Text: ""}}
	if err != nil || !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%+v error=%v", events, err)
	}
}

func TestAnalyzeFile4Imba(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "test.imba")

	if err := os.WriteFile(tmpfile, []byte(`###
This color is my favorite
I need several lines to really
emphasize this fact.
###
const color = "blue"

# this is line comment
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("Imba", []string{"#"}, [][]string{{"###", "###"}})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 1 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 6 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 1 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Imba" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}

func TestAnalyzeFile4Just(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "tmp.jf")

	if err := os.WriteFile(tmpfile, []byte(`polyglot: python js perl sh ruby nu

python:
  #!/usr/bin/env python3
  print('Hello from python!')

js:
  #!/usr/bin/env node
  console.log('Greetings from JavaScript!')  # with comment

# this is comment
`),
		0o600); err != nil {
		t.Fatalf("os.WriteFile() error. err=[%v]", err)
	}

	language := NewLanguageDefinition("Just", []string{"#"}, [][]string{{"", ""}}).
		withRegexLineComments([]string{`^#[^!].*`})
	clocOpts := &FileOptions{}
	clocFile, readErr := AnalyzeFile(tmpfile, language, clocOpts)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if clocFile.Blanks != 3 {
		t.Errorf("invalid logic. blanks=%v", clocFile.Blanks)
	}
	if clocFile.Comments != 1 {
		t.Errorf("invalid logic. comments=%v", clocFile.Comments)
	}
	if clocFile.Code != 7 {
		t.Errorf("invalid logic. code=%v", clocFile.Code)
	}
	if clocFile.Lang != "Just" {
		t.Errorf("invalid logic. lang=%v", clocFile.Lang)
	}
}
