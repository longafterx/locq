package core

import (
	"crypto/md5"
	"sync"
)

type fileCandidate struct {
	sequence uint64
	path     string
}

type fileAnalysisResult struct {
	fileCandidate
	languageKey string
	err         error
	digest      [md5.Size]byte
	clocFile    *ClocFile
	ignored     bool
}

func parallelResultWindowSize(workers int) int {
	windowSize := 32 * workers
	if windowSize < 256 {
		return 256
	}
	return windowSize
}

func addFileToResult(result map[string]*Language, languages *DefinedLanguages, languageKey, path string) {
	if _, ok := result[languageKey]; !ok {
		result[languageKey] = &Language{Name: languages.Langs[languageKey].Name, Files: []string{}}
	}
	result[languageKey].Files = append(result[languageKey].Files, path)
}

// Workers analyze once; the ordered collector bounds pending results and keeps
// the first discovered duplicate independently of worker scheduling.
func scanAndAnalyzeFiles(
	paths []string,
	languages *DefinedLanguages,
	opts *runState,
) (map[string]*Language, map[string]*ClocFile, error) {
	workers := opts.Workers
	result := make(map[string]*Language)
	clocFiles := make(map[string]*ClocFile)
	fileCache := make(map[string]struct{})
	windowSize := parallelResultWindowSize(workers)
	tokens := make(chan struct{}, windowSize)
	for range windowSize {
		tokens <- struct{}{}
	}
	candidates := make(chan fileCandidate, windowSize)
	results := make(chan fileAnalysisResult, windowSize)

	var workerGroup sync.WaitGroup
	for range workers {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			reader := newLineReader(nil)
			for candidate := range candidates {
				item := detectAndAnalyzeFile(
					candidate,
					languages,
					opts,
					reader,
				)
				results <- item
			}
		}()
	}

	walkErrors := make(chan error, 1)
	go func() {
		var walkErr error
		sequence := uint64(0)
		for _, root := range paths {
			walkErr = walkCandidateFiles(root, opts, func(path string) error {
				<-tokens
				candidates <- fileCandidate{sequence: sequence, path: path}
				sequence++
				return nil
			})
			if walkErr != nil {
				break
			}
		}
		close(candidates)
		walkErrors <- walkErr
	}()

	go func() {
		workerGroup.Wait()
		close(results)
	}()

	pending := make(map[uint64]fileAnalysisResult)
	nextSequence := uint64(0)
	for resultItem := range results {
		pending[resultItem.sequence] = resultItem
		for {
			current, ok := pending[nextSequence]
			if !ok {
				break
			}
			delete(pending, nextSequence)
			if current.err != nil {
				opts.warn(current.err)
				current.ignored = true
			}
			if !current.ignored && opts.Dedup {
				current.ignored = duplicateDigest(current.digest, fileCache)
				if current.ignored && opts.Debug {
					opts.diagnosticf("[SKIP] file=%q reason=\"duplicate content\"\n", current.path)
				}
			}
			if !current.ignored {
				addFileToResult(
					result,
					languages,
					current.languageKey,
					current.path,
				)
				language := result[current.languageKey]
				addFileCounts(language, current.clocFile)
				clocFiles[current.path] = current.clocFile
			}
			tokens <- struct{}{}
			nextSequence++
		}
	}

	return result, clocFiles, <-walkErrors
}

func includeLanguage(languageKey string, opts *runState) bool {
	if _, excluded := opts.ExcludeLangs[languageKey]; excluded {
		return false
	}
	if len(opts.IncludeLangs) == 0 {
		return true
	}
	_, included := opts.IncludeLangs[languageKey]
	return included
}

func addFileCounts(language *Language, file *ClocFile) {
	language.Code += file.Code
	language.Comments += file.Comments
	language.Blanks += file.Blanks
}
