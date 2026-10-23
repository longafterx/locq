package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/longafterx/locq"
	"github.com/longafterx/locq/internal/lexertest"
)

func loadGolden(filename string) (goldenManifest, error) {
	manifest := goldenManifest{Files: []goldenFile{}}
	if filename == "" {
		return manifest, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.Semantics != lexertest.SemanticsVersion {
		return manifest, fmt.Errorf("golden semantics version differs")
	}
	return manifest, nil
}

func collectGolden(filename string, input inputState) (snapshot, error) {
	manifest, err := loadGolden(filename)
	if err != nil {
		return snapshot{}, err
	}
	s := snapshot{Tool: "contract", Version: manifest.Semantics, RulesVersion: manifest.Semantics,
		Command: []string{"golden", filename}, Complete: true, Errors: []string{}, Files: map[string]fileRecord{}}
	for _, file := range manifest.Files {
		if _, exists := s.Files[file.Path]; exists {
			return s, fmt.Errorf("duplicate golden path: %s", file.Path)
		}
		if input.Files[file.Path] != file.SHA256 {
			return s, fmt.Errorf("golden input changed or missing: %s", file.Path)
		}
		if strings.Trim(file.Kinds, "CMB") != "" {
			return s, fmt.Errorf("invalid golden labels: %s", file.Path)
		}
		s.Files[file.Path] = fileRecord{Path: file.Path, Language: file.Language, Counts: counts{
			Code: int64(strings.Count(file.Kinds, "C")), Comment: int64(strings.Count(file.Kinds, "M")),
			Blank: int64(strings.Count(file.Kinds, "B"))}}
	}
	return s, nil
}

// File totals alone cannot detect swapped line labels. Check every golden,
// including files whose three category totals already agree.
func compareGoldenLines(opts settings, result *comparison, left, right snapshot, input inputState) error {
	if !result.Complete {
		return nil
	}
	if left.BuildSHA256 == "" || left.BuildSHA256 != executableDigest() {
		result.Complete = false
		result.Categories[unclassified]++
		result.Differences = append(result.Differences, difference{
			Category: unclassified, Reason: "Imported executable differs; line-level contract cannot be replayed."})
		return nil
	}
	manifest, err := loadGolden(opts.golden)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, d := range result.Differences {
		existing[d.Path] = true
	}
	definitions := locq.NewDefinedLanguages()
	for _, golden := range manifest.Files {
		a, ok := left.Files[golden.Path]
		if !ok || existing[golden.Path] || a.Language != golden.Language {
			continue
		}
		data, err := os.ReadFile(sourcePath(opts.root, golden.Path))
		if err != nil {
			return err
		}
		if digest(data) != golden.SHA256 {
			return fmt.Errorf("golden input changed while checking lines: %s", golden.Path)
		}
		language := definitions.Langs[a.Language]
		if language == nil {
			return fmt.Errorf("golden language lacks a rule: %s", a.Language)
		}
		var labels strings.Builder
		file, err := locq.AnalyzeReader(golden.Path, language, bytes.NewReader(data), &locq.FileOptions{
			OnLine: func(event locq.LineEvent) { labels.WriteByte("CMB"[event.Kind]) },
		})
		if err != nil {
			return err
		}
		observed := counts{Code: int64(file.Code), Comment: int64(file.Comments), Blank: int64(file.Blanks)}
		if observed != a.Counts {
			return fmt.Errorf("locq counts changed while checking golden: %s", golden.Path)
		}
		if labels.String() == golden.Kinds {
			continue
		}
		b := right.Files[golden.Path]
		result.Categories[lexicalCount]++
		result.Differences = append(result.Differences, difference{
			Path: golden.Path, Category: lexicalCount, Locq: &a, Reference: &b,
			SourceSHA256: input.Files[golden.Path],
			Reason:       "Per-line golden labels differ even though file totals agree.",
		})
	}
	slices.SortFunc(result.Differences, func(a, b difference) int { return strings.Compare(a.Path, b.Path) })
	return nil
}

func attachTraces(opts settings, result *comparison, left snapshot) error {
	manifest, err := loadGolden(opts.golden)
	if err != nil {
		return err
	}
	goldens := map[string]goldenFile{}
	for _, file := range manifest.Files {
		goldens[file.Path] = file
	}
	definitions := locq.NewDefinedLanguages()
	for i := range result.Differences {
		d := &result.Differences[i]
		if d.Path == "" {
			continue
		}
		filename := sourcePath(opts.root, d.Path)
		info, err := os.Lstat(filename)
		if err != nil || !info.Mode().IsRegular() {
			d.Reason += " Source unavailable or not a regular file; no line attribution."
			continue
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		if digest(data) != d.SourceSHA256 {
			d.Reason += " Source changed; trace not attributed."
			result.Complete = false
			continue
		}
		relative := filepath.Join("repro", result.Reference, "src", filepath.FromSlash(d.Path))
		target := filepath.Join(opts.out, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
		d.Reproduction = filepath.ToSlash(relative)
		if d.Locq == nil {
			continue
		}
		if left.BuildSHA256 == "" || left.BuildSHA256 != executableDigest() {
			d.Reason += " Imported locq executable differs; current line labels are not attributed to the imported run."
			continue
		}
		language := definitions.Langs[d.Locq.Language]
		if language == nil {
			d.Reason += " No built-in rule for a locq line trace."
			continue
		}
		lines := []sourceLine{}
		rawLines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		golden := goldens[d.Path]
		file, err := locq.AnalyzeReader(
			filename,
			language,
			bytes.NewReader(data),
			&locq.FileOptions{OnLine: func(event locq.LineEvent) {
				line := sourceLine{Number: event.Number, Text: rawLines[event.Number-1], Locq: string("CMB"[event.Kind])}
				if golden.SHA256 == d.SourceSHA256 && int(event.Number) <= len(golden.Kinds) {
					line.Expected = string(golden.Kinds[event.Number-1])
				}
				if result.Reference == "contract" {
					line.Reference = line.Expected
				}
				lines = append(lines, line)
			}},
		)
		if err != nil {
			d.Reason += " Incomplete line trace: " + err.Error()
			result.Complete = false
			continue
		}
		got := counts{Code: int64(file.Code), Comment: int64(file.Comments), Blank: int64(file.Blanks)}
		if got != d.Locq.Counts {
			d.Reason += " Imported locq counts differ from the current scanner; trace is not attributed."
			continue
		}
		if err := writeJSON(target+".lines.json", lines); err != nil {
			return err
		}
		// Keep reports bounded; the complete trace and original bytes are artifacts.
		d.Lines = slices.Clone(lines[:min(len(lines), 32)])
		for _, line := range lines[min(len(lines), 32):] {
			if line.Expected != "" && line.Expected != line.Locq {
				d.Lines = append(d.Lines, line)
			}
		}
	}
	return nil
}

func mdEscape(text string) string {
	text = html.EscapeString(text)
	text = strings.ReplaceAll(text, "|", "\\|")
	text = strings.ReplaceAll(text, "\r", "\\r")
	return strings.ReplaceAll(text, "\n", "\\n")
}

func markdownReport(r report) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# locq differential correctness report\n\nInput SHA-256: `%s`\n\n", r.Input.SHA256)
	fmt.Fprintf(&out, "Semantics: `%s`. Input stable: **%t**.\n\n", r.Semantics, r.InputStable)
	if r.Input.Commit != "" {
		fmt.Fprintf(&out, "Repository commit: `%s`; git status: `%s`.\n\n", r.Input.Commit, mdEscape(r.Input.GitStatus))
	}
	out.WriteString("External counts are observations, not a correctness oracle. " +
		"Reference JSON provides no per-line labels; `?` means unavailable. " +
		"Full source reproductions and locq traces accompany file-level differences. " +
		"Golden expectations are independently specified.\n\n")
	for _, tool := range r.Tools {
		fmt.Fprintf(
			&out,
			"- %s: `%s`; complete=%t; command=`%s`\n",
			tool.Tool,
			mdEscape(tool.Version),
			tool.Complete,
			mdEscape(strings.Join(tool.Command, " ")),
		)
		for _, err := range tool.Errors {
			fmt.Fprintf(&out, "  - %s\n", mdEscape(err))
		}
	}
	for _, comparison := range r.Comparisons {
		fmt.Fprintf(&out, "\n## locq / %s\n\nComplete: **%t**. locq only: %d; reference only: %d; common: %d.\n\n",
			comparison.Reference, comparison.Complete, comparison.LocqOnly, comparison.RefOnly, comparison.Common)
		out.WriteString("| Category | Differences |\n| --- | ---: |\n")
		categories := []string{
			fileSet, languageMapping, lexicalCount, embeddedLanguage, ioOrTimeout, knownSemanticDiff, unclassified,
		}
		for _, category := range categories {
			fmt.Fprintf(&out, "| %s | %d |\n", category, comparison.Categories[category])
		}
		for _, d := range comparison.Differences {
			fmt.Fprintf(&out, "\n### [%s] %s\n\n%s\n\n", d.Category, mdEscape(d.Path), mdEscape(d.Reason))
			for _, side := range []struct {
				name string
				file *fileRecord
			}{{name: "locq", file: d.Locq}, {name: comparison.Reference, file: d.Reference}} {
				if side.file != nil {
					fmt.Fprintf(
						&out,
						"- %s (%s): code=%d comment=%d blank=%d\n",
						side.name,
						side.file.Language,
						side.file.Counts.Code,
						side.file.Counts.Comment,
						side.file.Counts.Blank,
					)
				}
			}
			if d.KnownID != "" {
				fmt.Fprintf(&out, "- Known record: `%s`, allowed=%t\n", mdEscape(d.KnownID), d.Allowed)
			}
			if d.Reproduction != "" {
				fmt.Fprintf(
					&out,
					"- Original source: [%s](<%s>), SHA-256 `%s`\n",
					mdEscape(d.Reproduction),
					d.Reproduction,
					d.SourceSHA256,
				)
			}
			if len(d.Lines) == 0 {
				continue
			}
			out.WriteString("\n| Line | locq | Reference | Golden | Source |\n| ---: | --- | --- | --- | --- |\n")
			for _, line := range d.Lines {
				reference := line.Reference
				if reference == "" {
					reference = "?"
				}
				golden := line.Expected
				if golden == "" {
					golden = "?"
				}
				text := line.Text
				if len(text) > 240 {
					text = text[:240] + "… (full line in artifact)"
				}
				fmt.Fprintf(&out, "| %d | %s | %s | %s | %s |\n", line.Number, line.Locq, reference, golden, mdEscape(text))
			}
		}
	}
	if len(r.StaleKnown) > 0 {
		fmt.Fprintf(&out, "\nStale/unmatched known records: %s\n", strings.Join(r.StaleKnown, ", "))
	}
	return out.String()
}
