package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

const metadataPath = "upstream.json"
const upstreamRepository = "https://github.com/XAMPPRocky/tokei"
const maxDownloadBytes = 4 << 20

type sourceMetadata struct {
	Commit    string `json:"commit"`
	SHA256    string `json:"sha256"`
	Languages int    `json:"languages"`
}

func readSourceMetadata(path string) (sourceMetadata, error) {
	var metadata sourceMetadata
	content, err := os.ReadFile(path)
	if err != nil {
		return metadata, fmt.Errorf("read upstream metadata: %w", err)
	}
	if err := decodeJSON(content, &metadata); err != nil {
		return metadata, fmt.Errorf("decode upstream metadata: %w", err)
	}
	if err := validateHex(metadata.Commit, 20); err != nil {
		return metadata, fmt.Errorf("invalid upstream commit: %w", err)
	}
	if err := validateHex(metadata.SHA256, sha256.Size); err != nil {
		return metadata, fmt.Errorf("invalid upstream checksum: %w", err)
	}
	if metadata.Languages <= 0 {
		return metadata, fmt.Errorf("upstream catalog must not be empty")
	}
	return metadata, nil
}

func validateHex(value string, size int) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != size {
		return fmt.Errorf("expected %d hexadecimal characters", size*2)
	}
	return nil
}

// updateRules resolves the latest commit touching languages.json before fetching
// immutable bytes. Validate and compile everything before changing local files.
func updateRules(client *http.Client, ref, directory string) error {
	query := url.Values{"path": {sourcePath}, "per_page": {"1"}, "sha": {ref}}
	endpoint := "https://api.github.com/repos/XAMPPRocky/tokei/commits?" + query.Encode()
	content, err := download(client, endpoint, "application/vnd.github+json")
	if err != nil {
		return err
	}
	commits := []struct {
		SHA string `json:"sha"`
	}{}
	if err := json.Unmarshal(content, &commits); err != nil {
		return fmt.Errorf("decode upstream commits: %w", err)
	}
	if len(commits) != 1 {
		return fmt.Errorf("expected one upstream language-rules commit, got %d", len(commits))
	}
	commit := commits[0].SHA
	if err := validateHex(commit, 20); err != nil {
		return fmt.Errorf("invalid upstream commit: %w", err)
	}
	endpoint = "https://api.github.com/repos/XAMPPRocky/tokei/contents/languages.json?ref=" + commit
	content, err = download(client, endpoint, "application/vnd.github.raw+json")
	if err != nil {
		return err
	}
	if bytes.Contains(content, []byte("\r\n")) {
		return fmt.Errorf("upstream snapshot must use LF line endings")
	}
	var upstream upstreamRules
	if err := decodeJSON(content, &upstream); err != nil {
		return fmt.Errorf("decode upstream rules: %w", err)
	}
	metadata := sourceMetadata{
		Commit: commit, SHA256: fmt.Sprintf("%x", sha256.Sum256(content)), Languages: len(upstream.Languages),
	}
	code, report, err := compileSnapshot(content, metadata, directory)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode upstream metadata: %w", err)
	}
	for _, file := range []struct {
		path string
		data []byte
	}{
		{path: sourcePath, data: content},
		{path: metadataPath, data: append(encoded, '\n')},
	} {
		if err := os.WriteFile(filepath.Join(directory, file.path), file.data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", file.path, err)
		}
	}
	if err := writeArtifacts(directory, code, report); err != nil {
		return err
	}
	fmt.Printf("Upstream: %s/blob/%s/languages.json\n", upstreamRepository, commit)
	fmt.Printf("Validated %d language rules (SHA-256 %s)\n", metadata.Languages, metadata.SHA256)
	return nil
}

func download(client *http.Client, endpoint, mediaType string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}
	request.Header.Set("User-Agent", "gocloc-language-rules-sync")
	request.Header.Set("Accept", mediaType)
	if request.URL.Host == "api.github.com" {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch upstream rules: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", endpoint, response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}
	if len(content) > maxDownloadBytes {
		return nil, fmt.Errorf("upstream response exceeds %d bytes", maxDownloadBytes)
	}
	return content, nil
}
