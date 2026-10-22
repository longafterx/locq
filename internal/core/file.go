package core

import (
	"crypto/md5"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"sort"

	"github.com/longafterx/locq/internal/lexer"
)

// ClocFile is collecting to line count result.
type ClocFile struct {
	Code     int32  `xml:"code,attr" json:"code"`
	Comments int32  `xml:"comment,attr" json:"comment"`
	Blanks   int32  `xml:"blank,attr" json:"blank"`
	Name     string `xml:"name,attr" json:"name"`
	Lang     string `xml:"language,attr" json:"language"`
}

// ClocFiles is locq result set.
type ClocFiles []ClocFile

func (cf ClocFiles) SortByName() {
	sortFunc := func(i, j int) bool {
		return cf[i].Name < cf[j].Name
	}
	sort.Slice(cf, sortFunc)
}

func (cf ClocFiles) SortByComments() {
	sortFunc := func(i, j int) bool {
		if cf[i].Comments == cf[j].Comments {
			return cf[i].Code > cf[j].Code
		}
		return cf[i].Comments > cf[j].Comments
	}
	sort.Slice(cf, sortFunc)
}

func (cf ClocFiles) SortByBlanks() {
	sortFunc := func(i, j int) bool {
		if cf[i].Blanks == cf[j].Blanks {
			return cf[i].Code > cf[j].Code
		}
		return cf[i].Blanks > cf[j].Blanks
	}
	sort.Slice(cf, sortFunc)
}

func (cf ClocFiles) SortByCode() {
	sortFunc := func(i, j int) bool {
		return cf[i].Code > cf[j].Code
	}
	sort.Slice(cf, sortFunc)
}

// AnalyzeFile synchronously analyzes a single file using explicit rules. It
// returns open, read, close and diagnostic errors; counts may be partial.
func AnalyzeFile(filename string, language *LanguageDefinition, opts *FileOptions) (*ClocFile, error) {
	definition, state, err := prepareFile(language, opts)
	if err != nil {
		return nil, err
	}
	fp, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	file, readErr := analyzeReader(
		filename,
		definition,
		fp,
		state,
	)
	return file, errors.Join(readErr, fp.Close(), state.diagnosticError())
}

func prepareFile(language *LanguageDefinition, opts *FileOptions) (*LanguageDefinition, *runState, error) {
	definition, err := snapshotDefinition(language)
	if err != nil {
		return nil, nil, err
	}
	if opts == nil {
		opts = &FileOptions{}
	}
	state := newRunState(&runConfig{Debug: opts.Debug}, opts.Diagnostics, opts.OnLine)
	return definition, state, nil
}

type hashingReader struct {
	reader io.Reader
	hash   hash.Hash
}

// detectAndAnalyzeFile reuses the detection prefix for line analysis and, when
// deduplication is enabled, hashes physical reads once. Workers bound open files.
func detectAndAnalyzeFile(
	candidate fileCandidate,
	languages *DefinedLanguages,
	opts *runState,
	reader *lineReader,
) (result fileAnalysisResult) {
	result = fileAnalysisResult{fileCandidate: candidate, ignored: true}
	file, err := os.Open(candidate.path)
	if err != nil {
		result.err = err
		return result
	}
	defer func() {
		result.err = errors.Join(result.err, file.Close())
	}()

	var source io.Reader = file
	var hashed *hashingReader
	if opts.Dedup {
		hashed = &hashingReader{reader: file, hash: md5.New()}
		source = hashed
	}
	reader.reset(source)
	defer reader.release()
	var prefix []byte
	var detectionErr error
	ext, ok := detectFileType(candidate.path, opts, func(all bool) ([]byte, error) {
		var readErr error
		if all {
			prefix, readErr = io.ReadAll(reader.reader)
		} else {
			prefix, readErr = reader.next()
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			detectionErr = fmt.Errorf("read %q for language detection: %w", candidate.path, readErr)
		}
		return prefix, readErr
	})
	if detectionErr != nil {
		result.err = detectionErr
		return result
	}
	if !ok {
		return result
	}
	exts := opts.exts
	languageKey, ok := exts[ext]
	if !ok || languages.Langs[languageKey] == nil {
		return result
	}
	if !includeLanguage(languageKey, opts) {
		return result
	}
	result.languageKey = languageKey
	// Detection's bytes are consumed before the next physical read; replaying
	// them requires neither another buffered reader nor another hash update.
	reader.prefix = prefix
	result.clocFile, result.err = analyzeLines(
		candidate.path,
		languages.Langs[languageKey],
		reader,
		opts,
	)
	if result.err != nil {
		return result
	}
	if hashed != nil {
		copy(result.digest[:], hashed.hash.Sum(nil))
	}
	result.ignored = false
	return result
}

func (r *hashingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if _, hashErr := r.hash.Write(p[:n]); hashErr != nil {
			return n, hashErr
		}
	}
	return n, err
}

// AnalyzeReader synchronously analyzes a reader. It leaves the reader open and
// returns partial counts together with read/diagnostic errors.
func AnalyzeReader(filename string, language *LanguageDefinition, source io.Reader, opts *FileOptions) (*ClocFile, error) {
	definition, state, err := prepareFile(language, opts)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, &OptionError{Field: "Reader", Err: fmt.Errorf("reader is required")}
	}
	result, err := analyzeReader(
		filename,
		definition,
		source,
		state,
	)
	return result, errors.Join(err, state.diagnosticError())
}

func analyzeReader(filename string, language *LanguageDefinition, source io.Reader, state *runState) (*ClocFile, error) {
	return analyzeLines(
		filename,
		language,
		newLineReader(source),
		state,
	)
}

func analyzeLines(filename string, language *LanguageDefinition, reader *lineReader, state *runState) (*ClocFile, error) {
	if state.Debug {
		state.diagnosticf("[FILE] file=%q\n", filename)
	}
	file := &ClocFile{Name: filename, Lang: language.Name}
	// Bind a concrete method once per file. Keeping the scanner here avoids
	// allocating its state on the heap through a factory-returned interface.
	var syntax syntaxScanner
	var legacy legacyScanner
	var classify lineClassifier
	if language.syntax != nil {
		syntax = syntaxScanner{scanner: lexer.NewScanner(language.syntax), first: true}
		classify = syntax.classify
	} else {
		legacy = legacyScanner{language: language, first: true, codeFlags: make([]bool, len(language.multiLines))}
		classify = legacy.classify
	}
	for {
		raw, err := reader.next()
		if err != nil && !errors.Is(err, io.EOF) {
			return file, fmt.Errorf("read %q: %w", filename, err)
		}
		if len(raw) == 0 && errors.Is(err, io.EOF) {
			break
		}
		classified := classify(raw)
		recordLine(
			file,
			state,
			classified,
			raw,
		)
	}
	return file, nil
}

func recordLine(file *ClocFile, state *runState, line classifiedLine, raw []byte) {
	var debugKind string
	switch line.kind {
	case CodeLine:
		file.Code++
		debugKind = "CODE"
	case CommentLine:
		file.Comments++
		debugKind = "COMM"
	case BlankLine:
		file.Blanks++
		debugKind = "BLNK"
	}
	if state.onLine != nil {
		state.onLine(
			LineEvent{
				Number: int64(file.Code) + int64(file.Comments) + int64(file.Blanks),
				Kind:   line.kind,
				Text:   string(line.text),
			},
		)
	}
	if state.Debug {
		state.debugLine(
			debugKind,
			file,
			line.inComment,
			raw,
		)
	}
}
