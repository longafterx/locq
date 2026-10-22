package core

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestBuiltInSyntaxCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, language, source string
		code, comments, blanks int32
	}{
		{"JS quoted delimiter", "JavaScript", "const marker = \"/*\";\nconst n = 1;\n", 2, 0, 0},
		{"JS line comment before block marker", "JavaScript", "const n = 1; // /*\nconst m = 2;\n", 2, 0, 0},
		{"JS regular expression", "JavaScript", "const r = /[/*]/;\nconst s = /https?:\\/\\/a/;\n// real\n", 2, 1, 0},
		{"JS division", "JavaScript", "const n = 10 / 2;\n// real\n", 1, 1, 0},
		{"JS return regex", "JavaScript", "function f() { return /[/*]/; }\n", 1, 0, 0},
		{"JS template interpolation", "JavaScript", "const t = `first\n// text\n${1 /* comment */ + 2}\nlast`;\n", 4, 0, 0},
		{"JS nested template", "JavaScript", "const x = `a ${`b ${1}`} c`;\n", 1, 0, 0},
		{"JS escaped template interpolation", "JavaScript", "const x = `\\${/* text */`;\n// real\n", 1, 1, 0},
		{"JS unterminated ordinary string", "JavaScript", "const s = \"unterminated\n// real\n", 1, 1, 0},
		{"TypeScript template", "TypeScript", "const value: string = `/* text`\n", 1, 0, 0},
		{"D nested comments", "D", "/+ outer /+ inner +/ outer +/\nint value;\n", 1, 1, 0},
		{"D ordinary comments do not nest", "D", "/* outer /* inner */ int value;\n", 1, 0, 0},
		{"Go raw literal", "Go", "package p\nvar s = `/*\n// text\n`\n// real\n", 4, 1, 0},
		{"C line splice", "C", "int n; // c \\\n/* still comment\nint m;\n", 2, 1, 0},
		{"BOM and blank block line", "Go", "\xef\xbb\xbfpackage p\r\n/* comment\r\n \t\r\n*/\r\n", 1, 2, 1},
		{
			"JS BOM CRLF and blank template line",
			"JavaScript",
			"\xef\xbb\xbfconst t = `open\r\n\r\n/* text */`\r\n// real",
			2,
			1,
			1,
		},
		{
			"JS invalid UTF8 and long line",
			"JavaScript",
			"const x = \"/*" + strings.Repeat("x", 128*1024) + "\xff\";\n// real\n",
			1,
			1,
			0,
		},
		{"JS unterminated regex recovers", "JavaScript", "const r = /unterminated\n// real\n", 1, 1, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var diagnostics bytes.Buffer
			file, readErr := AnalyzeReader(
				"sample",
				NewDefinedLanguages().Langs[test.language],
				strings.NewReader(test.source),
				&FileOptions{Diagnostics: &diagnostics},
			)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if file.Code != test.code || file.Comments != test.comments || file.Blanks != test.blanks {
				t.Fatalf("counts = %+v; want code=%d comments=%d blanks=%d",
					file, test.code, test.comments, test.blanks)
			}
			if diagnostics.Len() != 0 {
				t.Fatalf("unexpected diagnostic: %s", &diagnostics)
			}
		})
	}
}

func TestBuiltInSyntaxDebugAndReadFailure(t *testing.T) {
	t.Parallel()
	language := NewDefinedLanguages().Langs["JavaScript"]
	var diagnostics bytes.Buffer
	var calls []string
	readErr := errors.New("source failed")
	reader := io.MultiReader(
		strings.NewReader("const s = `open\n// template text\n`;\n// real\n"),
		iotest.ErrReader(readErr),
	)
	file, err := AnalyzeReader("sample.js", language, reader, (&FileOptions{
		OnLine: observeKinds(
			func(line string) { calls = append(calls, "code:"+line) },
			func(line string) { calls = append(calls, "comment:"+line) },
			nil,
		),
		Debug:       true,
		Diagnostics: &diagnostics,
	}))
	if !errors.Is(err, readErr) {
		t.Fatalf("read error = %v, want %v", err, readErr)
	}
	if file.Code != 3 || file.Comments != 1 {
		t.Fatalf("partial counts = %+v", file)
	}
	want := []string{"code:const s = `open", "code:// template text", "code:`;", "comment:// real"}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Fatalf("callbacks = %q, want %q", calls, want)
	}
	if !strings.Contains(diagnostics.String(), "[CODE] file=\"sample.js\" line=2") ||
		!strings.Contains(diagnostics.String(), "[COMM] file=\"sample.js\" line=4") {
		t.Fatalf("debug records = %q", &diagnostics)
	}
}

