package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func testSnapshot(tool string) snapshot {
	return snapshot{Tool: tool, Version: "1", RulesVersion: "rules-1", Complete: true,
		Files: map[string]fileRecord{}, Errors: []string{}, Command: []string{}}
}

func TestComparePrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		left, right *fileRecord
		category    string
		incomplete  bool
	}{
		{name: "file set", left: &fileRecord{Path: "a.go", Language: "Go", Counts: counts{Code: 1}}, category: fileSet},
		{name: "language before counts",
			left: &fileRecord{Path: "a.go",
				Language: "Go",
				Counts:   counts{Code: 1}},
			right: &fileRecord{Path: "a.go",
				Language: "C",
				Counts:   counts{Code: 2}},
			category: languageMapping},
		{name: "lexical",
			left: &fileRecord{Path: "a.go",
				Language: "Go",
				Counts:   counts{Code: 1}},
			right: &fileRecord{Path: "a.go",
				Language: "Go",
				Counts:   counts{Comment: 1}},
			category: lexicalCount},
		{name: "embedded",
			left: &fileRecord{Path: "a.go",
				Language: "HTML",
				Counts:   counts{Code: 1}},
			right: &fileRecord{Path: "a.go",
				Language: "HTML",
				Embedded: map[string]counts{"JavaScript": {Code: 1}}},
			category: embeddedLanguage},
		{name: "incomplete suppresses false absence",
			left: &fileRecord{Path: "a.go",
				Language: "Go",
				Counts:   counts{Code: 1}},
			incomplete: true,
			category:   ioOrTimeout},
		{name: "equal",
			left: &fileRecord{Path: "a.go",
				Language: "Go",
				Counts:   counts{Code: 1}},
			right: &fileRecord{Path: "a.go",
				Language: "Go",
				Counts:   counts{Code: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left, right := testSnapshot("locq"), testSnapshot("tokei")
			if tc.left != nil {
				left.Files["a.go"] = *tc.left
			}
			if tc.right != nil {
				right.Files["a.go"] = *tc.right
			}
			if tc.incomplete {
				right.Complete = false
				right.Errors = append(right.Errors, "timeout")
			}
			got := compare(left, right, inputState{Files: map[string]string{"a.go": "digest"}})
			if tc.category == "" {
				if len(got.Differences) != 0 {
					t.Fatal(got)
				}
				return
			}
			if len(got.Differences) != 1 || got.Differences[0].Category != tc.category {
				t.Fatalf("classification=%+v", got)
			}
		})
	}
}

func TestKnownDifferenceMatchesExactObservation(t *testing.T) {
	t.Parallel()
	left, right := testSnapshot("locq"), testSnapshot("tokei")
	left.Files["a.go"] = fileRecord{Path: "a.go", Language: "Go", Counts: counts{Code: 1}}
	right.Files["a.go"] = fileRecord{Path: "a.go", Language: "Go", Counts: counts{Comment: 1}}
	input := inputState{Files: map[string]string{"a.go": "digest"}}
	base := compare(left, right, input).Differences[0]
	rule := knownDifference{ID: "policy", Path: "a.go", Category: lexicalCount, Reason: "reviewed", Issue: "local:policy",
		ReferenceTool: "tokei", ReferenceVersion: "1", RulesVersion: "rules-1", SourceSHA256: "digest",
		Locq: base.Locq, Reference: base.Reference, Allowed: true}
	for _, tc := range []struct {
		name    string
		mutate  func(*knownDifference)
		matched bool
	}{
		{name: "exact", matched: true, mutate: func(*knownDifference) {}},
		{name: "changed tool version", mutate: func(r *knownDifference) { r.ReferenceVersion = "2" }},
		{name: "changed rules", mutate: func(r *knownDifference) { r.RulesVersion = "rules-2" }},
		{name: "changed bytes", mutate: func(r *knownDifference) { r.SourceSHA256 = "other" }},
		{name: "changed counts", mutate: func(r *knownDifference) { file := *r.Locq; file.Counts.Code++; r.Locq = &file }},
		{name: "changed path", mutate: func(r *knownDifference) { r.Path = "b.go" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := rule
			tc.mutate(&candidate)
			result := compare(left, right, input)
			used := map[string]bool{}
			if err := applyKnown(&result, left, right, []knownDifference{candidate}, used); err != nil {
				t.Fatal(err)
			}
			if used[rule.ID] != tc.matched {
				t.Fatalf("known rule match=%t want %t", used[rule.ID], tc.matched)
			}
			if (result.Differences[0].Category == knownSemanticDiff) != tc.matched {
				t.Fatal("classification mismatch")
			}
		})
	}
	result := compare(left, right, input)
	if err := applyKnown(&result, left, right, []knownDifference{rule, rule}, map[string]bool{}); err == nil {
		t.Fatal("overlapping known rules accepted")
	}
}

func TestUnknownIsNeverSilentlyAllowed(t *testing.T) {
	t.Parallel()
	left, right := testSnapshot("locq"), testSnapshot("tokei")
	left.Files["missing.go"] = fileRecord{Path: "missing.go", Language: "Go"}
	result := compare(left, right, inputState{Files: map[string]string{}})
	if result.Categories[unclassified] != 1 {
		t.Fatal(result)
	}
	if !hasUnresolved(report{InputStable: true, Comparisons: []comparison{result}, StaleKnown: []string{}}) {
		t.Fatal("unknown difference was suppressed")
	}
	paths := []string{}
	for _, d := range result.Differences {
		paths = append(paths, d.Path)
	}
	if !slices.IsSorted(paths) {
		t.Fatal("nondeterministic order")
	}
}

func TestKnownDifferenceConfiguration(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "known.json")
	for _, tc := range []struct {
		name, input string
		valid       bool
	}{
		{name: "empty", input: "[]", valid: true},
		{name: "wildcard ignore", input: `[{"allowedDiff":true}]`},
		{name: "trailing document", input: `[] {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filename, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readKnown(filename)
			if (err == nil) != tc.valid {
				t.Fatalf("config valid=%t, err=%v", tc.valid, err)
			}
		})
	}
}
