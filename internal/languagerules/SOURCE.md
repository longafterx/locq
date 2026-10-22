# Tokei language rules

Upstream: https://github.com/XAMPPRocky/tokei/blob/master/languages.json

[upstream.json](upstream.json) records the pinned upstream commit, SHA-256 of
the exact `languages.json` bytes and expected number of language entries. The
snapshot is downloaded from that immutable commit, not from a moving raw URL.

The snapshot is used under its MIT license; see LICENCE-MIT in this directory.

The snapshot, upstream metadata and both generated artifacts use LF line endings,
enforced by .gitattributes even when core.autocrlf is enabled. The SHA-256 and generated-file
checks remain byte-for-byte; Windows checkout must not rewrite their contents.

## Generation and activation

Run `go generate ./...` to regenerate rules_generated.go and COVERAGE.md. The
generator reads the checked-in snapshot and mappings.json without network access.
It generates all upstream entries, including languages with no comment
delimiters. Strict JSON decoding rejects unknown fields, malformed delimiters and
conflicting opener meanings; prefix overlaps are resolved by longest matching.
Vimscript's ambiguous double quote is an explicitly deferred exception.

mappings.json lists all existing locq names explicitly. Some enable
the generated scanner; others retain the legacy scanner, either because the
snapshot has no corresponding language or because activation needs unsupported
context. See [COVERAGE.md](COVERAGE.md) for the exact mappings and local additions.
The generated catalog is larger than the set of languages enabled in the CLI.

This migration does **not** import extensions, filenames, shebangs, environment
names, path suffixes or MIME types. It does not add file types, rename results or
change file detection. Caller-created NewLanguage definitions still use their
own markers and regexes. The historical corrected C/Go scanner remains available
when custom definitions have exactly the built-in comment markers. Explicit
regex overrides on built-in definitions also retain legacy semantics.

HTML, Vue, Svelte, Ruby HTML, Templ, Markdown, JSX, notebooks and other embedding
or literate contexts are not activated by this change. Plain Text continues
counting nonblank text as code. Their data is generated, but script/style blocks,
code fences, notebook cells and prose classification are deliberately deferred.
The upstream blank flag is metadata, not an instruction to classify nonblank
lines as blank. Literate, kind and important_syntax metadata is recorded in the
coverage report, not silently treated as generic lexical syntax.

## Generic scanning semantics

- Line and block comments, ordinary quotes, verbatim quotes and doc quotes all
  participate in longest opening-delimiter matching. Equal-length block/line
  collisions prefer the block; other conflicting meanings fail generation.
- Ordinary quotes recognize backslash escapes. Verbatim quotes do not. C#/F#
  verbatim quotes and selected SQL/VB/Pascal-style quotes recognize doubled
  closers. Triple quotes and verbatim quotes can span physical lines.
- The snapshot does not describe newline recovery. Like tokei, generic ordinary
  quotes retain state across lines unless there is an explicit local recovery
  policy (such as C, Go, Java, JavaScript and Python). Malformed source is
  classified, not rejected or reparsed.
- Nested blocks recognize their matching closer and supported nested openers,
  including distinct nested block forms. Non-nested blocks close at the first
  matching closer. Comment markers inside quotes never start comments.
- Doc delimiters remain separate from ordinary quotes. Generic standalone doc
  strings count as documentation; assigned strings count as code. Raku POD and
  inline documentation forms are comments. This is not AST-level classification.
- Word-like comment markers such as REM and dnl respect identifier boundaries.
  Selected BASIC/batch rules are case-insensitive. Ruby/Perl blocks are column
  anchored; fixed-form Fortran comment letters are recognized only in column 1.

Physically blank lines count as blank even inside comments and strings. Any
nonblank line with code outside comments counts as code, including code mixed
with comments. Counts, callbacks, debug and fragmented reads share the same
per-file scanner. Rules are immutable and shared; scanning state is never shared
between concurrent files.

