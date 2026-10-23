//go:build integration

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func buildReferenceHelper(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	source := `package main
import("fmt";"os";"time")
func main(){
 if len(os.Args)>1&&os.Args[1]=="--version"{
 if os.Getenv("LOCQ_DIFFCHECK_TEST_MODE")=="version-failure"{os.Exit(2)}
 fmt.Println("tokei test-reference-1");return
 }
 switch os.Getenv("LOCQ_DIFFCHECK_TEST_MODE"){
 case "timeout":time.Sleep(10*time.Second)
 case "malformed":fmt.Print("not JSON");return
 case "nonzero":defer os.Exit(2)
 case "inaccurate":fmt.Print("{\"Total\":{\"inaccurate\":true},\"Go\":{\"reports\":[]}}");return
 case "null-reports":fmt.Print("{\"Go\":{\"reports\":null}}");return
 case "diagnostic":fmt.Fprintln(os.Stderr,"file timed out")
 case "mutate":if err:=os.WriteFile("a.go",[]byte("changed\n"),0600);err!=nil{panic(err)}
 }
 fmt.Print("{\"Go\":{\"reports\":[{\"name\":\"a.go\",\"stats\":{\"code\":0,\"comments\":2,\"blanks\":0}}]}}")
}`
	filename := filepath.Join(root, "main.go")
	if err := os.WriteFile(filename, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "reference")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", binary, filename).CombinedOutput(); err != nil {
		t.Fatalf("build reference: %v: %s", err, output)
	}
	return binary
}

func readReport(t *testing.T, directory string) report {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRunAndReplay(t *testing.T) {
	binary := buildReferenceHelper(t)
	root := filepath.Join(t.TempDir(), "input")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("value\n// real\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "report")
	opts := settings{root: root,
		out:     out,
		rules:   "../../internal/lexer/rules_generated.go",
		tokei:   binary,
		policy:  "aligned",
		timeout: time.Second}
	if err := run(opts); err != nil {
		t.Fatal(err)
	}
	r := readReport(t, out)
	if !r.InputStable || !r.Comparisons[0].Complete || r.Comparisons[0].Categories[lexicalCount] != 1 {
		t.Fatalf("report=%+v", r)
	}
	d := r.Comparisons[0].Differences[0]
	if len(d.Lines) != 2 || d.Lines[0].Reference != "" {
		t.Fatal("reference line classifications were fabricated")
	}
	if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(d.Reproduction))); err != nil {
		t.Fatal(err)
	}
	if err := run(opts); err == nil {
		t.Fatal("existing report directory overwritten")
	}
	replay := opts
	replay.out = filepath.Join(t.TempDir(), "replay")
	replay.tokei = ""
	replay.locqJSON = filepath.Join(out, "locq.json")
	replay.tokeiJSON = filepath.Join(out, "tokei.json")
	replay.metadata = filepath.Join(out, "report.json")
	relocated := t.TempDir()
	if err := os.WriteFile(filepath.Join(relocated, "a.go"), []byte("value\n// real\n"), 0600); err != nil {
		t.Fatal(err)
	}
	replay.root = relocated
	if err := run(replay); err != nil {
		t.Fatal(err)
	}
	if got := readReport(t, replay.out); got.Input.SHA256 != r.Input.SHA256 || got.Comparisons[0].Categories[lexicalCount] != 1 {
		t.Fatal("replay changed observations")
	}
	second := replay
	second.out = filepath.Join(t.TempDir(), "second-replay")
	second.metadata = filepath.Join(replay.out, "report.json")
	second.locqJSON = filepath.Join(replay.out, "locq.json")
	second.tokeiJSON = filepath.Join(replay.out, "tokei.json")
	if err := run(second); err != nil {
		t.Fatal(err)
	}
	if got := readReport(t, second.out); !got.Comparisons[0].Complete ||
		got.Comparisons[0].Categories[lexicalCount] != 1 {
		t.Fatal("replaying a relocated replay lost original raw paths")
	}
	if err := os.WriteFile(replay.tokeiJSON, []byte(`{"Go":{"reports":[]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	replay.out = filepath.Join(t.TempDir(), "changed")
	if err := run(replay); err == nil {
		t.Fatal("modified imported JSON accepted with old execution metadata")
	}
}

func TestRunReportsIncompleteReferences(t *testing.T) {
	binary := buildReferenceHelper(t)
	for _, tc := range []struct {
		name, category string
		stable         bool
	}{
		{name: "nonzero", category: ioOrTimeout, stable: true},
		{name: "version-failure", category: ioOrTimeout, stable: true},
		{name: "inaccurate", category: ioOrTimeout, stable: true},
		{name: "null-reports", category: unclassified, stable: true},
		{name: "timeout", category: ioOrTimeout, stable: true},
		{name: "malformed", category: unclassified, stable: true},
		{name: "diagnostic", category: ioOrTimeout, stable: true},
		{name: "mutate", category: ioOrTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LOCQ_DIFFCHECK_TEST_MODE", tc.name)
			root := filepath.Join(t.TempDir(), "input")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("value\n// real\n"), 0600); err != nil {
				t.Fatal(err)
			}
			opts := settings{root: root,
				out:   filepath.Join(t.TempDir(), "report"),
				rules: "../../internal/lexer/rules_generated.go",
				tokei: binary, policy: "aligned", timeout: 250 * time.Millisecond, check: true}
			if err := run(opts); err == nil {
				t.Fatal("strict run accepted an incomplete measurement")
			}
			r := readReport(t, opts.out)
			c := r.Comparisons[0]
			if c.Complete || c.Categories[tc.category] == 0 || c.Categories[fileSet] != 0 {
				t.Fatalf("partial output misclassified: %+v", c)
			}
			if r.InputStable != tc.stable {
				t.Fatal("input mutation was not recorded")
			}
			if !hasUnresolved(r) {
				t.Fatal("incomplete report accepted by strict check")
			}
		})
	}
}

func TestMinimizeRejectsChangingInput(t *testing.T) {
	binary := buildReferenceHelper(t)
	t.Setenv("LOCQ_DIFFCHECK_TEST_MODE", "mutate")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("value\n// real\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "minimized")
	err := minimizeInput(settings{root: root, out: out, minimize: "a.go", minBudget: 10,
		rules: "../../internal/lexer/rules_generated.go", tokei: binary, timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "input changed") {
		t.Fatalf("mutated probe accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "minimization.json")); !os.IsNotExist(err) {
		t.Fatal("unverified reproduction saved")
	}
}
