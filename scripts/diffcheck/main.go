package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type settings struct {
	root, out, known, rules                 string
	tokei, cloc, policy                     string
	locqJSON, tokeiJSON, clocJSON, metadata string
	exportCorpus, matrix, golden            string
	minimize                                string
	minBudget                               int
	timeout                                 time.Duration
	check                                   bool
}

func main() {
	var opts settings
	flag.StringVar(&opts.root, "root", ".", "scan root; output must be outside it")
	flag.StringVar(&opts.out, "out", "", "new report directory (never overwritten)")
	flag.StringVar(
		&opts.known,
		"known",
		"scripts/diffcheck/known-differences.json",
		"exact known-difference records; empty disables",
	)
	flag.StringVar(&opts.rules, "rules", "internal/lexer/rules_generated.go", "compiled rule source to fingerprint")
	flag.StringVar(&opts.tokei, "tokei", "tokei", "tokei executable; empty disables")
	flag.StringVar(&opts.cloc, "cloc", "", "optional cloc executable")
	flag.StringVar(
		&opts.policy,
		"policy",
		"defaults",
		"reference input policy: defaults or aligned (disable ignores and deduplication)",
	)
	flag.StringVar(&opts.locqJSON, "locq-json", "", "import per-file locq JSON")
	flag.StringVar(&opts.tokeiJSON, "tokei-json", "", "import per-file tokei JSON")
	flag.StringVar(&opts.clocJSON, "cloc-json", "", "import per-file cloc JSON")
	flag.StringVar(&opts.metadata, "metadata", "", "prior report.json binding imported JSON digests, versions and completeness")
	flag.StringVar(&opts.exportCorpus, "export-corpus", "", "export independent golden corpus into a new directory, then exit")
	flag.StringVar(&opts.matrix, "matrix", "", "write language coverage Markdown, then exit")
	flag.StringVar(&opts.golden, "golden", "", "optional exported golden.json for line-level contract checks")
	flag.DurationVar(&opts.timeout, "timeout", 2*time.Minute, "deadline for each external tool")
	flag.BoolVar(&opts.check, "check", false, "fail on unknown/disallowed differences, stale rules or incomplete runs")
	flag.StringVar(&opts.minimize, "minimize", "", "reduce one relative source path while preserving its lexical count delta")
	flag.IntVar(&opts.minBudget, "minimize-budget", 100, "maximum line-deletion probes")
	flag.Parse()
	var err error
	switch {
	case flag.NArg() != 0:
		err = fmt.Errorf("unexpected positional arguments")
	case opts.minimize != "":
		err = minimizeInput(opts)
	case opts.exportCorpus != "":
		err = exportCorpus(opts.exportCorpus)
	case opts.matrix != "":
		err = writeMatrix(opts.matrix)
	default:
		err = run(opts)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
