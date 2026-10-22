package core

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/go-enry/go-enry/v2"
	"github.com/longafterx/locq/internal/lexer"
)

// ClocLanguage is provided for xml-cloc and json format.
type ClocLanguage struct {
	Name       string `xml:"name,attr" json:"name,omitempty"`
	FilesCount int32  `xml:"files_count,attr" json:"files"`
	Code       int32  `xml:"code,attr" json:"code"`
	Comments   int32  `xml:"comment,attr" json:"comment"`
	Blanks     int32  `xml:"blank,attr" json:"blank"`
}

// Language contains accumulated statistics, independently of parser definitions.
type Language struct {
	Name     string
	Files    []string
	Code     int32
	Comments int32
	Blanks   int32
	Total    int32
}

// LanguageDefinition describes immutable parsing rules. Constructors select the
// engine explicitly; a custom definition always uses literal legacy markers.
// Name is the name reported in results. Do not mutate definitions during a call.
type LanguageDefinition struct {
	Name              string
	syntax            *lexer.Rules
	lineComments      []string
	regexLineComments []*regexp.Regexp
	multiLines        [][]string
}

// Languages is an array representation of Language.
type Languages []Language

func (ls Languages) SortByName() {
	sortFunc := func(i, j int) bool {
		return ls[i].Name < ls[j].Name
	}
	sort.Slice(ls, sortFunc)
}

func (ls Languages) SortByFiles() {
	sortFunc := func(i, j int) bool {
		if len(ls[i].Files) == len(ls[j].Files) {
			return ls[i].Code > ls[j].Code
		}
		return len(ls[i].Files) > len(ls[j].Files)
	}
	sort.Slice(ls, sortFunc)
}

func (ls Languages) SortByComments() {
	sortFunc := func(i, j int) bool {
		if ls[i].Comments == ls[j].Comments {
			return ls[i].Code > ls[j].Code
		}
		return ls[i].Comments > ls[j].Comments
	}
	sort.Slice(ls, sortFunc)
}

func (ls Languages) SortByBlanks() {
	sortFunc := func(i, j int) bool {
		if ls[i].Blanks == ls[j].Blanks {
			return ls[i].Code > ls[j].Code
		}
		return ls[i].Blanks > ls[j].Blanks
	}
	sort.Slice(ls, sortFunc)
}

func (ls Languages) SortByCode() {
	sortFunc := func(i, j int) bool {
		return ls[i].Code > ls[j].Code
	}
	sort.Slice(ls, sortFunc)
}

var reShebangEnv = regexp.MustCompile(`^#! *(\S+/env) ([a-zA-Z]+)`)
var reShebangLang = regexp.MustCompile(`^#! *[.a-zA-Z/]+/([a-zA-Z]+)`)

