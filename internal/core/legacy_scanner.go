package core

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// legacyScanner preserves the historical marker engine for 50 built-ins and
// explicit custom rules. In particular, blank lines do not consume first-line
// handling, and BOM stripping occurs after the shebang check. Ordinary code and
// comments historically do not consume that flag either. Keep these quirks here;
// changing them requires independent language classification fixtures.
type legacyScanner struct {
	language  *LanguageDefinition
	first     bool
	comments  [][2]string
	codeFlags []bool
}

func (s *legacyScanner) classify(raw []byte) classifiedLine {
	line := bytes.TrimSpace(raw)

	if len(line) == 0 {
		return classifiedLine{kind: BlankLine, text: line, inComment: len(s.comments) > 0}
	}

	// shebang line is 'code'
	if s.first && bytes.HasPrefix(line, []byte("#!")) {
		s.first = false
		return classifiedLine{kind: CodeLine, text: line, inComment: len(s.comments) > 0}
	}

	if len(s.comments) == 0 {
		if s.first {
			line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
		}

		if len(s.language.regexLineComments) > 0 {
		singleloopRegex:
			for _, singleCommentRegex := range s.language.regexLineComments {
				if singleCommentRegex.Match(line) {
					// check if single comment is a prefix of multi comment
					for _, ml := range s.language.multiLines {
						if ml[0] != "" && bytes.HasPrefix(line, []byte(ml[0])) {
							break singleloopRegex
						}
					}
					return classifiedLine{kind: CommentLine, text: line, inComment: len(s.comments) > 0}
				}
			}
		} else {
		singleloop:
			for _, singleComment := range s.language.lineComments {
				if bytes.HasPrefix(line, []byte(singleComment)) {
					// check if single comment is a prefix of multi comment
					for _, ml := range s.language.multiLines {
						if ml[0] != "" && bytes.HasPrefix(line, []byte(ml[0])) {
							break singleloop
						}
					}
					return classifiedLine{kind: CommentLine, text: line, inComment: len(s.comments) > 0}
				}
			}
		}

		if len(s.language.multiLines) == 0 {
			return classifiedLine{kind: CodeLine, text: line, inComment: len(s.comments) > 0}
		}
	}

	if len(s.comments) == 0 && !containsCommentBytes(line, s.language.multiLines) {
		return classifiedLine{kind: CodeLine, text: line, inComment: len(s.comments) > 0}
	}

	lenLine := len(line)
	singleDelimiter := len(s.language.multiLines) == 1 && len(s.language.multiLines[0]) == 2
	if singleDelimiter && s.language.multiLines[0][0] == "" {
		return classifiedLine{kind: CodeLine, text: line, inComment: len(s.comments) > 0}
	}
	clear(s.codeFlags)
	for pos := 0; pos < lenLine; {
		for idx, ml := range s.language.multiLines {
			begin, end := ml[0], ml[1]
			lenBegin := len(begin)

			hasBegin := pos+lenBegin <= lenLine && bytes.HasPrefix(line[pos:], []byte(begin))
			if hasBegin && (begin != end || len(s.comments) == 0) {
				pos += lenBegin
				s.comments = append(s.comments, [2]string{begin, end})
				continue
			}

			if n := len(s.comments); n > 0 {
				last := s.comments[n-1]
				if pos+len(last[1]) <= lenLine && bytes.HasPrefix(line[pos:], []byte(last[1])) {
					s.comments = s.comments[:n-1]
					pos += len(last[1])
				}
			} else if pos < lenLine {
				r, _ := utf8.DecodeRune(line[pos:])
				if !unicode.IsSpace(r) {
					s.codeFlags[idx] = true
				}
			}
		}
		pos++
	}

	isCode := true
	for _, b := range s.codeFlags {
		if !b {
			isCode = false
		}
	}

	if isCode {
		return classifiedLine{kind: CodeLine, text: line, inComment: len(s.comments) > 0}
	}
	return classifiedLine{kind: CommentLine, text: line, inComment: len(s.comments) > 0}
}
