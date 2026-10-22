package core

import (
	"bytes"

	"github.com/longafterx/locq/internal/lexer"
)

type classifiedLine struct {
	kind      LineKind
	text      []byte
	inComment bool
}

// lineClassifier is the per-line contract shared by the two concrete engines.
type lineClassifier func([]byte) classifiedLine

type syntaxScanner struct {
	scanner lexer.Scanner
	first   bool
}

func (s *syntaxScanner) classify(raw []byte) classifiedLine {
	line := bytes.TrimSpace(raw)
	input := raw
	if s.first {
		input = bytes.TrimPrefix(input, []byte("\xef\xbb\xbf"))
		line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
	}
	shebang := s.first && bytes.HasPrefix(line, []byte("#!"))
	s.first = false
	if shebang {
		return classifiedLine{kind: CodeLine, text: line, inComment: s.scanner.InComment()}
	}
	isCode := s.scanner.ScanLine(input, line)
	kind := CommentLine
	if len(line) == 0 {
		kind = BlankLine
	} else if isCode {
		kind = CodeLine
	}
	return classifiedLine{kind: kind, text: line, inComment: s.scanner.InComment()}
}