// builtInExtensions maps detection keys to language names. Never modified.
var builtInExtensions = map[string]string{
	"as":          "ActionScript",
	"ada":         "Ada",
	"adb":         "Ada",
	"ads":         "Ada",
	"alda":        "Alda",
	"Ant":         "Ant",
	"adoc":        "AsciiDoc",
	"asciidoc":    "AsciiDoc",
	"asm":         "Assembly",
	"S":           "Assembly",
	"s":           "Assembly",
	"dats":        "ATS",
	"sats":        "ATS",
	"hats":        "ATS",
	"ahk":         "AutoHotkey",
	"awk":         "Awk",
	"bal":         "Ballerina",
	"bat":         "Batch",
	"btm":         "Batch",
	"bicep":       "Bicep",
	"bb":          "BitBake",
	"be":          "Berry",
	"cairo":       "Cairo",
	"carbon":      "Carbon",
	"cbl":         "COBOL",
	"cmd":         "Batch",
	"bash":        "BASH",
	"sh":          "Bourne Shell",
	"c":           "C",
	"carp":        "Carp",
	"csh":         "C Shell",
	"ec":          "C",
	"erl":         "Erlang",
	"hrl":         "Erlang",
	"pgc":         "C",
	"capnp":       "Cap'n Proto",
	"chpl":        "Chapel",
	"circom":      "Circom",
	"cs":          "C#",
	"clj":         "Clojure",
	"coffee":      "CoffeeScript",
	"cfm":         "ColdFusion",
	"cfc":         "ColdFusion CFScript",
	"cmake":       "CMake",
	"cc":          "C++",
	"cpp":         "C++",
	"cxx":         "C++",
	"pcc":         "C++",
	"c++":         "C++",
	"cr":          "Crystal",
	"css":         "CSS",
	"cu":          "CUDA",
	"d":           "D",
	"dart":        "Dart",
	"dhall":       "Dhall",
	"Dockerfile":  "Dockerfile",
	"dtrace":      "DTrace",
	"dts":         "Device Tree",
	"dtsi":        "Device Tree",
	"dune":        "Dune",
	"e":           "Eiffel",
	"elm":         "Elm",
	"el":          "LISP",
	"exp":         "Expect",
	"ex":          "Elixir",
	"exs":         "Elixir",
	"feature":     "Gherkin",
	"factor":      "Factor",
	"fish":        "Fish",
	"fr":          "Frege",
	"fst":         "F*",
	"F#":          "F#",   // deplicated F#/GLSL
	"GLSL":        "GLSL", // both use ext '.fs'
	"vs":          "GLSL",
	"shader":      "HLSL",
	"cg":          "HLSL",
	"cginc":       "HLSL",
	"hlsl":        "HLSL",
	"lean":        "Lean",
	"hlean":       "Lean",
	"lgt":         "Logtalk",
	"lisp":        "LISP",
	"lsp":         "LISP",
	"lua":         "Lua",
	"ls":          "LiveScript",
	"sc":          "LISP",
	"f":           "FORTRAN Legacy",
	"F":           "FORTRAN Legacy",
	"f77":         "FORTRAN Legacy",
	"for":         "FORTRAN Legacy",
	"ftn":         "FORTRAN Legacy",
	"pfo":         "FORTRAN Legacy",
	"f90":         "FORTRAN Modern",
	"F90":         "FORTRAN Modern",
	"f95":         "FORTRAN Modern",
	"f03":         "FORTRAN Modern",
	"f08":         "FORTRAN Modern",
	"gleam":       "Gleam",
	"g4":          "ANTLR",
	"go":          "Go",
	"go2":         "Go",
	"groovy":      "Groovy",
	"gradle":      "Groovy",
	"h":           "C Header",
	"hbs":         "Handlebars",
	"hs":          "Haskell",
	"hpp":         "C++ Header",
	"hh":          "C++ Header",
	"html":        "HTML",
	"ha":          "Hare",
	"hurl":        "Hurl",
	"hx":          "Haxe",
	"hxx":         "C++ Header",
	"hy":          "Hy",
	"idr":         "Idris",
	"imba":        "Imba",
	"il":          "SKILL",
	"ino":         "Arduino Sketch",
	"io":          "Io",
	"iss":         "Inno Setup",
	"ipynb":       "Jupyter Notebook",
	"jai":         "JAI",
	"java":        "Java",
	"jsp":         "JSP",
	"js":          "JavaScript",
	"jl":          "Julia",
	"janet":       "Janet",
	"json":        "JSON",
	"jsx":         "JSX",
	"just":        "Just",
	"kak":         "KakouneScript",
	"kk":          "Koka",
	"kt":          "Kotlin",
	"kts":         "Kotlin",
	"lds":         "LD Script",
	"less":        "LESS",
	"ly":          "Lilypond",
	"Objective-C": "Objective-C", // deplicated Obj-C/Matlab/Mercury
	"MATLAB":      "MATLAB",      // language name returned by go-enry for '.m'
	"Mercury":     "Mercury",     // use ext '.m'
	"md":          "Markdown",
	"markdown":    "Markdown",
	"mo":          "Motoko",
	"Motoko":      "Motoko",
	"ne":          "Nearley",
	"nix":         "Nix",
	"nsi":         "NSIS",
	"nsh":         "NSIS",
	"nu":          "Nu",
	"ML":          "OCaml",
	"ml":          "OCaml",
	"mli":         "OCaml",
	"mll":         "OCaml",
	"mly":         "OCaml",
	"mm":          "Objective-C++",
	"maven":       "Maven",
	"makefile":    "Makefile",
	"meson":       "Meson",
	"mustache":    "Mustache",
	"m4":          "M4",
	"mojo":        "Mojo",
	"🔥":           "Mojo",
	"move":        "Move",
	"l":           "lex",
	"nim":         "Nim",
	"njk":         "Nunjucks",
	"odin":        "Odin",
	"ohm":         "Ohm",
	"php":         "PHP",
	"pas":         "Pascal",
	"PL":          "Perl",
	"pl":          "Perl",
	"pm":          "Perl",
	"plan9sh":     "Plan9 Shell",
	"pony":        "Pony",
	"ps1":         "PowerShell",
	"text":        "Plain Text",
	"txt":         "Plain Text",
	"polly":       "Polly",
	"proto":       "Protocol Buffers",
	"prql":        "PRQL",
	"py":          "Python",
	"pxd":         "Cython",
	"pyx":         "Cython",
	"q":           "Q",
	"qml":         "QML",
	"r":           "R",
	"R":           "R",
	"raml":        "RAML",
	"re":          "Reason",
	"rei":         "Reason",
	"Rebol":       "Rebol",
	"red":         "Red",
	"rego":        "Rego",
	"Rmd":         "RMarkdown",
	"rake":        "Ruby",
	"rb":          "Ruby",
	"resx":        "XML resource", // ref: https://docs.microsoft.com/en-us/dotnet/framework/resources/creating-resource-files-for-desktop-apps#ResxFiles
	"ring":        "Ring",
	"rkt":         "Racket",
	"rhtml":       "Ruby HTML",
	"rs":          "Rust",
	"rst":         "ReStructuredText",
	"sass":        "Sass",
	"scala":       "Scala",
	"scss":        "Sass",
	"scm":         "Scheme",
	"sed":         "sed",
	"stan":        "Stan",
	"star":        "Starlark",
	"sml":         "Standard ML",
	"sol":         "Solidity",
	"sql":         "SQL",
	"svelte":      "Svelte",
	"swift":       "Swift",
	"t":           "Terra",
	"templ":       "Templ",
	"tex":         "TeX",
	"thy":         "Isabelle",
	"tla":         "TLA",
	"sty":         "TeX",
	"tcl":         "Tcl/Tk",
	"toml":        "TOML",
	"TypeScript":  "TypeScript",
	"tsx":         "TypeScript",
	"tf":          "HCL",
	"um":          "Umka",
	"mat":         "Unity-Prefab",
	"prefab":      "Unity-Prefab",
	"Coq":         "Coq",
	"vala":        "Vala",
	"Verilog":     "Verilog",
	"csproj":      "MSBuild script",
	"vbproj":      "MSBuild script",
	"vcproj":      "MSBuild script",
	"vb":          "Visual Basic",
	"vim":         "VimL",
	"vue":         "Vue",
	"vy":          "Vyper",
	"xml":         "XML",
	"XML":         "XML",
	"xsd":         "XSD",
	"xsl":         "XSLT",
	"xslt":        "XSLT",
	"wxs":         "WiX",
	"yaml":        "YAML",
	"yml":         "YAML",
	"y":           "Yacc",
	"yul":         "Yul",
	"zep":         "Zephir",
	"zig":         "Zig",
	"zsh":         "Zsh",
}

