package core

// Result defined processing result.
type Result struct {
	Total         *Language
	Files         map[string]*ClocFile
	Languages     map[string]*Language
	MaxPathLength int
}

// Analyze counts source lines. Empty paths return an empty result. Invalid
// options fail before I/O. Unreadable inputs are skipped with diagnostics; writer
// failures are returned after all workers finish. Traversal order determines
// which identical file Dedup retains, regardless of worker completion order.
func Analyze(paths []string, opts *Options) (*Result, error) {
	config, err := opts.prepare()
	if err != nil {
		return nil, err
	}
	state := newRunState(config, nil, nil)
	if opts != nil && opts.Diagnostics != nil {
		state.diagnostics.writer = opts.Diagnostics
	}

	total := &Language{Name: "TOTAL", Files: []string{}}
	languages, clocFiles, err := scanAndAnalyzeFiles(paths, config.languages, state)
	if err != nil {
		return nil, err
	}
	if err := state.diagnosticError(); err != nil {
		return nil, err
	}
	maxPathLen := 0
	for _, lang := range languages {
		for _, file := range lang.Files {
			l := len(file)
			if maxPathLen < l {
				maxPathLen = l
			}
		}
	}
	for _, language := range languages {
		files := int32(len(language.Files))
		if len(language.Files) <= 0 {
			continue
		}

		total.Total += files
		total.Blanks += language.Blanks
		total.Comments += language.Comments
		total.Code += language.Code
	}

	return &Result{
		Total:         total,
		Files:         clocFiles,
		Languages:     languages,
		MaxPathLength: maxPathLen,
	}, nil
}
