// diffcheck is a development tool. It never participates in locq's CLI hot path.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

const reportVersion = 1

type counts struct {
	Code    int64 `json:"code"`
	Comment int64 `json:"comment"`
	Blank   int64 `json:"blank"`
}

type fileRecord struct {
	Path     string            `json:"path"`
	Language string            `json:"language"`
	Counts   counts            `json:"counts"`
	Embedded map[string]counts `json:"embedded,omitempty"`
}

type snapshot struct {
	FailureCategory string                `json:"failure_category,omitempty"`
	BuildSHA256     string                `json:"build_sha256,omitempty"`
	RawSHA256       string                `json:"raw_sha256"`
	RawRoot         string                `json:"raw_path_root,omitempty"`
	Tool            string                `json:"tool"`
	Version         string                `json:"version"`
	RulesVersion    string                `json:"rules_version,omitempty"`
	Command         []string              `json:"command"`
	Complete        bool                  `json:"complete"`
	Errors          []string              `json:"errors"`
	Files           map[string]fileRecord `json:"files"`
}

type sourceLine struct {
	Number    int64  `json:"number"`
	Text      string `json:"text"`
	Locq      string `json:"locq"`
	Expected  string `json:"expected,omitempty"`
	Reference string `json:"reference,omitempty"`
}

type difference struct {
	Path         string       `json:"path"`
	Category     string       `json:"category"`
	Observed     string       `json:"observed_category,omitempty"`
	Reason       string       `json:"reason"`
	Locq         *fileRecord  `json:"locq,omitempty"`
	Reference    *fileRecord  `json:"reference,omitempty"`
	SourceSHA256 string       `json:"source_sha256,omitempty"`
	Lines        []sourceLine `json:"lines,omitempty"`
	Reproduction string       `json:"reproduction,omitempty"`
	KnownID      string       `json:"known_id,omitempty"`
	Allowed      bool         `json:"allowed"`
}

type comparison struct {
	Reference   string         `json:"reference"`
	Complete    bool           `json:"complete"`
	LocqOnly    int            `json:"locq_only"`
	RefOnly     int            `json:"reference_only"`
	Common      int            `json:"common"`
	Categories  map[string]int `json:"categories"`
	Differences []difference   `json:"differences"`
}

type inputState struct {
	Root      string            `json:"root"`
	Commit    string            `json:"commit,omitempty"`
	GitStatus string            `json:"git_status,omitempty"`
	SHA256    string            `json:"sha256"`
	Files     map[string]string `json:"files"`
}

