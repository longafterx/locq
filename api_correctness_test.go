package locq

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/longafterx/locq/internal/lexertest"
)

func TestCorrectnessCorpusAggregation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixtures := lexertest.AllCases()
	extensions := map[string]string{}
	for i, fixture := range fixtures {
		name := filepath.Join(root, fmt.Sprintf("case-%04d.%s", i, fixture.Extension))
		if err := os.WriteFile(name, []byte(fixture.Source), 0600); err != nil {
			t.Fatal(err)
		}
		extensions[fixture.Extension] = fixture.Language
	}
	for _, dedup := range []bool{false, true} {
		t.Run(fmt.Sprintf("dedup=%t", dedup), func(t *testing.T) {
			want := map[string]*ClocFile{}
			seen := map[[32]byte]bool{}
			for i, fixture := range fixtures {
				hash := sha256.Sum256([]byte(fixture.Source))
				if dedup && seen[hash] {
					continue
				}
				seen[hash] = true
				name := filepath.Join(root, fmt.Sprintf("case-%04d.%s", i, fixture.Extension))
				want[name] = &ClocFile{Name: name, Lang: fixture.Language,
					Code: int32(strings.Count(fixture.Kinds, "C")), Comments: int32(strings.Count(fixture.Kinds, "M")),
					Blanks: int32(strings.Count(fixture.Kinds, "B"))}
			}
			for _, workers := range []int{1, 8} {
				result, err := Analyze([]string{root}, &Options{Workers: workers, Dedup: dedup, Extensions: extensions})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(result.Files, want) {
					for name, file := range want {
						if !reflect.DeepEqual(result.Files[name], file) {
							t.Errorf("workers=%d %s: got %+v want %+v", workers, name, result.Files[name], file)
						}
					}
					t.Fatalf("workers=%d file set: got %d want %d", workers, len(result.Files), len(want))
				}
				var code, comment, blank int32
				for _, file := range result.Files {
					code += file.Code
					comment += file.Comments
					blank += file.Blanks
				}
				if result.Total.Code != code || result.Total.Comments != comment || result.Total.Blanks != blank ||
					result.Total.Total != int32(len(want)) {
					t.Fatal("file totals differ from aggregate")
				}
				wantLanguages := map[string]*Language{}
				for name, file := range want {
					language := wantLanguages[file.Lang]
					if language == nil {
						language = &Language{Name: file.Lang, Files: []string{}}
						wantLanguages[file.Lang] = language
					}
					language.Files = append(language.Files, name)
					language.Code += file.Code
					language.Comments += file.Comments
					language.Blanks += file.Blanks
				}
				for name, language := range wantLanguages {
					slices.Sort(language.Files)
					got := result.Languages[name]
					if got == nil {
						t.Fatalf("missing language %s", name)
					}
					slices.Sort(got.Files)
					if !reflect.DeepEqual(got, language) {
						t.Errorf("workers=%d language %s: got %+v want %+v", workers, name, got, language)
					}
				}
				if len(result.Languages) != len(wantLanguages) {
					t.Fatal("language set differs from independent fixtures")
				}
			}
		})
	}
}
