package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/longafterx/locq/internal/lexertest"
)

func TestReplayRetainsIncompleteEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input, err := inspectInput(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		raw              string
		stable, complete bool
		schema           int
		semantics        string
		invalid          bool
	}{
		{name: "partial execution",
			raw:       `{"Go":{"reports":[]}}`,
			stable:    true,
			schema:    reportVersion,
			semantics: lexertest.SemanticsVersion},
		{name: "malformed raw retained",
			raw:       `not JSON`,
			stable:    true,
			schema:    reportVersion,
			semantics: lexertest.SemanticsVersion},
		{name: "unstable input",
			raw:       `{"Go":{"reports":[]}}`,
			complete:  true,
			schema:    reportVersion,
			semantics: lexertest.SemanticsVersion,
			invalid:   true},
		{name: "unknown schema",
			raw:       `{}`,
			stable:    true,
			complete:  true,
			schema:    reportVersion + 1,
			semantics: lexertest.SemanticsVersion,
			invalid:   true},
		{name: "unknown semantics",
			raw:       `{}`,
			stable:    true,
			complete:  true,
			schema:    reportVersion,
			semantics: "unknown",
			invalid:   true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			raw := filepath.Join(dir, "tokei.json")
			metadata := filepath.Join(dir, "report.json")
			if err := os.WriteFile(raw, []byte(tc.raw), 0600); err != nil {
				t.Fatal(err)
			}
			prior := report{SchemaVersion: tc.schema, Semantics: tc.semantics, Input: input, InputStable: tc.stable,
				Tools: []snapshot{{Tool: "tokei", Complete: tc.complete, RawSHA256: digest([]byte(tc.raw))}}}
			if err := writeJSON(metadata, prior); err != nil {
				t.Fatal(err)
			}
			got, retained, err := importSnapshot(settings{root: root, metadata: metadata}, "tokei", raw, input)
			if (err != nil) != tc.invalid {
				t.Fatalf("replay error=%v, invalid=%t", err, tc.invalid)
			}
			if !tc.invalid && (got.Complete || string(retained) != tc.raw) {
				t.Fatal("incomplete evidence lost or laundered into complete output")
			}
		})
	}
}

func TestInputRejectsSymlinks(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "target.go")
	if err := os.WriteFile(target, []byte("value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "alias.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := inspectInput(root); err == nil {
		t.Fatal("symlink target was not bound to input fingerprint")
	}
}
