package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/longafterx/locq"
	"github.com/longafterx/locq/internal/lexertest"
)

func inspectInput(root string) (inputState, error) {
	state := inputState{Root: root, Files: map[string]string{}}
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink input %q: export a snapshot containing regular files", filename)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular input %q", filename)
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		state.Files[name] = digest(data)
		return nil
	})
	if err != nil {
		return state, fmt.Errorf("fingerprint input: %w", err)
	}
	data, err := json.Marshal(state.Files)
	if err != nil {
		return state, err
	}
	state.SHA256 = digest(data)
	// Only label a repository whose root is the actual scanned root.
	command := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel")
	if top, err := command.Output(); err == nil && filepath.Clean(strings.TrimSpace(string(top))) == filepath.Clean(root) {
		commit, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
		if err != nil {
			return state, err
		}
		status, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
		if err != nil {
			return state, err
		}
		state.Commit = strings.TrimSpace(string(commit))
		state.GitStatus = string(status)
	}
	return state, nil
}

var executableDigest = sync.OnceValue(func() string {
	filename, err := os.Executable()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return ""
	}
	return digest(data)
})

func locqVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
		return info.Main.Version
	}
	return "devel"
}

func collectLocq(root, rules string) (snapshot, []byte, []byte, error) {
	ruleData, err := os.ReadFile(rules)
	if err != nil {
		return snapshot{}, nil, nil, err
	}
	var diagnostics bytes.Buffer
	result, err := locq.Analyze([]string{root}, &locq.Options{Diagnostics: &diagnostics})
	if err != nil {
		return snapshot{}, nil, diagnostics.Bytes(), err
	}
	s := snapshot{Tool: "locq",
		Version:      locqVersion(),
		BuildSHA256:  executableDigest(),
		RawRoot:      root,
		RulesVersion: lexertest.SemanticsVersion + ":" + digest(ruleData),
		Command:      []string{"locq.Analyze", root, "default options"}, Files: map[string]fileRecord{},
		Complete: diagnostics.Len() == 0, Errors: []string{}}
	if diagnostics.Len() > 0 {
		s.Errors = append(s.Errors, diagnostics.String())
	}
	files := []locq.ClocFile{}
	for _, file := range result.Files {
		files = append(files, *file)
		err := addFile(&s, root, fileRecord{Path: file.Name, Language: file.Lang,
			Counts: counts{Code: int64(file.Code), Comment: int64(file.Comments), Blank: int64(file.Blanks)}})
		if err != nil {
			return s, nil, diagnostics.Bytes(), err
		}
	}
	slices.SortFunc(files, func(a, b locq.ClocFile) int { return strings.Compare(a.Name, b.Name) })
	data, err := json.Marshal(struct {
		Files []locq.ClocFile `json:"files"`
	}{Files: files})
	return s, data, diagnostics.Bytes(), err
}