var shebang2ext = map[string]string{
	"gosh":    "scm",
	"make":    "make",
	"perl":    "pl",
	"rc":      "plan9sh",
	"python":  "py",
	"ruby":    "rb",
	"escript": "erl",
}

func getShebang(line string) (shebangLang string, ok bool) {
	ret := reShebangEnv.FindAllStringSubmatch(line, -1)
	if ret != nil && len(ret[0]) == 3 {
		shebangLang = ret[0][2]
		if sl, ok := shebang2ext[shebangLang]; ok {
			return sl, ok
		}
		return shebangLang, true
	}

	ret = reShebangLang.FindAllStringSubmatch(line, -1)
	if ret != nil && len(ret[0]) >= 2 {
		shebangLang = ret[0][1]
		if sl, ok := shebang2ext[shebangLang]; ok {
			return sl, ok
		}
		return shebangLang, true
	}

	return "", false
}

// detectFileType preserves filename and shebang precedence while allowing callers
// to reuse the bytes consumed for language detection.
func detectFileType(path string, opts *runState, readContent func(bool) ([]byte, error)) (ext string, ok bool) {
	ext = filepath.Ext(path)
	base := filepath.Base(path)

	switch ext {
	case ".m", ".v", ".fs", ".r", ".ts":
		content, err := readContent(true)
		if err != nil {
			return "", false
		}
		lang := enry.GetLanguage(path, content)
		if opts.Debug {
			opts.diagnosticf("[LANG] file=%q language=%q\n", path, lang)
		}
		return lang, true
	case ".mo":
		content, err := readContent(true)
		if err != nil {
			return "", false
		}
		lang := enry.GetLanguage(path, content)
		if opts.Debug {
			opts.diagnosticf("[LANG] file=%q language=%q\n", path, lang)
		}
		if lang != "" {
			return "Motoko", true
		}
		return lang, true
	}

	switch base {
	case "meson.build", "meson_options.txt":
		return "meson", true
	case "CMakeLists.txt":
		return "cmake", true
	case "configure.ac":
		return "m4", true
	case "Makefile.am":
		return "makefile", true
	case "build.xml":
		return "Ant", true
	case "pom.xml":
		return "maven", true
	}

	switch strings.ToLower(base) {
	case "justfile":
		return "just", true
	case "makefile":
		return "makefile", true
	case "nukefile":
		return "nu", true
	case "dune":
		return "dune", true
	case "rebar": // skip
		return "", false
	case "dockerfile":
		return "Dockerfile", true
	}

	line, err := readContent(false)
	// Like the original detector, an unterminated first line is not a shebang.
	if err == nil {
		line = bytes.TrimLeftFunc(line, unicode.IsSpace)
		if len(line) > 2 && line[0] == '#' && line[1] == '!' {
			if lang, found := getShebang(string(line)); found {
				return lang, true
			}
		}
	}

	if len(ext) >= 2 {
		return ext[1:], true
	}
	return ext, ok
}