func TestBuiltInSyntaxCustomRules(t *testing.T) {
	t.Parallel()
	custom := NewLanguageDefinition("Go", []string{"#"}, [][]string{{"{-", "-}"}})
	result, readErr := AnalyzeReader(
		"sample.go", custom,
		strings.NewReader("# comment\nvar s = \"/*\"\n"),
		&FileOptions{},
	)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if result.Comments != 1 || result.Code != 1 {
		t.Fatalf("custom language rules were replaced: %+v", result)
	}
}

func TestGeneratedLanguagesPreserveCustomDefinitions(t *testing.T) {
	t.Parallel()
	for name, language := range NewDefinedLanguages().Langs {
		if language.syntax == nil {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			custom := NewLanguageDefinition(name, []string{"#custom"}, [][]string{{"{-", "-}"}})
			file, readErr := AnalyzeReader(
				"sample",
				custom,
				strings.NewReader("#custom\ncode /* literal */\n{- comment -}\n"),
				&FileOptions{},
			)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if file.Code != 1 || file.Comments != 2 {
				t.Fatalf("generated rules replaced custom rules: %+v", file)
			}
		})
	}
}

func TestBuiltInSyntaxPreservesRegexOverrides(t *testing.T) {
	t.Parallel()
	language := NewDefinedLanguages().Langs["Ada"].withRegexLineComments([]string{`^custom.*`})
	file, readErr := AnalyzeReader(
		"sample",
		language,
		strings.NewReader("custom comment\ncode\n"),
		&FileOptions{},
	)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if file.Comments != 1 || file.Code != 1 {
		t.Fatalf("built-in rules ignored a caller-supplied regex: %+v", file)
	}
}

func TestDeferredLanguagesRetainLegacyScanner(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"HTML", "Vue", "Svelte", "Ruby HTML", "Markdown", "Templ", "Plain Text",
		"Jupyter Notebook", "JSX", "Just", "VimL",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			language := NewDefinedLanguages().Langs[name]
			if language.syntax != nil {
				t.Fatal("deferred language was enabled")
			}
			checkReaderAgainstReference(t, language, "<script>\n// text\n</script>\n```go\n// text\n```\n<!-- real -->\n")
		})
	}
}

func TestEngineSelectionIsExplicit(t *testing.T) {
	var generated, legacy int
	for _, definition := range NewDefinedLanguages().Langs {
		if definition.syntax != nil {
			generated++
		} else {
			legacy++
		}
	}
	if generated != 138 || legacy != 50 {
		t.Fatalf("engine selection: generated=%d legacy=%d", generated, legacy)
	}
	custom := NewLanguageDefinition("Go", []string{"//"}, [][]string{{"/*", "*/"}})
	if custom.syntax != nil {
		t.Fatal("custom language name inferred an engine")
	}
}

func TestEngineFirstLineSemantics(t *testing.T) {
	generated := NewDefinedLanguages().Langs["Python"]
	legacy := NewLanguageDefinition("Python", []string{"#"}, nil)
	for _, tc := range []struct {
		source            string
		generated, legacy LineKind
	}{
		{"\n#!/usr/bin/python\n", CommentLine, CodeLine},
		{"\xef\xbb\xbf#!/usr/bin/python\n", CodeLine, CommentLine},
	} {
		for _, engine := range []struct {
			definition *LanguageDefinition
			want       LineKind
		}{{generated, tc.generated}, {legacy, tc.legacy}} {
			var last LineKind
			_, err := AnalyzeReader(
				"sample.py",
				engine.definition,
				strings.NewReader(tc.source),
				&FileOptions{OnLine: func(e LineEvent) { last = e.Kind }},
			)
			if err != nil || last != engine.want {
				t.Fatalf(
					"source=%q kind=%v want=%v err=%v",
					tc.source,
					last,
					engine.want,
					err,
				)
			}
		}
	}
}
