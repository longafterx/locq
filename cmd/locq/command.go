package main

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/longafterx/locq"
	"github.com/longafterx/locq/internal/core"
	"github.com/spf13/cobra"
)

// CmdOptions is locq command options.
type CmdOptions struct {
	ByFile      bool
	SortTag     string
	OutputType  string
	ExcludeLang string
	Include     []string
	Exclude     []string
	IncludeLang string
	Debug       bool
	Workers     int
	Dedup       bool
	ShowLang    bool
	ShowVersion bool
}

func newRootCommand() *cobra.Command {
	var opts CmdOptions
	command := &cobra.Command{
		Use:   "locq [OPTIONS] PATH[...]",
		Short: "A fast, parallel source code line counter",
		Example: `  # Count the current directory
  locq .

  # Count multiple directories
  locq src tests

  # Exclude dependency and build directories by name
  locq --exclude '{dist,node_modules,target}/' .

  # Include Go files and exclude tests with globs
  locq --include '*.go' --exclude '*_test.go' .

  # Include a directory tree and exclude generated directories
  locq --include 'src/' --exclude '{dist,node_modules,target}/' .

  # Count only Go and Python files
  locq -l Go,Python .

  # Report each file, sorted by code lines
  locq -f -s code .

  # Export JSON outside the scanned directory
  locq -o json . > ../counts.json

  # Count identical contents only once
  locq --dedup .`,
		Args:               cobra.ArbitraryArgs,
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(cmd *cobra.Command, paths []string) error {
			analysisOpts := &locq.Options{Workers: opts.Workers}
			if err := core.ValidateWorkers(opts.Workers); err != nil {
				return fmt.Errorf("--workers %w", err)
			}
			if opts.ShowVersion {
				info, _ := debug.ReadBuildInfo()
				_, err := fmt.Fprintln(cmd.OutOrStdout(), versionString(Version, GitCommit, info))
				return err
			}
			if opts.ShowLang {
				languages := locq.NewDefinedLanguages()
				_, err := fmt.Fprintln(cmd.OutOrStdout(), languages.GetFormattedString())
				return err
			}
			if len(paths) == 0 {
				return cmd.Help()
			}
			analysisOpts.Diagnostics = cmd.ErrOrStderr()
			return runAnalysis(
				paths,
				opts,
				analysisOpts,
				cmd.OutOrStdout(),
			)
		},
	}
	command.DisableFlagsInUseLine = true
	command.SetUsageTemplate(`Usage:
  {{.UseLine}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}
`)
	flags := command.Flags()
	flags.SetInterspersed(true)
	flags.BoolVarP(
		&opts.ByFile,
		"by-file",
		"f",
		false,
		"report results for every encountered source file",
	)
	opts.SortTag = "code"
	flags.VarP(
		(*sortTagValue)(&opts.SortTag),
		"sort",
		"s",
		"sort based on a certain column [name,files,blank,comment,code]",
	)
	flags.StringVarP(
		&opts.OutputType,
		"output-type",
		"o",
		OutputTypeDefault,
		"output type [values: default,markdown,cloc-xml,sloccount,json]",
	)
	flags.StringVar(
		&opts.ExcludeLang,
		"exclude-lang",
		"",
		"exclude language names (separated commas)",
	)
	flags.StringArrayVar(
		&opts.Exclude,
		"exclude",
		[]string{},
		"exclude files or directories matching a glob (repeatable; trailing / matches directories only)",
	)
	flags.StringVarP(
		&opts.IncludeLang,
		"include-lang",
		"l",
		"",
		"include language names (separated commas)",
	)
	flags.StringArrayVar(&opts.Include, "include", nil, "include matching files or directory trees (glob; repeatable; patterns are ORed)")
	flags.BoolVar(
		&opts.Debug,
		"debug",
		false,
		"log analysis activity to stderr (files may interleave)",
	)
	flags.VarP(
		(*workerCountValue)(&opts.Workers),
		"workers",
		"w",
		"number of file analysis workers (0=automatic, 1-64)",
	)
	flags.BoolVar(
		&opts.Dedup,
		"dedup",
		false,
		"count files with identical contents only once",
	)
	flags.BoolVarP(
		&opts.ShowLang,
		"show-lang",
		"L",
		false,
		"print about all languages and extensions",
	)
	flags.BoolVarP(
		&opts.ShowVersion,
		"version",
		"V",
		false,
		"print version info",
	)
	return command
}

// go-flags parsed integers in base ten; pflag's IntVar instead uses base zero.
// Keep values such as --workers=08 decimal and reject hexadecimal notation.
type workerCountValue int

func (v *workerCountValue) String() string { return strconv.Itoa(int(*v)) }
func (v *workerCountValue) Type() string   { return "int" }

func (v *workerCountValue) Set(value string) error {
	count, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid worker count: %w", err)
	}
	*v = workerCountValue(count)
	return nil
}

// Validate each occurrence during parsing, including before informational exits.
type sortTagValue string

func (v *sortTagValue) String() string { return string(*v) }
func (v *sortTagValue) Type() string   { return "string" }

func (v *sortTagValue) Set(value string) error {
	switch value {
	case "name", "files", "blank", "comment", "code":
		*v = sortTagValue(value)
		return nil
	default:
		return fmt.Errorf("must be one of name, files, blank, comment or code")
	}
}

func runAnalysis(paths []string, opts CmdOptions, analysisOpts *locq.Options, out io.Writer) error {
	// check sort tag option with other options
	if opts.ByFile && opts.SortTag == "files" {
		return fmt.Errorf("`--sort files` option cannot be used in conjunction with the `--by-file` option")
	}
	analysisOpts.ExcludeLangs = splitLanguageNames(opts.ExcludeLang)
	analysisOpts.Exclude = opts.Exclude
	analysisOpts.Include = opts.Include
	analysisOpts.IncludeLangs = splitLanguageNames(opts.IncludeLang)
	analysisOpts.Debug = opts.Debug
	analysisOpts.Dedup = opts.Dedup

	result, err := locq.Analyze(paths, analysisOpts)
	if err != nil {
		var optionErr *locq.OptionError
		if errors.As(err, &optionErr) {
			flag := map[string]string{
				"Exclude": "exclude", "Include": "include", "Workers": "workers",
				"IncludeLangs": "include-lang", "ExcludeLangs": "exclude-lang",
			}[optionErr.Field]
			if flag != "" {
				return fmt.Errorf("invalid --%s: %w", flag, optionErr.Err)
			}
		}
		return fmt.Errorf("fail locq analyze: %w", err)
	}

	return newOutputBuilder(result, &opts, out).WriteResult()
}

func splitLanguageNames(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}
