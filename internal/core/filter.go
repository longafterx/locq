package core

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type globRule struct {
	pattern  string
	nameOnly bool
	dirOnly  bool
}

type pathExcluder struct {
	rules    []globRule
	hasPaths bool
}

type pathFilter struct {
	include  *pathExcluder
	exclude  *pathExcluder
	hasPaths bool
}

func preparePathFilter(includes, excludes []string) (*pathFilter, error) {
	if len(includes) == 0 && len(excludes) == 0 {
		return nil, nil
	}
	include, err := preparePatterns("Include", includes)
	if err != nil {
		return nil, err
	}
	exclude, err := preparePatterns("Exclude", excludes)
	if err != nil {
		return nil, err
	}
	return &pathFilter{include: include, exclude: exclude, hasPaths: include != nil || (exclude != nil && exclude.hasPaths)}, nil
}

func preparePatterns(field string, patterns []string) (*pathExcluder, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	filter := &pathExcluder{rules: make([]globRule, 0, len(patterns))}
	for i, pattern := range patterns {
		rule, err := prepareGlobRule(pattern)
		if err != nil {
			return nil, &OptionError{Field: field, Err: fmt.Errorf("pattern %d %q: %w", i+1, pattern, err)}
		}
		filter.rules = append(filter.rules, rule)
		filter.hasPaths = filter.hasPaths || !rule.nameOnly
	}
	return filter, nil
}

func prepareGlobRule(pattern string) (globRule, error) {
	for strings.HasPrefix(pattern, "./") {
		pattern = strings.TrimPrefix(pattern, "./")
	}
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	dirOnly := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	if pattern == "" {
		return globRule{}, errors.New("empty glob pattern")
	}
	if !doublestar.ValidatePattern(pattern) {
		return globRule{}, fmt.Errorf("invalid glob syntax: %w", doublestar.ErrBadPattern)
	}
	if strings.HasPrefix(pattern, "/") {
		return globRule{}, errors.New("pattern must be relative to a scan root")
	}
	if len(pattern) >= 2 && pattern[1] == ':' {
		letter := pattern[0]
		upperDrive := letter >= 'A' && letter <= 'Z'
		lowerDrive := letter >= 'a' && letter <= 'z'
		if upperDrive || lowerDrive {
			return globRule{}, errors.New("windows drive paths are not supported; use / relative to a scan root")
		}
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == ".." {
			return globRule{}, errors.New("parent directory segments are not supported")
		}
	}
	return globRule{
		pattern:  pattern,
		nameOnly: !anchored && !strings.Contains(pattern, "/"),
		dirOnly:  dirOnly,
	}, nil
}

// An empty relative path checks only name rules, including the scan root itself.
func (filter *pathExcluder) excluded(name, relative string, isDir bool) bool {
	if filter == nil {
		return false
	}
	for _, rule := range filter.rules {
		if rule.matches(name, relative, isDir) {
			return true
		}
	}
	return false
}

func (rule *globRule) matches(name, relative string, isDir bool) bool {
	if rule == nil {
		return false
	}
	if rule.dirOnly && !isDir {
		return false
	}
	target := name
	if !rule.nameOnly {
		if relative == "" {
			return false
		}
		target = relative
	}
	return doublestar.MatchUnvalidated(rule.pattern, target)
}

type rootFilter struct {
	filter       *pathFilter
	root         string
	rootName     string
	excludedRoot bool
}

func (filter *pathFilter) forRoot(root string, isDir bool) (rootFilter, error) {
	if !isDir {
		root = filepath.Dir(root)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return rootFilter{}, fmt.Errorf("resolve glob root %q: %w", root, err)
	}
	name := filepath.Base(absolute)
	return rootFilter{
		filter:       filter,
		root:         root,
		rootName:     name,
		excludedRoot: filter.exclude.excluded(name, "", true),
	}, nil
}

func (scope *rootFilter) accepts(filename string, entry fs.DirEntry) (bool, error) {
	if scope.excludedRoot {
		return false, nil
	}
	filter := scope.filter
	name := entry.Name()
	relative := ""
	switch {
	case filename == scope.root:
		name = scope.rootName
	case filter.hasPaths:
		pathFromRoot, err := filepath.Rel(scope.root, filename)
		if err != nil {
			return false, fmt.Errorf("make glob path %q relative to %q: %w", filename, scope.root, err)
		}
		relative = filepath.ToSlash(pathFromRoot)
	}
	if filter.exclude.excluded(name, relative, entry.IsDir()) {
		return false, nil
	}
	if entry.IsDir() {
		return true, nil
	}
	if filter.include == nil || filter.include.excluded(name, relative, false) {
		return true, nil
	}
	// An included directory includes every descendant, without pruning unmatched
	// ancestors while walking. Root names participate only in unanchored rules.
	for dir := path.Dir(relative); dir != "."; dir = path.Dir(dir) {
		if filter.include.excluded(path.Base(dir), dir, true) {
			return true, nil
		}
	}
	return filter.include.excluded(scope.rootName, "", true), nil
}
