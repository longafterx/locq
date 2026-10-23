package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/longafterx/locq/internal/lexertest"
)

func TestGoldenChecksLabelsWhenCountsAgree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := []byte("value\n// real\n")
	if err := os.WriteFile(filepath.Join(root, "a.go"), source, 0600); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join(t.TempDir(), "golden.json")
	if err := writeJSON(golden, goldenManifest{Semantics: lexertest.SemanticsVersion, Files: []goldenFile{
		{Path: "a.go", Language: "Go", SHA256: digest(source), Kinds: "MC"},
	}}); err != nil {
		t.Fatal(err)
	}
	opts := settings{root: root, out: filepath.Join(t.TempDir(), "report"), golden: golden,
		rules: "../../internal/lexer/rules_generated.go", timeout: time.Second, policy: "aligned", check: true}
	if err := run(opts); err == nil {
		t.Fatal("swapped labels accepted because totals agree")
	}
	data, err := os.ReadFile(filepath.Join(opts.out, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The full report must retain exact line attribution, despite equal totals.
	text := string(data)
	if !strings.Contains(text, "Per-line golden labels differ") || !strings.Contains(text, `"expected": "M"`) {
		t.Fatalf("missing per-line mismatch: %s", text)
	}
}

func TestTraceRejectsChangedSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	data := []byte("// changed\nvalue\n") // Equal totals, different line labels.
	if err := os.WriteFile(filepath.Join(root, "a.go"), data, 0600); err != nil {
		t.Fatal(err)
	}
	d := difference{Path: "a.go", SourceSHA256: digest([]byte("value\n// original\n")),
		Locq: &fileRecord{Language: "Go", Counts: counts{Code: 1, Comment: 1}}}
	result := comparison{Reference: "tokei", Complete: true, Differences: []difference{d}}
	if err := attachTraces(settings{root: root, out: t.TempDir()}, &result, snapshot{BuildSHA256: executableDigest()}); err != nil {
		t.Fatal(err)
	}
	if result.Complete || len(result.Differences[0].Lines) != 0 {
		t.Fatal("new line labels attributed to old source snapshot")
	}
}

func TestMarkdownEscapesSourceHTML(t *testing.T) {
	t.Parallel()
	got := mdEscape("<script>alert(1)</script>|\r\n")
	if strings.Contains(got, "<script>") || !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, `\|`) {
		t.Fatalf("unsafe or invisible source: %s", got)
	}
}
