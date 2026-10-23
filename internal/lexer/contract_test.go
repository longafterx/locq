package lexer

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/longafterx/locq/internal/lexertest"
)

// Semantic fields only: optimization flags/indexes must not change the contract.
func ruleSignature(r *Rules) string {
	surface := fmt.Sprintf("%#v|%#v|%#v|%#v|%d|%d|%t|%t|%t|%t",
		r.lineComments, r.blockComments, r.strings, r.docQuotes, r.special, r.raw,
		r.lineSplice, r.continuation, r.fortran, r.foldCase)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(surface)))
}

func TestLanguageMatrix(t *testing.T) {
	t.Parallel()
	for name, entry := range lexertest.LanguageMatrix {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rules := BuiltIn(name)
			if entry.LegacyReason != "" {
				if rules != nil || entry.Signature != "" || entry.Tier != 3 {
					t.Fatal("legacy contract was activated without a matrix review")
				}
				return
			}
			if rules == nil {
				t.Fatal("matrix promises an active scanner")
			}
			if signature := ruleSignature(rules); signature != entry.Signature {
				t.Fatalf("semantic rules changed; independently review fixtures and matrix: got signature %s", signature)
			}
			if features := ruleFeatures(rules); !slices.Equal(features, entry.Features) {
				t.Fatalf("matrix features differ from declared rules: got %v, want %v", features, entry.Features)
			}
			for _, fixture := range ruleCases(name, rules) {
				t.Run(fixture.Name, func(t *testing.T) {
					assertScannerKinds(t, rules, fixture.Source, fixture.Kinds)
				})
			}
		})
	}
	for name := range builtInSyntax {
		if _, ok := lexertest.LanguageMatrix[name]; !ok {
			t.Fatalf("active language %q has no reviewed matrix entry", name)
		}
	}
}

// Sources come from declared markers; expectations come from the contract, not
// scanner output. The separately checked signature prevents a changed marker
// from silently rewriting what these templates test.
func ruleCases(language string, rules *Rules) []lexertest.Case {
	cases := []lexertest.Case{}
	add := func(name, source, kinds string) {
		cases = append(cases, lexertest.Case{Name: name, Language: language, Source: source, Kinds: kinds})
	}
	add("physical lines", "value\n \t\nvalue", "CBC")
	for i, marker := range rules.lineComments {
		add(fmt.Sprintf("line/%d", i), marker+" comment\nvalue\n", "MC")
	}
	for i, block := range rules.blockComments {
		endPrefix := "end "
		if block.lineStart {
			endPrefix = ""
		}
		add(fmt.Sprintf("block/%d", i), block.open+" comment\n\n"+endPrefix+block.close+"\nvalue\n", "MBMC")
		if !block.lineStart {
			add(fmt.Sprintf("mixed/%d", i), block.open+" comment "+block.close+" value\n", "C")
		}
		if block.nested {
			add(fmt.Sprintf("nested/%d", i), block.open+" outer\n"+block.open+" inner "+block.close+
				"\nstill outer\n"+block.close+" value\n", "MMMC")
		}
	}
	for i, quote := range rules.strings {
		add(fmt.Sprintf("quote/%d", i), "value = "+quote.open+"literal"+quote.close+"\nvalue\n", "CC")
		if quote.multiline {
			add(fmt.Sprintf("multiline/%d", i), "value = "+quote.open+"first\n\ntext\n"+quote.close+"\n", "CBCC")
			for j, marker := range rules.lineComments {
				add(fmt.Sprintf("quoted-line-state/%d/%d", i, j),
					"value = "+quote.open+"first\n"+marker+" literal\n"+quote.close+"\n"+marker+" real\n", "CCCM")
			}
			for j, block := range rules.blockComments {
				add(fmt.Sprintf("quoted-block-state/%d/%d", i, j),
					"value = "+quote.open+"first\n"+block.open+" literal\n"+quote.close+
						"\nvalue\n"+block.open+" real\n"+block.close+"\n", "CCCCMM")
			}
		}
		for j, block := range rules.blockComments {
			if rules.special == syntaxRust && quote.open == "'" {
				continue // A multi-character Rust literal is a lifetime, not a valid character.
			}
			add(fmt.Sprintf("quoted-block/%d/%d", i, j), "value = "+quote.open+block.open+quote.close+
				"\nvalue\n", "CC")
		}
	}
	for i, quote := range rules.docQuotes {
		add(fmt.Sprintf("documentation/%d", i), quote.open+"docs\n\nend"+quote.close+"\nvalue\n", "MBMC")
		add(fmt.Sprintf("assigned-doc/%d", i), "value = "+quote.open+"text\nend"+quote.close+"\n", "CC")
	}
	return cases
}

func ruleFeatures(rules *Rules) []string {
	features := []string{"physical-lines", "fragmentation", "state-isolation"}
	if len(rules.lineComments) > 0 {
		features = append(features, "line-comments")
	}
	if len(rules.blockComments) > 0 {
		features = append(features, "block-comments", "mixed-lines")
	}
	if len(rules.strings) > 0 {
		features = append(features, "quotes")
	}
	for _, block := range rules.blockComments {
		if block.nested {
			features = append(features, "nested-blocks")
			break
		}
	}
	for _, quote := range rules.strings {
		if quote.multiline {
			features = append(features, "multiline-quotes")
			break
		}
	}
	if len(rules.docQuotes) > 0 || rules.special == syntaxPython {
		features = append(features, "documentation")
	}
	if rules.raw != rawNone {
		features = append(features, "dynamic-raw")
	}
	if rules.special == syntaxJavaScript {
		features = append(features, "template-regex-heuristics")
	}
	if rules.fortran || rules.foldCase || rules.lineSplice {
		features = append(features, "local-context")
	}
	return features
}

func TestMatrixGoldenCoverage(t *testing.T) {
	counts := map[string]int{}
	for _, fixture := range lexertest.AllCases() {
		if strings.ContainsAny(fixture.Kinds, "CM") {
			counts[fixture.Language]++
		}
	}
	for _, name := range []string{"Go", "Java", "Rust", "Python", "JavaScript"} {
		if counts[name] < 10 {
			t.Errorf("%s needs at least 10 nonempty independently specified goldens, got %d", name, counts[name])
		}
	}
}
