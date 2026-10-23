# Language correctness matrix

View of the reviewed baseline in `internal/lexertest/matrix.go`; regenerate this document with:

```sh
go run ./scripts/diffcheck -matrix docs/language-test-matrix.md
```

Tier 1: deep independent goldens plus feature contracts. Tier 2: declared marker/quote feature contracts. Tier 3: frozen legacy behavior. Goldens below count shared Case entries; template tests are additional.

Every active language also runs full-reader/fragmented/debug/plain invariants; every legacy language is compared with the frozen legacy oracle. These checks establish the stated lexical contract, not a complete language grammar.

Uncovered contextual syntax remains explicit: embedded host languages, dialect-specific SQL, dynamic shell heredocs, full JavaScript grammar/regex disambiguation, Python parenthesized docstrings and replacement expressions. See [counting semantics](counting-semantics.md). A feature label promises the fixture contract, not arbitrary syntax.

| Language | Tier | Shared goldens | Tested features / legacy reason |
| --- | ---: | ---: | --- |
| ANTLR | 3 | 0 | No corresponding entry in the pinned snapshot |
| ATS | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| ActionScript | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Ada | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| Alda | 3 | 0 | No corresponding entry in the pinned snapshot |
| Ant | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| Arduino Sketch | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| AsciiDoc | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines |
| Assembly | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| AutoHotkey | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines |
| Awk | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| BASH | 1 | 12 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Ballerina | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Batch | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, local-context |
| Berry | 3 | 0 | No corresponding entry in the pinned snapshot |
| Bicep | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| BitBake | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Bourne Shell | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| C | 1 | 14 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, local-context |
| C Header | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, local-context |
| C Shell | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| C# | 1 | 17 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| C++ | 1 | 23 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, dynamic-raw, local-context |
| C++ Header | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, dynamic-raw, local-context |
| CMake | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| COBOL | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| CSS | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| CUDA | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Cairo | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Cap'n Proto | 3 | 0 | No corresponding entry in the pinned snapshot |
| Carbon | 3 | 0 | No corresponding entry in the pinned snapshot |
| Carp | 3 | 0 | No corresponding entry in the pinned snapshot |
| Chapel | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Circom | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines |
| Clojure | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| CoffeeScript | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| ColdFusion | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| ColdFusion CFScript | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Coq | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| Crystal | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Cython | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, documentation |
| D | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks |
| DTrace | 3 | 0 | No corresponding entry in the pinned snapshot |
| Dart | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Device Tree | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Dhall | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| Dockerfile | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Dune | 3 | 0 | No corresponding entry in the pinned snapshot |
| Eiffel | 3 | 0 | No corresponding entry in the pinned snapshot |
| Elixir | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Elm | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, nested-blocks |
| Erlang | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| Expect | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| F# | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| F* | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| FORTRAN Legacy | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes, local-context |
| FORTRAN Modern | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Factor | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines |
| Fish | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Frege | 3 | 0 | No corresponding entry in the pinned snapshot |
| GLSL | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Gherkin | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| Gleam | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Go | 1 | 19 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Groovy | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| HCL | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| HLSL | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| HTML | 3 | 1 | Embedded-language parsing is deferred |
| Handlebars | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| Hare | 3 | 0 | No corresponding entry in the pinned snapshot |
| Haskell | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, nested-blocks |
| Haxe | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Hurl | 3 | 0 | No corresponding entry in the pinned snapshot |
| Hy | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| Idris | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| Imba | 3 | 0 | No corresponding entry in the pinned snapshot |
| Inno Setup | 3 | 0 | No corresponding entry in the pinned snapshot |
| Io | 3 | 0 | No corresponding entry in the pinned snapshot |
| Isabelle | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| JAI | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| JSON | 2 | 0 | physical-lines, fragmentation, state-isolation |
| JSP | 3 | 0 | No corresponding entry in the pinned snapshot |
| JSX | 3 | 1 | Embedded JSX markup parsing is deferred |
| Janet | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Java | 1 | 22 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| JavaScript | 1 | 20 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes, template-regex-heuristics |
| Julia | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| Jupyter Notebook | 3 | 1 | Notebook cell-language parsing is deferred |
| Just | 3 | 0 | Preserve regex-based recipe shebang handling |
| KakouneScript | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Koka | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| Kotlin | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| LD Script | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| LESS | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| LISP | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, nested-blocks |
| Lean | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, nested-blocks |
| Lilypond | 3 | 0 | No corresponding entry in the pinned snapshot |
| LiveScript | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Logtalk | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Lua | 2 | 2 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| M4 | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| MATLAB | 3 | 0 | No corresponding entry in the pinned snapshot |
| MSBuild script | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| Makefile | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| Markdown | 3 | 1 | Literate prose and embedded code-block parsing are deferred |
| Maven | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| Mercury | 3 | 0 | No corresponding entry in the pinned snapshot |
| Meson | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Mojo | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, documentation |
| Motoko | 3 | 0 | No corresponding entry in the pinned snapshot |
| Move | 3 | 0 | No corresponding entry in the pinned snapshot |
| Mustache | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| NSIS | 3 | 0 | No corresponding entry in the pinned snapshot |
| Nearley | 3 | 0 | No corresponding entry in the pinned snapshot |
| Nim | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| Nix | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Nu | 3 | 0 | No corresponding entry in the pinned snapshot |
| Nunjucks | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines |
| OCaml | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| Objective-C | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Objective-C++ | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Odin | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Ohm | 3 | 0 | No corresponding entry in the pinned snapshot |
| PHP | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| PRQL | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Pascal | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| Perl | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Plain Text | 3 | 1 | Preserve nonblank text as code; literate prose classification is deferred |
| Plan9 Shell | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Polly | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| Pony | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes, documentation |
| PowerShell | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Protocol Buffers | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| Python | 1 | 25 | physical-lines, fragmentation, state-isolation, line-comments, quotes, documentation |
| Q | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| QML | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| R | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| RAML | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| RMarkdown | 3 | 0 | Literate prose and embedded code-block parsing are deferred |
| Racket | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, nested-blocks |
| ReStructuredText | 2 | 0 | physical-lines, fragmentation, state-isolation |
| Reason | 3 | 0 | No corresponding entry in the pinned snapshot |
| Rebol | 3 | 0 | No corresponding entry in the pinned snapshot |
| Red | 3 | 0 | No corresponding entry in the pinned snapshot |
| Rego | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Ring | 3 | 0 | No corresponding entry in the pinned snapshot |
| Ruby | 2 | 2 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Ruby HTML | 3 | 0 | Embedded-language parsing is deferred |
| Rust | 1 | 24 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes, dynamic-raw |
| SKILL | 3 | 0 | No corresponding entry in the pinned snapshot |
| SQL | 1 | 17 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Sass | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Scala | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Scheme | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, nested-blocks |
| Solidity | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Stan | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Standard ML | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| Starlark | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, quotes, documentation |
| Svelte | 3 | 1 | Embedded-language parsing is deferred |
| Swift | 2 | 2 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, nested-blocks, multiline-quotes |
| TLA | 3 | 0 | No corresponding entry in the pinned snapshot |
| TOML | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Tcl/Tk | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| TeX | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments |
| Templ | 3 | 0 | Embedded-language parsing is deferred |
| Terra | 3 | 0 | No corresponding entry in the pinned snapshot |
| TypeScript | 1 | 14 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes, template-regex-heuristics |
| Umka | 3 | 0 | No corresponding entry in the pinned snapshot |
| Unity-Prefab | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Vala | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| Verilog | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines, quotes, multiline-quotes |
| VimL | 3 | 0 | Double quotes require Vimscript expression/comment context |
| Visual Basic | 2 | 1 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes, local-context |
| Vue | 3 | 1 | Embedded-language parsing is deferred |
| Vyper | 3 | 0 | No corresponding entry in the pinned snapshot |
| WiX | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| XML | 2 | 2 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| XML resource | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| XSD | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| XSLT | 2 | 0 | physical-lines, fragmentation, state-isolation, block-comments, mixed-lines, quotes, multiline-quotes |
| YAML | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Yacc | 3 | 0 | No corresponding entry in the pinned snapshot |
| Yul | 3 | 0 | No corresponding entry in the pinned snapshot |
| Zephir | 3 | 0 | No corresponding entry in the pinned snapshot |
| Zig | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| Zsh | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, quotes, multiline-quotes |
| lex | 2 | 0 | physical-lines, fragmentation, state-isolation, line-comments, block-comments, mixed-lines |
| sed | 3 | 0 | No corresponding entry in the pinned snapshot |
