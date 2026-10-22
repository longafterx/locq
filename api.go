package gocloc

import (
	"io"

	"github.com/longafterx/gocloc/internal/core"
)

// Implementation details live in internal/core.
type (
	ClocFile            = core.ClocFile
	ClocFiles           = core.ClocFiles
	ClocLanguage        = core.ClocLanguage
	Language            = core.Language
	Languages           = core.Languages
	DefinedLanguages    = core.DefinedLanguages
	LanguageDefinition  = core.LanguageDefinition
	FileOptions         = core.FileOptions
	LineEvent           = core.LineEvent
	LineKind            = core.LineKind
	Options             = core.Options
	OptionError         = core.OptionError
	Result              = core.Result
	JSONLanguagesResult = core.JSONLanguagesResult
	JSONFilesResult     = core.JSONFilesResult
	XMLResultType       = core.XMLResultType
	XMLTotalLanguages   = core.XMLTotalLanguages
	XMLResultLanguages  = core.XMLResultLanguages
	XMLTotalFiles       = core.XMLTotalFiles
	XMLResultFiles      = core.XMLResultFiles
	XMLResult           = core.XMLResult
)

const (
	CodeLine           = core.CodeLine
	CommentLine        = core.CommentLine
	BlankLine          = core.BlankLine
	MaxWorkers         = core.MaxWorkers
	XMLResultWithLangs = core.XMLResultWithLangs
	XMLResultWithFiles = core.XMLResultWithFiles
)

func Analyze(paths []string, opts *Options) (*Result, error) {
	return core.Analyze(paths, opts)
}

func NewLanguageDefinition(name string, lineComments []string, blocks [][]string) *LanguageDefinition {
	return core.NewLanguageDefinition(name, lineComments, blocks)
}

func NewDefinedLanguages() *DefinedLanguages { return core.NewDefinedLanguages() }

func AnalyzeFile(filename string, language *LanguageDefinition, opts *FileOptions) (*ClocFile, error) {
	return core.AnalyzeFile(filename, language, opts)
}

func AnalyzeReader(filename string, language *LanguageDefinition, reader io.Reader, opts *FileOptions) (*ClocFile, error) {
	return core.AnalyzeReader(
		filename,
		language,
		reader,
		opts,
	)
}

func NewJSONLanguagesResultFromCloc(total *Language, languages Languages) JSONLanguagesResult {
	return core.NewJSONLanguagesResultFromCloc(total, languages)
}

func NewJSONFilesResultFromCloc(total *Language, files ClocFiles) JSONFilesResult {
	return core.NewJSONFilesResultFromCloc(total, files)
}

func NewXMLResultFromCloc(total *Language, languages Languages, kind XMLResultType) *XMLResult {
	return core.NewXMLResultFromCloc(total, languages, kind)
}

func InsertPipesInTheMiddle(input string) string {
	return core.InsertPipesInTheMiddle(input)
}