// NewLanguageDefinition creates explicit legacy rules, independent of the name.
// Markers are literal; block comments must have nonempty opening/closing markers.
// Empty marker pairs are accepted as the historical no-block-comments sentinel.
// Inputs are copied. Invalid block pairs are reported by analysis entry points.
func NewLanguageDefinition(name string, lineComments []string, multiLines [][]string) *LanguageDefinition {
	definition := &LanguageDefinition{Name: name, lineComments: append([]string(nil), lineComments...)}
	for _, pair := range multiLines {
		definition.multiLines = append(definition.multiLines, append([]string(nil), pair...))
	}
	return definition
}

func (l *LanguageDefinition) withRegexLineComments(patterns []string) *LanguageDefinition {
	for _, pattern := range patterns {
		l.regexLineComments = append(l.regexLineComments, regexp.MustCompile(pattern))
	}
	l.syntax = nil
	return l
}

func lang2exts(lang string, extensions map[string]string) (exts string) {
	var es []string
	for ext, l := range extensions {
		if lang == l {
			switch lang {
			case "Objective-C", "MATLAB", "Mercury":
				ext = "m"
			case "F#":
				ext = "fs"
			case "GLSL":
				if ext == "GLSL" {
					ext = "fs"
				}
			case "TypeScript":
				ext = "ts"
			case "Motoko":
				ext = "mo"
			}
			es = append(es, ext)
		}
	}
	return strings.Join(es, ", ")
}

// DefinedLanguages contains built-in parser definitions keyed by language name.
type DefinedLanguages struct {
	Langs map[string]*LanguageDefinition
}

// GetFormattedString return DefinedLanguages as a human-readable string.
func (langs *DefinedLanguages) GetFormattedString() string {
	var buf bytes.Buffer
	extensions := builtInExtensions
	var printLangs []string
	for _, lang := range langs.Langs {
		printLangs = append(printLangs, lang.Name)
	}
	sort.Strings(printLangs)
	for _, lang := range printLangs {
		buf.WriteString(fmt.Sprintf("%-30v (%s)\n", lang, lang2exts(lang, extensions)))
	}
	return buf.String()
}

// Built-in marker slices are fresh literals owned by this catalog, so copying
// them again through the public custom-definition constructor adds no isolation.
func newBuiltInDefinition(name string, lines []string, blocks [][]string) *LanguageDefinition {
	return &LanguageDefinition{Name: name, lineComments: lines, multiLines: blocks}
}