func collectReference(opts settings, tool, binary string) (snapshot, []byte, []byte) {
	s := snapshot{Tool: tool, Version: "unknown", Command: []string{}, Complete: false, RawRoot: opts.root,
		Files: map[string]fileRecord{}, Errors: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	version, err := exec.CommandContext(ctx, binary, "--version").CombinedOutput()
	if err != nil {
		s.Errors = append(s.Errors, fmt.Sprintf("%s version: %v: %s", tool, err, version))
		return s, []byte{}, version
	}
	s.Version = strings.TrimSpace(string(version))
	args := []string{"--files", "--output", "json"}
	if tool == "cloc" {
		args = []string{"--by-file", "--json"}
		if opts.policy == "aligned" {
			args = append(args, "--skip-uniqueness", "--timeout", "0")
		}
	}
	if tool == "tokei" && opts.policy == "aligned" {
		args = append(args, "--no-ignore", "--hidden")
	}
	args = append(args, ".")
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = opts.root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	runErr := command.Run()
	parsed, parseErr := decodeSnapshot(tool, opts.root, stdout.Bytes())
	if parseErr == nil {
		s.Files = parsed.Files
		s.Complete = parsed.Complete
		s.Errors = parsed.Errors
	}
	s.Command = append([]string{binary}, args...)
	if parseErr != nil {
		s.FailureCategory = unclassified
		s.Errors = append(s.Errors, "decode output: "+parseErr.Error())
	}
	if runErr != nil {
		s.FailureCategory = ioOrTimeout
		s.Errors = append(s.Errors, fmt.Sprintf("execute: %v (deadline: %v)", runErr, ctx.Err()))
	}
	if stderr.Len() > 0 {
		s.Errors = append(s.Errors, "stderr: "+stderr.String())
	}
	if parseErr != nil || runErr != nil || stderr.Len() > 0 {
		s.Complete = false
	}
	return s, stdout.Bytes(), stderr.Bytes()
}

func importSnapshot(opts settings, tool, filename string, input inputState) (snapshot, []byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return snapshot{}, nil, err
	}
	metadata, err := os.ReadFile(opts.metadata)
	if err != nil {
		return snapshot{}, data, fmt.Errorf("imports require execution metadata: %w", err)
	}
	var prior report
	if err := json.Unmarshal(metadata, &prior); err != nil {
		return snapshot{}, data, err
	}
	if prior.SchemaVersion != reportVersion || prior.Semantics != lexertest.SemanticsVersion {
		return snapshot{}, data, fmt.Errorf("import metadata schema or semantics version differs")
	}
	if !prior.InputStable {
		return snapshot{}, data, fmt.Errorf("cannot replay observations from unstable input")
	}
	if prior.Input.SHA256 != input.SHA256 {
		return snapshot{}, data, fmt.Errorf("import metadata input fingerprint differs")
	}
	for _, source := range prior.Tools {
		if source.Tool != tool {
			continue
		}
		if source.RawSHA256 != digest(data) {
			return snapshot{}, data, fmt.Errorf("imported %s JSON digest differs from metadata", tool)
		}
		// Preserve the raw path root through any number of relocated replays.
		rawRoot := source.RawRoot
		if rawRoot == "" {
			rawRoot = prior.Input.Root
		}
		s, parseErr := decodeSnapshot(tool, rawRoot, data)
		if parseErr != nil {
			s.Complete = false
			s.FailureCategory = unclassified
			s.Errors = append(s.Errors, "decode imported output: "+parseErr.Error())
		}
		s.RawRoot = rawRoot
		s.Version = source.Version
		s.BuildSHA256 = source.BuildSHA256
		if parseErr == nil {
			s.FailureCategory = source.FailureCategory
		}
		s.RulesVersion = source.RulesVersion
		s.Command = source.Command
		s.Complete = s.Complete && source.Complete
		s.Errors = append(s.Errors, source.Errors...)
		return s, data, nil
	}
	return snapshot{}, data, fmt.Errorf("execution metadata lacks %s", tool)
}

