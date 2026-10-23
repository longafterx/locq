package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
)

const (
	fileSet           = "FILE_SET"
	languageMapping   = "LANGUAGE_MAPPING"
	lexicalCount      = "LEXICAL_COUNT"
	embeddedLanguage  = "EMBEDDED_LANGUAGE"
	ioOrTimeout       = "IO_OR_TIMEOUT"
	knownSemanticDiff = "KNOWN_SEMANTIC_DIFF"
	unclassified      = "UNCLASSIFIED"
)

type knownDifference struct {
	ID               string      `json:"id"`
	Path             string      `json:"path"`
	Category         string      `json:"category"`
	Reason           string      `json:"reason"`
	Issue            string      `json:"issue"`
	ReferenceTool    string      `json:"reference_tool"`
	ReferenceVersion string      `json:"reference_version"`
	RulesVersion     string      `json:"rules_version"`
	SourceSHA256     string      `json:"source_sha256"`
	Locq             *fileRecord `json:"locq"`
	Reference        *fileRecord `json:"reference"`
	Allowed          bool        `json:"allowed"`
}

func readKnown(filename string) ([]knownDifference, error) {
	rules := []knownDifference{}
	if filename == "" {
		return rules, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read known differences: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rules); err != nil {
		return nil, fmt.Errorf("decode known differences: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("known differences must contain one JSON document")
	}
	ids := map[string]bool{}
	for _, r := range rules {
		validCategory := slices.Contains([]string{fileSet, languageMapping, lexicalCount, embeddedLanguage}, r.Category)
		missingMetadata := r.ID == "" || r.Reason == "" || r.Issue == "" || r.ReferenceVersion == "" || r.RulesVersion == ""
		name, pathErr := normalizePath("", r.Path)
		_, digestErr := hex.DecodeString(r.SourceSHA256)
		invalidPath := pathErr != nil || name != r.Path
		invalidDigest := len(r.SourceSHA256) != 64 || digestErr != nil
		invalidRecord := missingMetadata || !validCategory || invalidPath || invalidDigest
		if invalidRecord {
			return nil, fmt.Errorf(
				"known difference %q needs an exact path, source digest, versions, category, reason and issue", r.ID,
			)
		}
		if r.ReferenceTool != "tokei" && r.ReferenceTool != "cloc" {
			return nil, fmt.Errorf("invalid reference tool for %q", r.ID)
		}
		if ids[r.ID] {
			return nil, fmt.Errorf("duplicate known difference ID %q", r.ID)
		}
		ids[r.ID] = true
	}
	return rules, nil
}

func equalFile(a, b *fileRecord) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Path == b.Path && a.Language == b.Language && a.Counts == b.Counts && maps.Equal(a.Embedded, b.Embedded)
}

func compare(left, right snapshot, input inputState) comparison {
	result := comparison{Reference: right.Tool, Complete: left.Complete && right.Complete,
		Categories: map[string]int{}, Differences: []difference{}}
	names := []string{}
	for name := range left.Files {
		names = append(names, name)
	}
	for name := range right.Files {
		if _, ok := left.Files[name]; !ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		_, a := left.Files[name]
		_, b := right.Files[name]
		switch {
		case a && b:
			result.Common++
		case a:
			result.LocqOnly++
		default:
			result.RefOnly++
		}
	}
	if !result.Complete {
		for _, tool := range []snapshot{left, right} {
			if tool.Complete {
				continue
			}
			category := tool.FailureCategory
			if category == "" {
				category = ioOrTimeout
			}
			reason := tool.Tool + ": " + strings.Join(tool.Errors, "; ")
			result.Differences = append(result.Differences, difference{Category: category,
				Reason: "Comparison incomplete; absence and partial counts are not classified: " + reason})
			result.Categories[category]++
		}
		return result
	}
	for _, name := range names {
		a, aOK := left.Files[name]
		b, bOK := right.Files[name]
		d := difference{Path: name, SourceSHA256: input.Files[name], Reason: ""}
		if aOK {
			d.Locq = &a
		}
		if bOK {
			d.Reference = &b
		}
		switch {
		case d.SourceSHA256 == "":
			d.Category = unclassified
			d.Reason = "Output path is absent from the recorded input inventory; no cause is inferred."
		case !aOK || !bOK:
			d.Category = fileSet
			d.Reason = "File is present in only one tool's output; inspect recorded ignore, detection, " +
				"deduplication and I/O settings. Cause is not inferred from absence."
		case a.Language != b.Language:
			d.Category = languageMapping
			d.Reason = "Same path, different canonical language names; counts are not yet compared."
		case a.Counts != b.Counts || !maps.Equal(a.Embedded, b.Embedded):
			d.Category = lexicalCount
			d.Reason = "Same path and language, different classifications. Reference JSON has file counts, not per-line labels."
			if len(a.Embedded) > 0 || len(b.Embedded) > 0 {
				d.Category = embeddedLanguage
				d.Reason = "Reference reports embedded-language blobs; host counts are preserved separately, never silently folded."
			}
		default:
			continue
		}
		result.Differences = append(result.Differences, d)
		result.Categories[d.Category]++
	}
	return result
}

func applyKnown(result *comparison, left, right snapshot, rules []knownDifference, used map[string]bool) error {
	for i := range result.Differences {
		d := &result.Differences[i]
		observed := d.Category
		for _, r := range rules {
			matchesMetadata := r.Path == d.Path && r.Category == observed && r.ReferenceTool == right.Tool
			matchesVersions := r.ReferenceVersion == right.Version && r.RulesVersion == left.RulesVersion
			matchesSource := r.SourceSHA256 == d.SourceSHA256 && d.SourceSHA256 != ""
			if !matchesMetadata || !matchesVersions || !matchesSource {
				continue
			}
			if !equalFile(r.Locq, d.Locq) || !equalFile(r.Reference, d.Reference) {
				continue
			}
			if d.KnownID != "" {
				return fmt.Errorf("multiple known rules match %s", d.Path)
			}
			d.KnownID = r.ID
			d.Allowed = r.Allowed
			d.Observed = observed
			d.Category = knownSemanticDiff
			d.Reason = r.Reason + " (" + r.Issue + ")"
			used[r.ID] = true
			result.Categories[d.Observed]--
			result.Categories[knownSemanticDiff]++
		}
	}
	return nil
}