## Local supplements and limits

C/C Header add character literals and backslash-newline splicing. C++ variants
add dynamic raw delimiters (up to 16 characters), encoded prefixes, character
literals and numeric digit separators. Go adds runes and raw strings. Java adds
character literals and text blocks. Rust adds raw strings with 0-255 hashes,
byte/C raw prefixes and lifetime/label distinctions. Assembly retains locq's
combined dialect comment markers. Nim, Coq and F# add missing nested-block rules.

Python uses a lightweight statement/indentation model: the first standalone
string at module level or in a def/class suite is documentation. Preambles,
shebangs and blanks do not consume that position. Assigned triples, later
expressions, bytes and f-strings are code. The same strategy covers Python-like
Cython, Mojo, Bazel/Starlark and Snakemake declarations; language-specific
declaration extensions are not a full grammar. Parenthesized docstrings,
operations applied after a multiline docstring closes and f-string replacement
expressions are not fully parsed.

JavaScript/TypeScript retain local template interpolation and heuristic
regex/division handling. A generated delimiter table does not supply dynamic
heredocs, arbitrary language interpolation, macros or other grammar-dependent
features not listed here. Full catalog coverage is not a claim of complete
parsing or identical output to tokei.

## Automatic upstream updates

[Sync tokei language rules](../../.github/workflows/language-rules.yml) checks
upstream every Monday at 02:23 UTC (10:23 Asia/Shanghai). It can also be run from
GitHub Actions using `workflow_dispatch`, with an optional upstream branch, tag
or commit. The workflow always uses the repository's default branch as its base.

The updater finds the latest commit touching `languages.json` at the selected
ref, fetches the snapshot at that commit, computes its SHA-256 and entry count,
then strictly decodes and compiles every rule against the existing mappings.
Unknown fields, invalid delimiters, empty catalogs or removed mapped languages
fail before writing any local files. Network requests have timeouts and size
limits. Existing mappings, local supplements and legacy deferrals are not changed
automatically; new upstream entries do not add CLI language detection.

When there is a diff, the workflow tests the exact candidate files on Linux,
macOS and Windows with `go test -race -tags=integration ./...`. Only after all
three pass does it create or update a single PR on
`automation/tokei-language-rules`. No changes means no PR. Failures appear in
the Actions run; they do not update the default branch. PRs require review and
are not merged automatically.

The repository must allow GitHub Actions to create pull requests under Settings
Actions → General → Workflow permissions. The workflow uses `GITHUB_TOKEN`
with write access only in the PR job. PRs created with this token do not trigger
ordinary push/pull_request workflows; the preceding matrix validates candidates
inside the sync workflow. Branch protection requiring separate PR checks may
need a GitHub App token or a manual rerun of the required checks.

To update locally, run from the repository root:

```sh
(cd internal/languagerules && go run ./cmd/gen -update)
# Or select an upstream tag/commit explicitly:
(cd internal/languagerules && go run ./cmd/gen -update -ref v14.0.0)
go test -race -tags=integration ./...
```

`GITHUB_TOKEN` is optional for local updates and authenticates only GitHub API
requests. Regular `go generate ./...`, builds and tests remain offline with
respect to the language-rule source. Review the four changed snapshot/metadata/
generated files before committing. For incompatible upstream changes, adapt the
schema/compiler or mappings explicitly, then rerun the update and tests.

## Verification

internal/lexertest holds independent per-line expectations. Regression tests
cover normalized callback contents/order, debug, fragmented reads, malformed
sources, longest matching, raw quotes, nesting and documentation. Generation
tests verify checksum, complete name mappings, deterministic artifacts and
explicit deferrals. Fuzzing checks per-file state and physical-line conservation.

The benchmark script measures full analysis, workers 1/8 and both dedup modes
using versioned multilingual fixtures. See [scripts/BENCHMARKS.md](../../scripts/BENCHMARKS.md).