func run(opts settings) error {
	if opts.out == "" {
		return fmt.Errorf("-out must name a new report directory")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("-timeout must be positive")
	}
	if opts.policy != "defaults" && opts.policy != "aligned" {
		return fmt.Errorf("-policy must be defaults or aligned")
	}
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	opts.root = root
	out, err := filepath.Abs(opts.out)
	if err != nil {
		return err
	}
	opts.out = out
	if _, err := normalizePath(root, out); err == nil || root == out {
		return fmt.Errorf("report directory must be outside scan root")
	}
	known, err := readKnown(opts.known)
	if err != nil {
		return err
	}
	input, err := inspectInput(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(out, 0755); err != nil {
		return fmt.Errorf("new report directory: %w", err)
	}
	r := report{SchemaVersion: reportVersion, Semantics: lexertest.SemanticsVersion, Input: input,
		InputStable: true, Tools: []snapshot{}, Comparisons: []comparison{}, StaleKnown: []string{}}
	if opts.locqJSON != "" {
		s, data, err := importSnapshot(opts, "locq", opts.locqJSON, input)
		if err != nil {
			return err
		}
		s.RawSHA256 = digest(data)
		r.Tools = append(r.Tools, s)
		if err := os.WriteFile(filepath.Join(out, "locq.json"), data, 0644); err != nil {
			return err
		}
	} else {
		s, data, stderr, err := collectLocq(root, opts.rules)
		if err != nil {
			return err
		}
		s.RawSHA256 = digest(data)
		r.Tools = append(r.Tools, s)
		if err := saveRaw(out, "locq", data, stderr); err != nil {
			return err
		}
	}
	for _, ref := range []struct{ tool, binary, filename string }{
		{tool: "tokei", binary: opts.tokei, filename: opts.tokeiJSON},
		{tool: "cloc", binary: opts.cloc, filename: opts.clocJSON},
	} {
		if ref.binary == "" && ref.filename == "" {
			continue
		}
		var s snapshot
		var data, stderr []byte
		if ref.filename != "" {
			s, data, err = importSnapshot(opts, ref.tool, ref.filename, input)
			if err != nil {
				return err
			}
		} else {
			s, data, stderr = collectReference(opts, ref.tool, ref.binary)
		}
		s.RawSHA256 = digest(data)
		r.Tools = append(r.Tools, s)
		if err := saveRaw(out, ref.tool, data, stderr); err != nil {
			return err
		}
	}
	if opts.golden != "" {
		s, err := collectGolden(opts.golden, input)
		if err != nil {
			return err
		}
		r.Tools = append(r.Tools, s)
	}
	if len(r.Tools) < 2 {
		return fmt.Errorf("select at least one reference tool or JSON input")
	}
	after, err := inspectInput(root)
	if err != nil {
		return err
	}
	r.InputStable = input.SHA256 == after.SHA256 && input.Commit == after.Commit && input.GitStatus == after.GitStatus
	if !r.InputStable {
		for i := range r.Tools {
			r.Tools[i].Complete = false
			r.Tools[i].Errors = append(r.Tools[i].Errors, "input changed during measurement")
		}
	}
	used := map[string]bool{}
	for _, ref := range r.Tools[1:] {
		result := compare(r.Tools[0], ref, input)
		if ref.Tool == "contract" {
			if err := compareGoldenLines(opts, &result, r.Tools[0], ref, input); err != nil {
				return err
			}
		}
		if err := applyKnown(&result, r.Tools[0], ref, known, used); err != nil {
			return err
		}
		if err := attachTraces(opts, &result, r.Tools[0]); err != nil {
			return err
		}
		r.Comparisons = append(r.Comparisons, result)
	}
	final, err := inspectInput(root)
	if err != nil {
		return err
	}
	if final.SHA256 != input.SHA256 || final.Commit != input.Commit || final.GitStatus != input.GitStatus {
		r.InputStable = false
		for i := range r.Tools {
			r.Tools[i].Complete = false
			r.Tools[i].Errors = append(r.Tools[i].Errors, "input changed while collecting traces")
		}
		for i := range r.Comparisons {
			r.Comparisons[i].Complete = false
			r.Comparisons[i].Categories[ioOrTimeout]++
			r.Comparisons[i].Differences = append(r.Comparisons[i].Differences, difference{
				Category: ioOrTimeout, Reason: "Input changed while collecting reproduction traces."})
		}
	}
	for _, rule := range known {
		if !used[rule.ID] {
			r.StaleKnown = append(r.StaleKnown, rule.ID)
		}
	}
	slices.Sort(r.StaleKnown)
	if err := writeJSON(filepath.Join(out, "report.json"), r); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "report.md"), []byte(markdownReport(r)), 0644); err != nil {
		return err
	}
	fmt.Printf("Report: %s\n", filepath.Join(out, "report.md"))
	if opts.check && hasUnresolved(r) {
		return fmt.Errorf("correctness report contains unresolved differences or incomplete execution")
	}
	return nil
}

func saveRaw(out, tool string, stdout, stderr []byte) error {
	if err := os.WriteFile(filepath.Join(out, tool+".json"), stdout, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, tool+".stderr.txt"), stderr, 0644)
}

func writeJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, append(data, '\n'), 0644)
}

func hasUnresolved(r report) bool {
	if !r.InputStable || len(r.StaleKnown) > 0 {
		return true
	}
	for _, comparison := range r.Comparisons {
		if !comparison.Complete {
			return true
		}
		for _, d := range comparison.Differences {
			if d.Category != knownSemanticDiff || !d.Allowed {
				return true
			}
		}
	}
	return false
}
