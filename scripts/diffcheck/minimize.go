package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type numberedLine struct {
	original int
	data     []byte
}

func joinLines(lines []numberedLine) []byte {
	var out bytes.Buffer
	for _, line := range lines {
		out.Write(line.data)
	}
	return out.Bytes()
}

// Reduce by deleting contiguous line ranges. Only the supplied observation
// predicate may accept a change; a budget-limited result is never called minimal.
func reduceLines(lines []numberedLine, budget int, preserves func([]byte) (bool, error)) ([]numberedLine, bool, int, error) {
	kept := slices.Clone(lines)
	granularity, probes := 2, 0
	for len(kept) > 0 {
		width := (len(kept) + granularity - 1) / granularity
		changed := false
		for start := 0; start < len(kept); start += width {
			if probes >= budget {
				return kept, false, probes, nil
			}
			end := min(start+width, len(kept))
			candidate := append(slices.Clone(kept[:start]), kept[end:]...)
			probes++
			accepted, err := preserves(joinLines(candidate))
			if err != nil {
				return kept, false, probes, err
			}
			if accepted {
				kept = candidate
				granularity = max(2, granularity-1)
				changed = true
				break
			}
		}
		if changed {
			continue
		}
		if granularity >= len(kept) {
			return kept, true, probes, nil
		}
		granularity = min(len(kept), granularity*2)
	}
	return kept, true, probes, nil
}

func countDelta(left, right counts) counts {
	return counts{Code: left.Code - right.Code, Comment: left.Comment - right.Comment, Blank: left.Blank - right.Blank}
}

func minimizeInput(opts settings) error {
	if opts.out == "" || opts.minBudget <= 0 || opts.timeout <= 0 {
		return fmt.Errorf("minimization requires -out and positive budget/timeout")
	}
	if opts.locqJSON != "" || opts.tokeiJSON != "" || opts.clocJSON != "" {
		return fmt.Errorf("minimization needs live tools, not imported counts")
	}
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	name, err := normalizePath(root, opts.minimize)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(sourcePath(root, name))
	if err != nil {
		return err
	}
	info, err := os.Lstat(sourcePath(root, name))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("minimization requires a regular source file")
	}
	out, err := filepath.Abs(opts.out)
	if err != nil {
		return err
	}
	if _, err := normalizePath(root, out); err == nil || root == out {
		return fmt.Errorf("minimization output must be outside scan root")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(out, 0755); err != nil {
		return err
	}
	scratch, err := os.MkdirTemp("", "locq-diff-minimize-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	filename := filepath.Base(name)
	tool, binary := "tokei", opts.tokei
	if opts.cloc != "" {
		tool, binary = "cloc", opts.cloc
	}
	if binary == "" {
		return fmt.Errorf("minimization needs a reference executable")
	}
	probeOpts := opts
	probeOpts.root = scratch
	probeOpts.policy = "aligned"
	deadline := time.Now().Add(opts.timeout)
	var baselineLeft, baselineRight fileRecord
	var baselineVersion string
	var expectedDelta counts
	acceptedLeft, acceptedRight := snapshot{}, snapshot{}
	var acceptedLocq, acceptedReference []byte
	probe := func(source []byte, baseline bool) (bool, error) {
		if time.Until(deadline) <= 0 {
			return false, fmt.Errorf("minimization deadline reached")
		}
		if err := os.WriteFile(filepath.Join(scratch, filename), source, 0600); err != nil {
			return false, err
		}
		left, leftJSON, _, err := collectLocq(scratch, opts.rules)
		if err != nil {
			return false, err
		}
		probeOpts.timeout = time.Until(deadline)
		right, rightJSON, _ := collectReference(probeOpts, tool, binary)
		observed, err := os.ReadFile(filepath.Join(scratch, filename))
		if err != nil || !bytes.Equal(observed, source) {
			return false, fmt.Errorf("minimization input changed during probe: %v", err)
		}
		if !left.Complete || !right.Complete {
			return false, fmt.Errorf("minimization probe incomplete: %v %v", left.Errors, right.Errors)
		}
		a, aOK := left.Files[filename]
		b, bOK := right.Files[filename]
		lexical := aOK && bOK && a.Language == b.Language && a.Counts != b.Counts && len(a.Embedded) == 0 && len(b.Embedded) == 0
		if !lexical {
			return false, nil
		}
		if baseline {
			baselineLeft, baselineRight = a, b
			baselineVersion = right.Version
			expectedDelta = countDelta(a.Counts, b.Counts)
		}
		if right.Version != baselineVersion {
			return false, fmt.Errorf("reference version changed during minimization")
		}
		sameLanguages := a.Language == baselineLeft.Language && b.Language == baselineRight.Language
		if !sameLanguages || countDelta(a.Counts, b.Counts) != expectedDelta {
			return false, nil
		}
		acceptedLeft, acceptedRight = left, right
		acceptedLocq, acceptedReference = leftJSON, rightJSON
		return true, nil
	}
	ok, err := probe(data, true)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("isolated original file does not reproduce a same-language lexical difference")
	}
	parts := bytes.SplitAfter(data, []byte{'\n'})
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	lines := []numberedLine{}
	for i, part := range parts {
		lines = append(lines, numberedLine{original: i + 1, data: part})
	}
	kept, minimal, probes, reduceErr := reduceLines(lines, opts.minBudget, func(source []byte) (bool, error) {
		return probe(source, false)
	})
	minimized := joinLines(kept)
	if err := os.MkdirAll(filepath.Join(out, "minimized"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(out, "original"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "minimized", filename), minimized, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "original", filename), data, 0644); err != nil {
		return err
	}
	if err := saveRaw(out, "locq-minimized", acceptedLocq, nil); err != nil {
		return err
	}
	if err := saveRaw(out, tool+"-minimized", acceptedReference, nil); err != nil {
		return err
	}
	numbers := []int{}
	for _, line := range kept {
		numbers = append(numbers, line.original)
	}
	stopReason := "finished"
	if !minimal {
		stopReason = "probe budget exhausted"
	}
	if reduceErr != nil {
		stopReason = reduceErr.Error()
	}
	metadata := struct {
		OriginalPath      string     `json:"original_path"`
		OriginalSHA       string     `json:"original_sha256"`
		MinimizedSHA      string     `json:"minimized_sha256"`
		KeptOriginalLines []int      `json:"kept_original_lines"`
		OneLineMinimal    bool       `json:"one_line_deletion_minimal"`
		Probes            int        `json:"probes"`
		StopReason        string     `json:"stop_reason"`
		Delta             counts     `json:"preserved_count_delta"`
		Tools             []snapshot `json:"tools"`
	}{OriginalPath: name, OriginalSHA: digest(data), MinimizedSHA: digest(minimized), KeptOriginalLines: numbers,
		OneLineMinimal: minimal,
		Probes:         probes,
		StopReason:     stopReason,
		Delta:          expectedDelta,
		Tools:          []snapshot{acceptedLeft, acceptedRight}}
	if err := writeJSON(filepath.Join(out, "minimization.json"), metadata); err != nil {
		return err
	}
	fmt.Printf("Reproduction: %s; probes=%d; one-line-deletion minimal=%t\n",
		filepath.Join(out, "minimized", filename), probes, minimal)
	return reduceErr
}
