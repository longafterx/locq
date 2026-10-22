package lexer

// Rules is immutable after initialization. A scanner copies only its
// state, so definitions can be shared safely by concurrent file workers.
type Rules struct {
	lineComments  []string
	blockComments []blockCommentRule
	strings       []stringRule
	docQuotes     []stringRule
	starts        [4]uint64
	// fastSimple is safe only for // and /* */ comment rules. Other rule sets
	// may have additional markers that a slash-only search would miss.
	fastSimple   bool
	rawDelimiter byte
	lineSplice   bool
	special      syntaxKind
	raw          rawKind
	continuation bool
	fortran      bool
	foldCase     bool
}

type syntaxKind uint8

const (
	syntaxGeneric syntaxKind = iota
	syntaxJavaScript
	syntaxPython
	syntaxRust
)

type rawKind uint8

const (
	rawNone rawKind = iota
	rawCpp
	rawRust
)

type blockCommentRule struct {
	open, close string
	nested      bool
	lineStart   bool
}

type stringRule struct {
	open, close string
	escape      byte
	multiline   bool
	doubled     bool
}

// String indices address ordinary/verbatim quotes first, then doc quotes. Keep
// documentation delimiters separate from their context-dependent classification.
func (r *Rules) quote(index int) stringRule {
	if index < len(r.strings) {
		return r.strings[index]
	}
	return r.docQuotes[index-len(r.strings)]
}

func BuiltIn(name string) *Rules {
	return builtInSyntax[name]
}