type report struct {
	SchemaVersion int          `json:"schema_version"`
	Semantics     string       `json:"semantics_version"`
	Input         inputState   `json:"input"`
	InputStable   bool         `json:"input_stable"`
	Tools         []snapshot   `json:"tools"`
	Comparisons   []comparison `json:"comparisons"`
	StaleKnown    []string     `json:"stale_known_differences"`
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func normalizePath(root, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	root = strings.TrimSuffix(strings.ReplaceAll(root, "\\", "/"), "/")
	if name == root {
		return "", fmt.Errorf("file path is the scan root: %q", name)
	}
	if strings.HasPrefix(name, root+"/") {
		name = strings.TrimPrefix(name, root+"/")
	}
	name = path.Clean(name)
	drivePath := len(name) >= 2 && name[1] == ':'
	outside := path.IsAbs(name) || drivePath || name == ".." || strings.HasPrefix(name, "../")
	if outside || name == "." {
		return "", fmt.Errorf("file path escapes scan root: %q", name)
	}
	return name, nil
}

func canonicalLanguage(name string) string {
	aliases := map[string]string{
		"C Header": "C Header", "C/C++ Header": "C Header", "CHeader": "C Header",
		"C++ Header": "C++ Header", "CppHeader": "C++ Header", "Cpp": "C++", "CSharp": "C#",
		"GNU Style Assembly": "Assembly", "Text": "Plain Text", "PlainText": "Plain Text",
		"Bash": "BASH", "Bourne Again Shell": "BASH", "Shell": "Bourne Shell",
		"DOS Batch": "Batch", "Windows Batch": "Batch", "make": "Makefile",
	}
	if canonical, ok := aliases[name]; ok {
		return canonical
	}
	return name
}

func addFile(s *snapshot, root string, record fileRecord) error {
	name, err := normalizePath(root, record.Path)
	if err != nil {
		return err
	}
	if record.Language == "" || record.Counts.Code < 0 || record.Counts.Comment < 0 || record.Counts.Blank < 0 {
		return fmt.Errorf("invalid language or counts for %q", name)
	}
	if _, exists := s.Files[name]; exists {
		return fmt.Errorf("duplicate normalized path %q", name)
	}
	record.Path = name
	record.Language = canonicalLanguage(record.Language)
	s.Files[name] = record
	return nil
}

func jsonObject(data []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return object, nil
}

func requiredString(object map[string]json.RawMessage, key string) (string, error) {
	var value string
	if err := json.Unmarshal(object[key], &value); err != nil || value == "" {
		return "", fmt.Errorf("missing or invalid %s", key)
	}
	return value, nil
}

func parseCounts(data []byte, plural bool) (counts, error) {
	object, err := jsonObject(data)
	if err != nil {
		return counts{}, err
	}
	keys := []string{"code", "comment", "blank"}
	if plural {
		keys = []string{"code", "comments", "blanks"}
	}
	values := []int64{}
	for _, key := range keys {
		raw, exists := object[key]
		if !exists || string(raw) == "null" {
			return counts{}, fmt.Errorf("missing count %s", key)
		}
		var value int64
		if err := json.Unmarshal(raw, &value); err != nil || value < 0 {
			return counts{}, fmt.Errorf("invalid count %s", key)
		}
		values = append(values, value)
	}
	return counts{Code: values[0], Comment: values[1], Blank: values[2]}, nil
}

func requiredList(raw json.RawMessage) ([]json.RawMessage, error) {
	items := []json.RawMessage{}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("expected a JSON array, not null or missing")
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func decodeSnapshot(tool, root string, data []byte) (snapshot, error) {
	s := snapshot{Tool: tool, Files: map[string]fileRecord{}, Errors: []string{}, Command: []string{}, Complete: true}
	object, err := jsonObject(data)
	if err != nil {
		return s, err
	}
	switch tool {
	case "locq":
		raw, exists := object["files"]
		if !exists {
			return s, fmt.Errorf("locq JSON needs per-file output (-f -o json)")
		}
		files, err := requiredList(raw)
		if err != nil {
			return s, err
		}
		for _, raw := range files {
			file, err := jsonObject(raw)
			if err != nil {
				return s, err
			}
			name, err := requiredString(file, "name")
			if err != nil {
				return s, err
			}
			language, err := requiredString(file, "language")
			if err != nil {
				return s, err
			}
			stats, err := parseCounts(raw, false)
			if err != nil {
				return s, err
			}
			if err := addFile(&s, root, fileRecord{Path: name, Language: language, Counts: stats}); err != nil {
				return s, err
			}
		}
	case "tokei":
		for language, raw := range object {
			group, err := jsonObject(raw)
			if err != nil {
				return s, err
			}
			var inaccurate bool
			if raw, exists := group["inaccurate"]; exists {
				if err := json.Unmarshal(raw, &inaccurate); err != nil {
					return s, err
				}
			}
			if inaccurate {
				s.Complete = false
				s.Errors = append(s.Errors, "tokei marks "+language+" inaccurate")
			}
			if language == "Total" {
				continue
			}
			reports, err := requiredList(group["reports"])
			if err != nil {
				return s, fmt.Errorf("tokei %q needs per-file reports: %w", language, err)
			}
			for _, raw := range reports {
				file, err := jsonObject(raw)
				if err != nil {
					return s, err
				}
				name, err := requiredString(file, "name")
				if err != nil {
					return s, err
				}
				stats, err := parseCounts(file["stats"], true)
				if err != nil {
					return s, err
				}
				source, err := jsonObject(file["stats"])
				if err != nil {
					return s, err
				}
				embedded := map[string]counts{}
				if raw, exists := source["blobs"]; exists && string(raw) != "null" {
					blobs, err := jsonObject(raw)
					if err != nil {
						return s, err
					}
					for name, raw := range blobs {
						stats, err := parseCounts(raw, true)
						if err != nil {
							return s, err
						}
						embedded[canonicalLanguage(name)] = stats
					}
				}
				if err := addFile(&s, root, fileRecord{Path: name, Language: language, Counts: stats, Embedded: embedded}); err != nil {
					return s, err
				}
			}
		}
	case "cloc":
		for name, raw := range object {
			if name == "header" || name == "SUM" {
				continue
			}
			file, err := jsonObject(raw)
			if err != nil {
				return s, err
			}
			language, err := requiredString(file, "language")
			if err != nil {
				return s, err
			}
			stats, err := parseCounts(raw, false)
			if err != nil {
				return s, err
			}
			if err := addFile(&s, root, fileRecord{Path: name, Language: language, Counts: stats}); err != nil {
				return s, err
			}
		}
	default:
		return s, fmt.Errorf("unsupported tool %q", tool)
	}
	return s, nil
}

func sourcePath(root, name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
