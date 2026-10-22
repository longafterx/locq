package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type ruleTransport func(*http.Request) (*http.Response, error)

func (transport ruleTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestReadSourceMetadata(t *testing.T) {
	t.Parallel()
	valid := sourceMetadata{
		Commit: strings.Repeat("a", 40), SHA256: strings.Repeat("b", 64), Languages: 1,
	}
	for _, test := range []struct {
		name     string
		commit   string
		checksum string
		count    int
		valid    bool
	}{
		{name: "valid", commit: valid.Commit, checksum: valid.SHA256, count: 1, valid: true},
		{name: "short commit", commit: "aaa", checksum: valid.SHA256, count: 1},
		{name: "invalid checksum", commit: valid.Commit, checksum: strings.Repeat("z", 64), count: 1},
		{name: "empty catalog", commit: valid.Commit, checksum: valid.SHA256},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), metadataPath)
			metadata := sourceMetadata{Commit: test.commit, SHA256: test.checksum, Languages: test.count}
			writeJSON(t, path, metadata)
			if _, err := readSourceMetadata(path); (err == nil) != test.valid {
				t.Fatalf("read metadata = %v, want valid=%t", err, test.valid)
			}
		})
	}
}

func TestUpdateRules(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("b", 40)
	for _, test := range []struct {
		name      string
		commits   string
		snapshot  string
		status    int
		wantError string
	}{
		{
			name: "valid update", snapshot: `{"languages":{"Example":{"line_comment":["#"]},"New":{}}}`,
		},
		{
			name: "API unavailable", status: http.StatusServiceUnavailable, wantError: "HTTP 503",
		},
		{
			name: "snapshot missing", snapshot: `{}`, status: http.StatusNotFound, wantError: "HTTP 404",
		},
		{
			name: "empty history", commits: `[]`, wantError: "expected one upstream",
		},
		{
			name: "bad commit", commits: `[{"sha":"../master"}]`, wantError: "invalid upstream commit",
		},
		{
			name: "unknown schema", snapshot: `{"languages":{"Example":{"future_field":true}}}`,
			wantError: "unknown field",
		},
		{
			name: "unsupported delimiter", snapshot: `{"languages":{"Example":{"multi_line_comments":[["/*"]]}}}`,
			wantError: "invalid delimiter pair",
		},
		{
			name: "removed mapped language", snapshot: `{"languages":{"New":{}}}`,
			wantError: "unknown source",
		},
		{
			name: "empty catalog", snapshot: `{"languages":{}}`, wantError: "catalog has 0 entries",
		},
		{
			name: "CRLF snapshot", snapshot: "{\r\n}", wantError: "LF line endings",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := languageRulesFixture(t)
			before := readRuleFiles(t, directory)
			client := &http.Client{Transport: ruleTransport(func(request *http.Request) (*http.Response, error) {
				status := http.StatusOK
				body := test.snapshot
				if request.URL.Host != "api.github.com" {
					t.Fatalf("unexpected host: %s", request.URL.Host)
				}
				switch request.URL.Path {
				case "/repos/XAMPPRocky/tokei/commits":
					query := request.URL.Query()
					if query.Get("path") != sourcePath || query.Get("sha") != "master" || query.Get("per_page") != "1" {
						t.Fatalf("unexpected commit query: %s", request.URL.RawQuery)
					}
					body = test.commits
					if body == "" {
						body = fmt.Sprintf(`[{"sha":%q}]`, commit)
					}
					if test.snapshot == "" && test.status != 0 {
						status = test.status
					}
				case "/repos/XAMPPRocky/tokei/contents/languages.json":
					if request.URL.Query().Get("ref") != commit {
						t.Fatalf("snapshot is not pinned to resolved commit: %s", request.URL.RawQuery)
					}
					if request.Header.Get("Accept") != "application/vnd.github.raw+json" {
						t.Fatal("snapshot request must fetch exact upstream bytes")
					}
					if test.status != 0 {
						status = test.status
					}
				default:
					t.Fatalf("unexpected API path: %s", request.URL.Path)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			err := updateRules(client, "master", directory)
			after := readRuleFiles(t, directory)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("update = %v, want error containing %q", err, test.wantError)
				}
				for path, content := range before {
					if !bytes.Equal(content, after[path]) {
						t.Errorf("failed update changed %s", path)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := readSourceMetadata(filepath.Join(directory, metadataPath))
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Commit != commit || metadata.Languages != 2 || string(after[sourcePath]) != test.snapshot {
				t.Fatalf("update did not preserve upstream bytes/provenance: %+v", metadata)
			}
			code, report, err := compileSnapshot(after[sourcePath], metadata, directory)
			if err != nil || !bytes.Equal(code, after[outputPath]) || !bytes.Equal(report, after[coveragePath]) {
				t.Fatalf("updated artifacts do not match snapshot: %v", err)
			}
			if err := updateRules(client, "master", directory); err != nil {
				t.Fatal(err)
			}
			for path, content := range readRuleFiles(t, directory) {
				if !bytes.Equal(content, after[path]) {
					t.Errorf("repeated update changed %s", path)
				}
			}
		})
	}
}

func TestDownloadLimits(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "oversized response", body: strings.Repeat("x", maxDownloadBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := &http.Client{Transport: ruleTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			if _, err := download(client, "https://api.github.com/test", "application/vnd.github.raw+json"); err == nil {
				t.Fatal("invalid snapshot was accepted")
			}
		})
	}
}

func languageRulesFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, "languagerules")
	for _, path := range []string{directory, filepath.Join(root, "lexer")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		sourcePath:  `{"languages":{"Example":{"line_comment":["//"]}}}`,
		mappingPath: `{"Example":{"source":"Example"}}`, outputPath: "old rules\n", coveragePath: "old coverage\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(filepath.Join(directory, sourcePath))
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(directory, metadataPath), sourceMetadata{
		Commit: strings.Repeat("a", 40), SHA256: fmt.Sprintf("%x", sha256.Sum256(content)), Languages: 1,
	})
	return directory
}

func readRuleFiles(t *testing.T, directory string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, path := range []string{sourcePath, metadataPath, mappingPath, outputPath, coveragePath} {
		content, err := os.ReadFile(filepath.Join(directory, path))
		if err != nil {
			t.Fatal(err)
		}
		files[path] = content
	}
	return files
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
}
