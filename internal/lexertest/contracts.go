package lexertest

// SemanticsVersion changes when the counting contract intentionally changes.
const SemanticsVersion = "locq-counting-v1"

// ContractCases contains explicit expectations, never scanner-derived goldens.
// The short, sometimes incomplete sources exercise lexical counting, not parsing.
func ContractCases() []Case {
	cases := []Case{}
	for _, language := range []struct {
		name, extension, line string
		block, recover        bool
	}{
		{name: "Go", extension: "go", line: "//", block: true, recover: true},
		{name: "Java", extension: "java", line: "//", block: true, recover: true},
		{name: "Rust", extension: "rs", line: "//", block: true},
		{name: "JavaScript", extension: "js", line: "//", block: true, recover: true},
		{name: "TypeScript", extension: "ts", line: "//", block: true, recover: true},
		{name: "C", extension: "c", line: "//", block: true, recover: true},
		{name: "C++", extension: "cpp", line: "//", block: true, recover: true},
		{name: "C#", extension: "cs", line: "//", block: true, recover: true},
		{name: "Python", extension: "py", line: "#", recover: true},
		{name: "BASH", extension: "bash", line: "#"},
		{name: "SQL", extension: "sql", line: "--", block: true},
	} {
		add := func(name, source, kinds string) {
			cases = append(cases, Case{
				Name:     "contract/" + language.name + "/" + name,
				Language: language.name, Extension: language.extension, Source: source, Kinds: kinds,
			})
		}
		add("empty", "", "")
		add("blank", " \t\n\n", "BB")
		add("code", "value\n", "C")
		add("line comment", language.line+" real\nvalue\n", "MC")
		add("trailing comment", "value "+language.line+" real\n", "C")
		quote := "\""
		if language.name == "SQL" {
			quote = "'"
		}
		add("quoted marker", "value = "+quote+language.line+" /*"+quote+"\nvalue\n", "CC")
		add("no final newline", language.line+" real\nvalue", "MC")
		add("CRLF BOM", "\xef\xbb\xbf"+language.line+" real\r\nvalue\r\n \t\r\n", "MCB")
		add("invalid UTF8", "value = \"\xff\"\n"+language.line+" real\n", "CM")
		quoteKinds := "CC"
		if language.recover {
			quoteKinds = "CM"
		}
		add("incomplete quote", "value = "+quote+"unfinished\n"+language.line+" real", quoteKinds)
		if language.block {
			add("code after block", "/* first\nlast */ value\n\nvalue "+language.line+" tail\n", "MCBC")
			add("quote inside comment", "/* \" ' //\nend */\nvalue\n", "MMC")
			add("blank inside block", "/*\n \t\n*/\n", "MBM")
			add("incomplete block", "/* incomplete\nvalue", "MM")
		}
	}
	return append(cases, []Case{
		{Name: "contract/JavaScript/division", Language: "JavaScript", Extension: "js",
			Source: "const n = 10 / 2;\n// real\n", Kinds: "CM"},
		{Name: "contract/JavaScript/regex", Language: "JavaScript", Extension: "js",
			Source: "const r = /[/*]/;\n// real\n", Kinds: "CM"},
		{Name: "contract/JavaScript/nested template", Language: "JavaScript", Extension: "js",
			Source: "const x = `a ${`b ${1}`} c`;\n// real\n", Kinds: "CM"},
		{Name: "contract/JavaScript/incomplete template", Language: "JavaScript", Extension: "js",
			Source: "const x = `first\n// literal", Kinds: "CC"},
		{Name: "contract/Go/non-nested blocks", Language: "Go", Extension: "go",
			Source: "/* outer /* inner */ value\n// real\n", Kinds: "CM"},
		{Name: "contract/Go/rune", Language: "Go", Extension: "go",
			Source: "var r = '\\'' /* comment\nend */\n", Kinds: "CM"},
		{Name: "contract/Go/raw blank", Language: "Go", Extension: "go",
			Source: "var s = `first\n\n// literal\n`\n", Kinds: "CBCC"},
		{Name: "contract/Java/character", Language: "Java", Extension: "java",
			Source: "char c = '\"'; /* real\nend */\n", Kinds: "CM"},
		{Name: "contract/SQL/dialect boundary", Language: "SQL", Extension: "sql",
			Source: "select '--';\n-- real\n", Kinds: "CM"},
	}...)
}

// AllCases is the shared corpus for correctness tests and differential export.
// BenchmarkCases remains unchanged so benchmark workload fingerprints stay stable.
func AllCases() []Case {
	cases := append(Cases(), ContractCases()...)
	cases = append(cases, LegacyCases()...)
	return append(cases, BenchmarkCases()...)
}
