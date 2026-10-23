package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/longafterx/locq/internal/lexertest"
)

func TestLanguageMatrixReaders(t *testing.T) {
	t.Parallel()
	definitions := NewDefinedLanguages()
	if len(definitions.Langs) != len(lexertest.LanguageMatrix) {
		t.Fatal("language matrix is incomplete")
	}
	data, err := os.ReadFile("../languagerules/mappings.json")
	if err != nil {
		t.Fatal(err)
	}
	mappings := map[string]struct {
		Legacy string `json:"keep_legacy"`
	}{}
	if err := json.Unmarshal(data, &mappings); err != nil {
		t.Fatal(err)
	}
	for name, language := range definitions.Langs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry, ok := lexertest.LanguageMatrix[name]
			if !ok {
				t.Fatal("missing reviewed matrix entry")
			}
			if entry.LegacyReason != mappings[name].Legacy {
				t.Fatal("legacy policy changed without matrix review")
			}
			if (language.syntax == nil) != (entry.Tier == 3) {
				t.Fatal("scanner activation differs from reviewed tier")
			}
			for _, source := range []string{
				"value\n \t\nvalue", "\xff界\r\n\nvalue\n",
				"\" ' /* # {- <!-- @\" ''' `\n\n*/ */ \" } -->\n",
			} {
				want, events := observeSource(t, "sample", language, strings.NewReader(source))
				for _, size := range []int{1, 2, 7, 13} {
					got, gotEvents := observeSourceDebug(t, "sample", language, fragmentedSource(source, size), true)
					if *got != *want || len(gotEvents) != len(events) {
						t.Fatal("fragmentation changed classification")
					}
					for i, event := range events {
						if gotEvents[i] != event {
							t.Fatalf("line %d changed with chunk %d", i+1, size)
						}
					}
				}
				fresh, _ := observeSource(t, "fresh", language, strings.NewReader("value\n\n"))
				if fresh.Code != 1 || fresh.Comments != 0 || fresh.Blanks != 1 {
					t.Fatal("previous input leaked lexical state")
				}
				if entry.Tier == 3 {
					checkReaderAgainstReference(t, language, source)
				}
			}
		})
	}
}

func TestReaderReuseKeepsFileStateIsolated(t *testing.T) {
	t.Parallel()
	definitions := NewDefinedLanguages()
	reader := newLineReader(nil)
	root := t.TempDir()
	for i, tc := range []struct{ extension, language, source string }{
		{extension: "rs", language: "Rust", source: "let s = r###\"open\n// literal"},
		{extension: "py", language: "Python", source: "'''documentation\n# docs"},
		{extension: "js", language: "JavaScript", source: "const s = `open\n// literal"},
		{extension: "go", language: "Go", source: "/* open\ncomment"},
	} {
		first := filepath.Join(root, fmt.Sprintf("a%d.%s", i, tc.extension))
		second := filepath.Join(root, fmt.Sprintf("b%d.%s", i, tc.extension))
		marker := "//"
		if tc.language == "Python" {
			marker = "#"
		}
		if err := os.WriteFile(first, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(second, []byte("value\n"+marker+" real\n"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, dedup := range []bool{true, false} {
			state := testRunState(t, &Options{Dedup: dedup})
			if result := detectAndAnalyzeFile(fileCandidate{path: first}, definitions, state, reader); result.err != nil {
				t.Fatal(result.err)
			}
			result := detectAndAnalyzeFile(fileCandidate{path: second}, definitions, state, reader)
			if result.err != nil || result.clocFile == nil {
				t.Fatalf("second file failed: %+v", result)
			}
			if result.clocFile.Code != 1 || result.clocFile.Comments != 1 || result.clocFile.Blanks != 0 {
				t.Fatalf("%s state escaped first file: %+v", tc.language, result.clocFile)
			}
		}
	}
}