// NewDefinedLanguages creates an independent built-in catalog.
// Keep the literal catalog entries compact so their marker sets can be scanned
// together; longer rows deliberately retain the established table layout.
func NewDefinedLanguages() *DefinedLanguages {
	definitions := &DefinedLanguages{
		Langs: map[string]*LanguageDefinition{
			"ActionScript":        newBuiltInDefinition("ActionScript", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Ada":                 newBuiltInDefinition("Ada", []string{"--"}, [][]string{{"", ""}}),
			"Alda":                newBuiltInDefinition("Alda", []string{"#"}, [][]string{{"", ""}}),
			"Ant":                 newBuiltInDefinition("Ant", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"ANTLR":               newBuiltInDefinition("ANTLR", []string{"//"}, [][]string{{"/*", "*/"}}),
			"AsciiDoc":            newBuiltInDefinition("AsciiDoc", []string{}, [][]string{{"", ""}}),
			"Assembly":            newBuiltInDefinition("Assembly", []string{"//", ";", "#", "@", "|", "!"}, [][]string{{"/*", "*/"}}),
			"ATS":                 newBuiltInDefinition("ATS", []string{"//"}, [][]string{{"/*", "*/"}, {"(*", "*)"}}),
			"AutoHotkey":          newBuiltInDefinition("AutoHotkey", []string{";"}, [][]string{{"", ""}}),
			"Awk":                 newBuiltInDefinition("Awk", []string{"#"}, [][]string{{"", ""}}),
			"Arduino Sketch":      newBuiltInDefinition("Arduino Sketch", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Ballerina":           newBuiltInDefinition("Ballerina", []string{"//"}, [][]string{{"", ""}}),
			"Batch":               newBuiltInDefinition("Batch", []string{"REM", "rem"}, [][]string{{"", ""}}),
			"Berry":               newBuiltInDefinition("Berry", []string{"#"}, [][]string{{"#-", "-#"}}),
			"BASH":                newBuiltInDefinition("BASH", []string{"#"}, [][]string{{"", ""}}),
			"Bicep":               newBuiltInDefinition("Bicep", []string{"//"}, [][]string{{"/*", "*/"}}),
			"BitBake":             newBuiltInDefinition("BitBake", []string{"#"}, [][]string{{"", ""}}),
			"C":                   newBuiltInDefinition("C", []string{"//"}, [][]string{{"/*", "*/"}}),
			"C Header":            newBuiltInDefinition("C Header", []string{"//"}, [][]string{{"/*", "*/"}}),
			"C Shell":             newBuiltInDefinition("C Shell", []string{"#"}, [][]string{{"", ""}}),
			"Cairo":               newBuiltInDefinition("Cairo", []string{"//"}, [][]string{{"", ""}}),
			"Carbon":              newBuiltInDefinition("Carbon", []string{"//"}, [][]string{{"", ""}}),
			"Cap'n Proto":         newBuiltInDefinition("Cap'n Proto", []string{"#"}, [][]string{{"", ""}}),
			"Carp":                newBuiltInDefinition("Carp", []string{";"}, [][]string{{"", ""}}),
			"C#":                  newBuiltInDefinition("C#", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Chapel":              newBuiltInDefinition("Chapel", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Circom":              newBuiltInDefinition("Circom", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Clojure":             newBuiltInDefinition("Clojure", []string{"#", "#_"}, [][]string{{"", ""}}),
			"COBOL":               newBuiltInDefinition("COBOL", []string{"*", "/"}, [][]string{{"", ""}}),
			"CoffeeScript":        newBuiltInDefinition("CoffeeScript", []string{"#"}, [][]string{{"###", "###"}}),
			"Coq":                 newBuiltInDefinition("Coq", []string{"(*"}, [][]string{{"(*", "*)"}}),
			"ColdFusion":          newBuiltInDefinition("ColdFusion", []string{}, [][]string{{"<!---", "--->"}}),
			"ColdFusion CFScript": newBuiltInDefinition("ColdFusion CFScript", []string{"//"}, [][]string{{"/*", "*/"}}),
			"CMake":               newBuiltInDefinition("CMake", []string{"#"}, [][]string{{"", ""}}),
			"C++":                 newBuiltInDefinition("C++", []string{"//"}, [][]string{{"/*", "*/"}}),
			"C++ Header":          newBuiltInDefinition("C++ Header", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Crystal":             newBuiltInDefinition("Crystal", []string{"#"}, [][]string{{"", ""}}),
			"CSS":                 newBuiltInDefinition("CSS", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Cython":              newBuiltInDefinition("Cython", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}}),
			"CUDA":                newBuiltInDefinition("CUDA", []string{"//"}, [][]string{{"/*", "*/"}}),
			"D":                   newBuiltInDefinition("D", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Dart":                newBuiltInDefinition("Dart", []string{"//", "///"}, [][]string{{"/*", "*/"}}),
			"Dhall":               newBuiltInDefinition("Dhall", []string{"--"}, [][]string{{"{-", "-}"}}),
			"DTrace":              newBuiltInDefinition("DTrace", []string{}, [][]string{{"/*", "*/"}}),
			"Device Tree":         newBuiltInDefinition("Device Tree", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Dockerfile":          newBuiltInDefinition("Dockerfile", []string{"#"}, [][]string{{"", ""}}),
			"Dune":                newBuiltInDefinition("Dune", []string{";"}, [][]string{{"", ""}}),
			"Eiffel":              newBuiltInDefinition("Eiffel", []string{"--"}, [][]string{{"", ""}}),
			"Elm":                 newBuiltInDefinition("Elm", []string{"--"}, [][]string{{"{-", "-}"}}),
			"Elixir":              newBuiltInDefinition("Elixir", []string{"#"}, [][]string{{"", ""}}),
			"Erlang":              newBuiltInDefinition("Erlang", []string{"%"}, [][]string{{"", ""}}),
			"Expect":              newBuiltInDefinition("Expect", []string{"#"}, [][]string{{"", ""}}),
			"Fish":                newBuiltInDefinition("Fish", []string{"#"}, [][]string{{"", ""}}),
			"Frege":               newBuiltInDefinition("Frege", []string{"--"}, [][]string{{"{-", "-}"}}),
			"F*":                  newBuiltInDefinition("F*", []string{"(*", "//"}, [][]string{{"(*", "*)"}}),
			"F#":                  newBuiltInDefinition("F#", []string{"(*"}, [][]string{{"(*", "*)"}}),
			"Lean":                newBuiltInDefinition("Lean", []string{"--"}, [][]string{{"/-", "-/"}}),
			"Logtalk":             newBuiltInDefinition("Logtalk", []string{"%"}, [][]string{{"", ""}}),
			"Lua":                 newBuiltInDefinition("Lua", []string{"--"}, [][]string{{"--[[", "]]"}}),
			"Lilypond":            newBuiltInDefinition("Lilypond", []string{"%"}, [][]string{{"", ""}}),
			"LISP":                newBuiltInDefinition("LISP", []string{";;"}, [][]string{{"#|", "|#"}}),
			"LiveScript":          newBuiltInDefinition("LiveScript", []string{"#"}, [][]string{{"/*", "*/"}}),
			"Factor":              newBuiltInDefinition("Factor", []string{"! "}, [][]string{{"", ""}}),
			"FORTRAN Legacy":      newBuiltInDefinition("FORTRAN Legacy", []string{"c", "C", "!", "*"}, [][]string{{"", ""}}),
			"FORTRAN Modern":      newBuiltInDefinition("FORTRAN Modern", []string{"!"}, [][]string{{"", ""}}),
			"Gherkin":             newBuiltInDefinition("Gherkin", []string{"#"}, [][]string{{"", ""}}),
			"Gleam":               newBuiltInDefinition("Gleam", []string{"//"}, [][]string{{"", ""}}),
			"GLSL":                newBuiltInDefinition("GLSL", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Go":                  newBuiltInDefinition("Go", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Groovy":              newBuiltInDefinition("Groovy", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Handlebars":          newBuiltInDefinition("Handlebars", []string{}, [][]string{{"<!--", "-->"}, {"{{!", "}}"}}),
			"Haskell":             newBuiltInDefinition("Haskell", []string{"--"}, [][]string{{"{-", "-}"}}),
			"Haxe":                newBuiltInDefinition("Haxe", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Hurl":                newBuiltInDefinition("Hurl", []string{"#"}, [][]string{{"", ""}}),
			"Hare":                newBuiltInDefinition("Hare", []string{"//"}, [][]string{{"", ""}}),
			"HLSL":                newBuiltInDefinition("HLSL", []string{"//"}, [][]string{{"/*", "*/"}}),
			"HTML":                newBuiltInDefinition("HTML", []string{"//", "<!--"}, [][]string{{"<!--", "-->"}}),
			"Hy":                  newBuiltInDefinition("Hy", []string{";"}, [][]string{{"", ""}}),
			"Idris":               newBuiltInDefinition("Idris", []string{"--"}, [][]string{{"{-", "-}"}}),
			"Imba":                newBuiltInDefinition("Imba", []string{"#"}, [][]string{{"###", "###"}}),
			"Io":                  newBuiltInDefinition("Io", []string{"//", "#"}, [][]string{{"/*", "*/"}}),
			"SKILL":               newBuiltInDefinition("SKILL", []string{";"}, [][]string{{"/*", "*/"}}),
			"JAI":                 newBuiltInDefinition("JAI", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Janet":               newBuiltInDefinition("Janet", []string{"#"}, [][]string{{"", ""}}),
			"Java":                newBuiltInDefinition("Java", []string{"//"}, [][]string{{"/*", "*/"}}),
			"JSP":                 newBuiltInDefinition("JSP", []string{"//"}, [][]string{{"/*", "*/"}}),
			"JavaScript":          newBuiltInDefinition("JavaScript", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Julia":               newBuiltInDefinition("Julia", []string{"#"}, [][]string{{"#:=", ":=#"}}),
			"Jupyter Notebook":    newBuiltInDefinition("Jupyter Notebook", []string{"#"}, [][]string{{"", ""}}),
			"Just":                newBuiltInDefinition("Just", []string{"#"}, [][]string{{"", ""}}).withRegexLineComments([]string{`^#[^!].*`}),
			"JSON":                newBuiltInDefinition("JSON", []string{}, [][]string{{"", ""}}),
			"JSX":                 newBuiltInDefinition("JSX", []string{"//"}, [][]string{{"/*", "*/"}}),
			"KakouneScript":       newBuiltInDefinition("KakouneScript", []string{"#"}, [][]string{{"", ""}}),
			"Koka":                newBuiltInDefinition("Koka", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Kotlin":              newBuiltInDefinition("Kotlin", []string{"//"}, [][]string{{"/*", "*/"}}),
			"LD Script":           newBuiltInDefinition("LD Script", []string{"//"}, [][]string{{"/*", "*/"}}),
			"LESS":                newBuiltInDefinition("LESS", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Objective-C":         newBuiltInDefinition("Objective-C", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Markdown":            newBuiltInDefinition("Markdown", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"Motoko":              newBuiltInDefinition("Motoko", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Nearley":             newBuiltInDefinition("Nearley", []string{"#"}, [][]string{{"", ""}}),
			"Nix":                 newBuiltInDefinition("Nix", []string{"#"}, [][]string{{"/*", "*/"}}),
			"NSIS":                newBuiltInDefinition("NSIS", []string{"#", ";"}, [][]string{{"/*", "*/"}}),
			"Nu":                  newBuiltInDefinition("Nu", []string{";", "#"}, [][]string{{"", ""}}),
			"OCaml":               newBuiltInDefinition("OCaml", []string{}, [][]string{{"(*", "*)"}}),
			"Objective-C++":       newBuiltInDefinition("Objective-C++", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Makefile":            newBuiltInDefinition("Makefile", []string{"#"}, [][]string{{"", ""}}),
			"MATLAB":              newBuiltInDefinition("MATLAB", []string{"%"}, [][]string{{"%{", "}%"}}),
			"Mercury":             newBuiltInDefinition("Mercury", []string{"%"}, [][]string{{"/*", "*/"}}),
			"Maven":               newBuiltInDefinition("Maven", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"Meson":               newBuiltInDefinition("Meson", []string{"#"}, [][]string{{"", ""}}),
			"Mojo":                newBuiltInDefinition("Mojo", []string{"#"}, [][]string{{"", ""}}),
			"Move":                newBuiltInDefinition("Move", []string{"//"}, [][]string{{"", ""}}),
			"Mustache":            newBuiltInDefinition("Mustache", []string{}, [][]string{{"{{!", "}}"}}),
			"M4":                  newBuiltInDefinition("M4", []string{"#"}, [][]string{{"", ""}}),
			"Nim":                 newBuiltInDefinition("Nim", []string{"#"}, [][]string{{"#[", "]#"}}),
			"Nunjucks":            newBuiltInDefinition("Nunjucks", []string{}, [][]string{{"{#", "#}"}, {"<!--", "-->"}}),
			"lex":                 newBuiltInDefinition("lex", []string{}, [][]string{{"/*", "*/"}}),
			"Odin":                newBuiltInDefinition("Odin", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Ohm":                 newBuiltInDefinition("Ohm", []string{"//"}, [][]string{{"/*", "*/"}}),
			"PHP":                 newBuiltInDefinition("PHP", []string{"#", "//"}, [][]string{{"/*", "*/"}}),
			"Pascal":              newBuiltInDefinition("Pascal", []string{"//"}, [][]string{{"{", ")"}}),
			"Perl":                newBuiltInDefinition("Perl", []string{"#"}, [][]string{{":=", ":=cut"}}),
			"Plain Text":          newBuiltInDefinition("Plain Text", []string{}, [][]string{{"", ""}}),
			"Plan9 Shell":         newBuiltInDefinition("Plan9 Shell", []string{"#"}, [][]string{{"", ""}}),
			"Pony":                newBuiltInDefinition("Pony", []string{"//"}, [][]string{{"/*", "*/"}}),
			"PowerShell":          newBuiltInDefinition("PowerShell", []string{"#"}, [][]string{{"<#", "#>"}}),
			"Polly":               newBuiltInDefinition("Polly", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"Protocol Buffers":    newBuiltInDefinition("Protocol Buffers", []string{"//"}, [][]string{{"", ""}}),
			"PRQL":                newBuiltInDefinition("PRQL", []string{"#"}, [][]string{{"", ""}}),
			"Python":              newBuiltInDefinition("Python", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}}),
			"Q":                   newBuiltInDefinition("Q", []string{"/ "}, [][]string{{"\\", "/"}, {"/", "\\"}}),
			"QML":                 newBuiltInDefinition("QML", []string{"//"}, [][]string{{"/*", "*/"}}),
			"R":                   newBuiltInDefinition("R", []string{"#"}, [][]string{{"", ""}}),
			"Reason":              newBuiltInDefinition("Reason", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Rebol":               newBuiltInDefinition("Rebol", []string{";"}, [][]string{{"", ""}}),
			"Red":                 newBuiltInDefinition("Red", []string{";"}, [][]string{{"", ""}}),
			"Rego":                newBuiltInDefinition("Rego", []string{"#"}, [][]string{{"", ""}}),
			"RMarkdown":           newBuiltInDefinition("RMarkdown", []string{}, [][]string{{"", ""}}),
			"RAML":                newBuiltInDefinition("RAML", []string{"#"}, [][]string{{"", ""}}),
			"Racket":              newBuiltInDefinition("Racket", []string{";"}, [][]string{{"#|", "|#"}}),
			"ReStructuredText":    newBuiltInDefinition("ReStructuredText", []string{}, [][]string{{"", ""}}),
			"Ring":                newBuiltInDefinition("Ring", []string{"#", "//"}, [][]string{{"/*", "*/"}}),
			"Ruby":                newBuiltInDefinition("Ruby", []string{"#"}, [][]string{{":=begin", ":=end"}}),
			"Ruby HTML":           newBuiltInDefinition("Ruby HTML", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"Rust":                newBuiltInDefinition("Rust", []string{"//", "///", "//!"}, [][]string{{"/*", "*/"}}),
			"Scala":               newBuiltInDefinition("Scala", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Sass":                newBuiltInDefinition("Sass", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Scheme":              newBuiltInDefinition("Scheme", []string{";"}, [][]string{{"#|", "|#"}}),
			"sed":                 newBuiltInDefinition("sed", []string{"#"}, [][]string{{"", ""}}),
			"Stan":                newBuiltInDefinition("Stan", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Starlark":            newBuiltInDefinition("Starlark", []string{"#"}, [][]string{{"", ""}}),
			"Solidity":            newBuiltInDefinition("Solidity", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Bourne Shell":        newBuiltInDefinition("Bourne Shell", []string{"#"}, [][]string{{"", ""}}),
			"Standard ML":         newBuiltInDefinition("Standard ML", []string{}, [][]string{{"(*", "*)"}}),
			"SQL":                 newBuiltInDefinition("SQL", []string{"--"}, [][]string{{"/*", "*/"}}),
			"Svelte":              newBuiltInDefinition("Svelte", []string{"//"}, [][]string{{"/*", "*/"}, {"<!--", "-->"}}),
			"Swift":               newBuiltInDefinition("Swift", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Templ":               newBuiltInDefinition("Templ", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Terra":               newBuiltInDefinition("Terra", []string{"--"}, [][]string{{"--[[", "]]"}}),
			"TeX":                 newBuiltInDefinition("TeX", []string{"%"}, [][]string{{"", ""}}),
			"Inno Setup":          newBuiltInDefinition("Inno Setup", []string{";"}, [][]string{{"", ""}}),
			"Isabelle":            newBuiltInDefinition("Isabelle", []string{}, [][]string{{"(*", "*)"}}),
			"TLA":                 newBuiltInDefinition("TLA", []string{"\\*"}, [][]string{{"(*", "*)"}}),
			"Tcl/Tk":              newBuiltInDefinition("Tcl/Tk", []string{"#"}, [][]string{{"", ""}}),
			"TOML":                newBuiltInDefinition("TOML", []string{"#"}, [][]string{{"", ""}}),
			"TypeScript":          newBuiltInDefinition("TypeScript", []string{"//"}, [][]string{{"/*", "*/"}}),
			"HCL":                 newBuiltInDefinition("HCL", []string{"#", "//"}, [][]string{{"/*", "*/"}}),
			"Umka":                newBuiltInDefinition("Umka", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Unity-Prefab":        newBuiltInDefinition("Unity-Prefab", []string{}, [][]string{{"", ""}}),
			"MSBuild script":      newBuiltInDefinition("MSBuild script", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"Vala":                newBuiltInDefinition("Vala", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Verilog":             newBuiltInDefinition("Verilog", []string{"//"}, [][]string{{"/*", "*/"}}),
			"VimL":                newBuiltInDefinition("VimL", []string{`"`}, [][]string{{"", ""}}),
			"Visual Basic":        newBuiltInDefinition("Visual Basic", []string{"'"}, [][]string{{"", ""}}),
			"Vue":                 newBuiltInDefinition("Vue", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"Vyper":               newBuiltInDefinition("Vyper", []string{"#"}, [][]string{{"\"\"\"", "\"\"\""}}),
			"WiX":                 newBuiltInDefinition("WiX", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"XML":                 newBuiltInDefinition("XML", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"XML resource":        newBuiltInDefinition("XML resource", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"XSLT":                newBuiltInDefinition("XSLT", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"XSD":                 newBuiltInDefinition("XSD", []string{"<!--"}, [][]string{{"<!--", "-->"}}),
			"YAML":                newBuiltInDefinition("YAML", []string{"#"}, [][]string{{"", ""}}),
			"Yacc":                newBuiltInDefinition("Yacc", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Yul":                 newBuiltInDefinition("Yul", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Zephir":              newBuiltInDefinition("Zephir", []string{"//"}, [][]string{{"/*", "*/"}}),
			"Zig":                 newBuiltInDefinition("Zig", []string{"//", "///"}, [][]string{{"", ""}}),
			"Zsh":                 newBuiltInDefinition("Zsh", []string{"#"}, [][]string{{"", ""}}),
		},
	}
	for name, language := range definitions.Langs {
		language.syntax = lexer.BuiltIn(name)
	}
	return definitions
}
