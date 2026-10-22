package core

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Each analysis owns a sink so callers may supply an ordinary bytes.Buffer.
// Never mutate the caller's options or share a lock by copying it with options.
type diagnosticSink struct {
	mu     sync.Mutex
	writer io.Writer
	err    error
}

func newRunState(config *runConfig, writer io.Writer, observer func(LineEvent)) *runState {
	if writer == nil {
		writer = os.Stderr
	}
	return &runState{runConfig: config, diagnostics: &diagnosticSink{writer: writer}, onLine: observer}
}

func (opts *runState) diagnosticf(format string, args ...any) {
	sink := opts.diagnostics
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.err == nil {
		message := fmt.Sprintf(format, args...)
		var n int
		n, sink.err = io.WriteString(sink.writer, message)
		if sink.err == nil && n != len(message) {
			sink.err = io.ErrShortWrite
		}
	}
}

func (opts *runState) debugLine(kind string, file *ClocFile, inComment bool, line []byte) {
	// Every physical line is classified exactly once. Derive its number only
	// for debug output so ordinary counting needs no additional per-line state.
	lineNumber := int64(file.Code) + int64(file.Comments) + int64(file.Blanks)
	// Quoting keeps paths and source text, including control characters, in a
	// single record. diagnosticf serializes each write, not whole files.
	opts.diagnosticf(
		"[%s] file=%q line=%d code=%d comment=%d blank=%d in_comment=%t text=%q\n",
		kind,
		file.Name,
		lineNumber,
		file.Code,
		file.Comments,
		file.Blanks,
		inComment,
		line,
	)
}

func (opts *runState) warn(err error) {
	if err != nil {
		opts.diagnosticf("warning: %v\n", err)
	}
}

func (opts *runState) diagnosticError() error {
	sink := opts.diagnostics
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.err != nil {
		return fmt.Errorf("write diagnostics: %w", sink.err)
	}
	return nil
}
