package core

import (
	"fmt"
	"io"
	"maps"
	"runtime"
)

const MaxWorkers = 64

// Options configures directory analysis. Nil and the zero value use built-in
// languages, automatic concurrency and no deduplication. Inputs are copied once;
// callers must not mutate options, maps or definitions during a call.
type Options struct {
	// Workers is zero for automatic selection, or 1-MaxWorkers.
	Workers int
	// Dedup retains the first discovered file for each distinct content hash.
	Dedup bool
	// Include and Exclude contain case-sensitive globs. Bare names match at any
	// depth; paths are relative to each root; leading / anchors to the root;
	// trailing / matches directories only. Matching directories includes their
	// descendants. Includes are ORed, exclusions always win, and unmatched parents
	// remain reachable. An empty Include accepts all files. Supports ** and {a,b}.
	Include []string
	Exclude []string
	// Language filters use exact names. Unknown names are rejected. Exclusion wins.
	IncludeLangs []string
	ExcludeLangs []string
	// Definitions adds or replaces built-in language definitions for this call.
	// Keys must equal definition names. Extensions adds or overrides detection keys
	// (extensions without dots or existing detector keys), mapped to defined names.
	Definitions map[string]*LanguageDefinition
	Extensions  map[string]string
	Debug       bool
	// Diagnostics receives serialized warnings/debug output. Nil uses stderr.
	// A writer shared by multiple calls must be concurrency-safe.
	Diagnostics io.Writer
}

// FileOptions configures synchronous single-input analysis. It deliberately has
// no directory filters, deduplication or worker settings.
type FileOptions struct {
	Debug       bool
	Diagnostics io.Writer
	// OnLine runs on the caller's goroutine in physical line order. Text owns its
	// storage and contains parser-normalized text (trimmed whitespace and engine
	// BOM handling), without a newline. Number is one-based. Events already sent
	// cannot be undone on a later read error; incomplete failing lines are omitted.
	OnLine func(LineEvent)
}

type LineKind uint8

const (
	CodeLine LineKind = iota
	CommentLine
	BlankLine
)

type LineEvent struct {
	Number int64
	Kind   LineKind
	Text   string
}

// OptionError identifies invalid caller configuration, before any I/O.
type OptionError struct {
	Field string
	Err   error
}

func (e *OptionError) Error() string { return fmt.Sprintf("invalid %s: %v", e.Field, e.Err) }
func (e *OptionError) Unwrap() error { return e.Err }

// runConfig is a validated snapshot. Workers and readers never mutate it.
type runConfig struct {
	Workers      int
	Dedup        bool
	Debug        bool
	IncludeLangs map[string]struct{}
	ExcludeLangs map[string]struct{}
	exts         map[string]string
	languages    *DefinedLanguages
	filter       *pathFilter
}

// runState owns one invocation's mutable diagnostics and single-input observer.
// Directory analysis never installs an observer.
type runState struct {
	*runConfig
	diagnostics *diagnosticSink
	onLine      func(LineEvent)
}

// ValidateWorkers shares worker validation with the command's informational
// exits, which do not otherwise prepare a scan configuration.
func ValidateWorkers(workers int) error {
	if workers < 0 || workers > MaxWorkers {
		return fmt.Errorf("must be zero (automatic) or between 1 and %d", MaxWorkers)
	}
	return nil
}

func (opts *Options) prepare() (*runConfig, error) {
	if opts == nil {
		opts = &Options{}
	}
	if err := ValidateWorkers(opts.Workers); err != nil {
		return nil, &OptionError{Field: "Workers", Err: err}
	}

	workers := opts.Workers
	if workers == 0 {
		workers = min(runtime.GOMAXPROCS(0), MaxWorkers)
	}
	config := &runConfig{Workers: workers, Dedup: opts.Dedup, Debug: opts.Debug,
		languages: NewDefinedLanguages(), exts: builtInExtensions,
		IncludeLangs: map[string]struct{}{}, ExcludeLangs: map[string]struct{}{}}
	for name, definition := range opts.Definitions {
		if definition == nil || name != definition.Name {
			return nil, &OptionError{Field: "Definitions", Err: fmt.Errorf("key %q must match a nonnil definition's name", name)}
		}
		snapshot, err := snapshotDefinition(definition)
		if err != nil {
			return nil, &OptionError{Field: "Definitions", Err: err}
		}
		config.languages.Langs[name] = snapshot
	}
	// Built-in mappings are private and immutable. Clone only when applying
	// caller overrides; ordinary scans need no extra copy of the whole catalog.
	if len(opts.Extensions) > 0 {
		config.exts = maps.Clone(builtInExtensions)
	}
	for extension, name := range opts.Extensions {
		if _, ok := config.languages.Langs[name]; !ok {
			return nil, &OptionError{Field: "Extensions", Err: fmt.Errorf("%q maps to unknown language %q", extension, name)}
		}
		config.exts[extension] = name
	}
	for _, filter := range []struct {
		field  string
		names  []string
		target map[string]struct{}
	}{
		{"IncludeLangs", opts.IncludeLangs, config.IncludeLangs}, {"ExcludeLangs", opts.ExcludeLangs, config.ExcludeLangs},
	} {
		for _, name := range filter.names {
			if _, ok := config.languages.Langs[name]; !ok {
				return nil, &OptionError{Field: filter.field, Err: fmt.Errorf("unknown language %q", name)}
			}
			filter.target[name] = struct{}{}
		}
	}
	var err error
	config.filter, err = preparePathFilter(opts.Include, opts.Exclude)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func snapshotDefinition(definition *LanguageDefinition) (*LanguageDefinition, error) {
	if definition == nil || definition.Name == "" {
		return nil, &OptionError{Field: "Language", Err: fmt.Errorf("a named language definition is required")}
	}
	for _, pair := range definition.multiLines {
		if len(pair) != 2 || ((pair[0] == "") != (pair[1] == "")) {
			return nil, &OptionError{Field: "Language", Err: fmt.Errorf("%q has an invalid block comment pair", definition.Name)}
		}
	}
	result := NewLanguageDefinition(definition.Name, definition.lineComments, definition.multiLines)
	result.syntax = definition.syntax
	result.regexLineComments = append(result.regexLineComments, definition.regexLineComments...)
	return result, nil
}
