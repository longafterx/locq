package core

import (
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/longafterx/locq/internal/lexertest"
)

func TestLanguageCorpus(t *testing.T) {
	t.Parallel()
	for _, fixture := range lexertest.AllCases() {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			language := NewDefinedLanguages().Langs[fixture.Language]
			if language == nil {
				t.Fatalf("missing lexical rules for %s", fixture.Language)
			}
			lines := physicalLines(fixture.Source)
			if len(lines) != len(fixture.Kinds) {
				t.Fatalf("invalid fixture: %d physical lines, %d expectations", len(lines), len(fixture.Kinds))
			}
			for i := range lines {
				lines[i] = strings.TrimSpace(lines[i])
				if i == 0 {
					lines[i] = strings.TrimPrefix(lines[i], "\xef\xbb\xbf")
				}
			}
			want := &ClocFile{
				Name: "sample." + fixture.Extension, Lang: fixture.Language,
				Code:     int32(strings.Count(fixture.Kinds, "C")),
				Comments: int32(strings.Count(fixture.Kinds, "M")), Blanks: int32(strings.Count(fixture.Kinds, "B")),
			}
			plain, err := AnalyzeReader(want.Name, language, strings.NewReader(fixture.Source), nil)
			if err != nil || !reflect.DeepEqual(plain, want) {
				t.Fatalf("plain analysis = %+v/%v, want %+v", plain, err, want)
			}
			for _, size := range []int{0, 1, 2, 7, 13} {
				for _, debug := range []bool{false, true} {
					reader := fragmentedSource(fixture.Source, size)
					file, events := observeSourceDebug(t, want.Name, language, reader, debug)
					if !reflect.DeepEqual(file, want) {
						t.Fatalf("chunk=%d debug=%t counts=%+v, want %+v", size, debug, file, want)
					}
					if len(events) != len(fixture.Kinds) {
						t.Fatalf("chunk=%d events=%d, want %d", size, len(events), len(fixture.Kinds))
					}
					for i, event := range events {
						kind := "CMB"[event.Kind]
						if kind != fixture.Kinds[i] || event.Number != int64(i+1) || event.Text != lines[i] {
							t.Fatalf("chunk=%d debug=%t line=%d: %+v, want kind=%c text=%q",
								size, debug, i+1, event, fixture.Kinds[i], lines[i])
						}
					}
				}
			}
		})
	}
}

func physicalLines(source string) []string {
	if source == "" {
		return []string{}
	}
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}

// A fixed schedule makes byte-boundary failures reproducible. Size 13 varies
// deterministically; DataErrReader also exercises data returned together with EOF.
func fragmentedSource(source string, size int) io.Reader {
	if size == 0 {
		return iotest.DataErrReader(strings.NewReader(source))
	}
	return &contractReader{source: strings.NewReader(source), size: size}
}

type contractReader struct {
	source *strings.Reader
	size   int
	step   int
}

func (r *contractReader) Read(buffer []byte) (int, error) {
	size := r.size
	if size == 13 {
		size = 1 + r.step%13
		r.step += 7
	}
	return r.source.Read(buffer[:min(len(buffer), size)])
}

func observeSource(t testing.TB, name string, language *LanguageDefinition, reader io.Reader) (*ClocFile, []LineEvent) {
	t.Helper()
	return observeSourceDebug(t, name, language, reader, false)
}

func observeSourceDebug(
	t testing.TB,
	name string,
	language *LanguageDefinition,
	reader io.Reader,
	debug bool,
) (*ClocFile, []LineEvent) {
	t.Helper()
	events := []LineEvent{}
	file, err := AnalyzeReader(name, language, reader, &FileOptions{
		Debug: debug, Diagnostics: io.Discard,
		OnLine: func(event LineEvent) { events = append(events, event) },
	})
	if err != nil {
		t.Fatal(err)
	}
	return file, events
}

func FuzzLanguageReader(f *testing.F) {
	languages := NewDefinedLanguages()
	names := make([]string, 0, len(languages.Langs))
	for name := range languages.Langs {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, fixture := range lexertest.AllCases() {
		index, _ := slices.BinarySearch(names, fixture.Language)
		f.Add(fixture.Source, uint16(index), uint8(7))
	}
	for _, source := range []string{"", " \r\n\r\n", " ", "\xef\xbb\xbf", "\xff\n/*", "界\\\n/*\n*/"} {
		for i := range names {
			f.Add(source, uint16(i), uint8(1))
		}
	}
	f.Fuzz(func(t *testing.T, source string, index uint16, chunk uint8) {
		if len(source) > 1024*1024 {
			t.Skip()
		}
		language := languages.Langs[names[int(index)%len(names)]]
		want, wantEvents := observeSource(t, "sample", language, strings.NewReader(source))
		got, events := observeSourceDebug(
			t,
			"sample",
			language,
			fragmentedSource(source, 1+int(chunk)%13),
			true,
		)
		plain, err := AnalyzeReader("sample", language, strings.NewReader(source), nil)
		if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(plain, want) {
			t.Fatalf("read paths changed counts: %+v / %+v / %+v / %v", got, plain, want, err)
		}
		if !reflect.DeepEqual(events, wantEvents) {
			t.Fatalf("read paths changed line events: %+v versus %+v", events, wantEvents)
		}
		if got.Code+got.Comments+got.Blanks != int32(len(physicalLines(source))) {
			t.Fatalf("physical lines were lost or counted twice: %+v", got)
		}
	})
}
