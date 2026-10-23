package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/longafterx/locq/internal/lexertest"
)

type goldenFile struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Language string `json:"language"`
	SHA256   string `json:"sha256"`
	Kinds    string `json:"kinds"`
}

type goldenManifest struct {
	Semantics string       `json:"semantics_version"`
	Files     []goldenFile `json:"files"`
}

func exportCorpus(directory string) error {
	if err := os.MkdirAll(filepath.Dir(directory), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(directory, 0755); err != nil {
		return err
	}
	root := filepath.Join(directory, "src")
	if err := os.Mkdir(root, 0755); err != nil {
		return err
	}
	manifest := goldenManifest{Semantics: lexertest.SemanticsVersion, Files: []goldenFile{}}
	for _, fixture := range lexertest.AllCases() {
		name := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return '-'
		}, fixture.Name)
		if len(name) > 100 {
			name = name[:100]
		}
		name += "-" + digest([]byte(fixture.Name))[:8] + "." + fixture.Extension
		data := []byte(fixture.Source)
		if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, goldenFile{Path: name, Name: fixture.Name,
			Language: fixture.Language, SHA256: digest(data), Kinds: fixture.Kinds})
	}
	if err := writeJSON(filepath.Join(directory, "golden.json"), manifest); err != nil {
		return err
	}
	fmt.Printf("Exported %d independent goldens: %s\n", len(manifest.Files), root)
	return nil
}

func writeMatrix(filename string) error {
	names := []string{}
	counts := map[string]int{}
	for _, fixture := range lexertest.AllCases() {
		counts[fixture.Language]++
	}
	for name := range lexertest.LanguageMatrix {
		names = append(names, name)
	}
	slices.Sort(names)
	var out strings.Builder
	out.WriteString("# Language correctness matrix\n\n")
	out.WriteString("View of the reviewed baseline in `internal/lexertest/matrix.go`; regenerate this document with:\n\n")
	out.WriteString("```sh\ngo run ./scripts/diffcheck -matrix docs/language-test-matrix.md\n```\n\n")
	out.WriteString("Tier 1: deep independent goldens plus feature contracts. " +
		"Tier 2: declared marker/quote feature contracts. " +
		"Tier 3: frozen legacy behavior. Goldens below count shared Case entries; template tests are additional.\n\n")
	out.WriteString("Every active language also runs full-reader/fragmented/debug/plain invariants; " +
		"every legacy language is compared " +
		"with the frozen legacy oracle. These checks establish the stated lexical contract, not a complete language grammar.\n\n")
	out.WriteString("Uncovered contextual syntax remains explicit: " +
		"embedded host languages, dialect-specific SQL, dynamic shell heredocs, " +
		"full JavaScript grammar/regex disambiguation, Python parenthesized docstrings " +
		"and replacement expressions. " +
		"See [counting semantics](counting-semantics.md). " +
		"A feature label promises the fixture contract, not arbitrary syntax.\n\n")
	out.WriteString("| Language | Tier | Shared goldens | Tested features / legacy reason |\n| --- | ---: | ---: | --- |\n")
	for _, name := range names {
		entry := lexertest.LanguageMatrix[name]
		detail := strings.Join(entry.Features, ", ")
		if entry.LegacyReason != "" {
			detail = entry.LegacyReason
		}
		fmt.Fprintf(&out, "| %s | %d | %d | %s |\n", name, entry.Tier, counts[name], strings.ReplaceAll(detail, "|", "\\|"))
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(out.String()), 0644)
}
