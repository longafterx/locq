package core

import (
	"errors"
	"io"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/bmatcuk/doublestar/v4"
)

func TestOptionsPrepareAutomaticWorkers(t *testing.T) {

	previous := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	for _, limit := range []int{1, 3, MaxWorkers + 1} {
		t.Run("GOMAXPROCS="+strconv.Itoa(limit), func(t *testing.T) {
			runtime.GOMAXPROCS(limit)
			for _, opts := range []*Options{nil, {}} {
				prepared, err := opts.prepare()
				if err != nil {
					t.Fatal(err)
				}
				if prepared.Workers != min(limit, MaxWorkers) || prepared.Dedup {
					t.Fatalf("prepared=%+v, want automatic workers without deduplication", prepared)
				}
				if opts != nil && opts.Workers != 0 {
					t.Fatal("automatic selection mutated caller options")
				}
			}
		})
	}
}

func TestOptionsPrepareFixedWorkers(t *testing.T) {
	t.Parallel()
	for _, workers := range []int{1, 8, MaxWorkers} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			opts := &Options{Workers: workers}
			prepared, err := opts.prepare()
			if err != nil {
				t.Fatal(err)
			}
			if prepared.Workers != workers {
				t.Fatalf("workers=%d, want %d", prepared.Workers, workers)
			}
		})
	}
}

func TestOptionsPrepareInvalidFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		opts  Options
		field string
	}{
		{name: "negative workers", opts: Options{Workers: -1}, field: "Workers"},
		{name: "too many workers", opts: Options{Workers: MaxWorkers + 1}, field: "Workers"},
		{name: "include", opts: Options{Include: []string{"["}}, field: "Include"},
		{name: "exclude", opts: Options{Exclude: []string{"["}}, field: "Exclude"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared, err := tc.opts.prepare()
			var optionErr *OptionError
			if !errors.As(err, &optionErr) {
				t.Fatalf("error=%v, want OptionError", err)
			}
			if prepared != nil || optionErr.Field != tc.field {
				t.Fatalf(
					"prepared=%+v error=%v, want OptionError for %s",
					prepared,
					err,
					tc.field,
				)
			}
			if tc.field != "Workers" {
				if !errors.Is(err, doublestar.ErrBadPattern) {
					t.Fatalf("error=%v, want wrapped glob syntax error", err)
				}
			}
		})
	}
}

func TestOptionsPrepareFiltersAndOwnership(t *testing.T) {
	opts := &Options{Workers: 4, Dedup: true, Debug: true, Diagnostics: io.Discard,
		Include: []string{"src/", "*.go"}, Exclude: []string{"target/", "*.min.js"},
		IncludeLangs: []string{"Go", "Python", "Go"}, ExcludeLangs: []string{"Python"}}
	before := *opts
	config, err := opts.prepare()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*opts, before) {
		t.Fatal("preparation mutated options")
	}
	if !config.Dedup || !config.Debug || config.Workers != 4 {
		t.Fatalf("config=%+v", config)
	}
	opts.Include[0] = "changed/"
	opts.Exclude[0] = "changed/"
	opts.IncludeLangs[0] = "JSON"
	opts.ExcludeLangs[0] = "JSON"
	if config.filter.include.rules[0].pattern != "src" || config.filter.exclude.rules[0].pattern != "target" {
		t.Fatal("glob rules alias caller input")
	}
	if _, ok := config.IncludeLangs["Go"]; !ok {
		t.Fatal("included languages alias input")
	}
	if _, ok := config.ExcludeLangs["Python"]; !ok {
		t.Fatal("excluded languages alias input")
	}
}

func TestConfigurationSnapshotsCustomDefinitions(t *testing.T) {
	custom := NewLanguageDefinition("Custom", []string{"#"}, [][]string{{"{-", "-}"}})
	opts := &Options{
		Definitions: map[string]*LanguageDefinition{"Custom": custom},
		Extensions:  map[string]string{"custom": "Custom"},
	}
	config, err := opts.prepare()
	if err != nil {
		t.Fatal(err)
	}
	custom.Name = "changed"
	custom.lineComments[0] = "!"
	custom.multiLines[0][0] = "!"
	opts.Extensions["custom"] = "Go"
	delete(opts.Definitions, "Custom")
	snapshot := config.languages.Langs["Custom"]
	if config.exts["custom"] != "Custom" || snapshot.Name != "Custom" || snapshot.lineComments[0] != "#" || snapshot.multiLines[0][0] != "{-" {
		t.Fatalf("snapshot aliases input: %+v", snapshot)
	}
}
