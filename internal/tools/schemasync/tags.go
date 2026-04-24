package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var stableTagRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

var linkNextRE = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

var errTagsStatus = errors.New("github tags: unexpected http status")

// filterStableTags strips a leading "v" from output (e.g. "v3.7.1" → "3.7.1")
// and preserves input order.
func filterStableTags(names []string, minMajor int) []string {
	out := make([]string, 0, len(names))

	for _, name := range names {
		match := stableTagRE.FindStringSubmatch(name)
		if match == nil {
			continue
		}

		major, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		if major < minMajor {
			continue
		}

		out = append(out, strings.TrimPrefix(name, "v"))
	}

	return out
}

// fetchTags returns names in API order (newest first). Empty token works for
// public repos but hits a tighter rate limit.
func fetchTags(ctx context.Context, client *http.Client, repo, token string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=100", repo)

	var names []string

	for url != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github+json")

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http do %s: %w", url, err)
		}

		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()

		if readErr != nil {
			return nil, fmt.Errorf("read body %s: %w", url, readErr)
		}

		if closeErr != nil {
			return nil, fmt.Errorf("close body %s: %w", url, closeErr)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: %d for %s: %s", errTagsStatus, resp.StatusCode, url, truncate(body))
		}

		var page []struct {
			Name string `json:"name"`
		}

		err = json.Unmarshal(body, &page)
		if err != nil {
			return nil, fmt.Errorf("decode tags %s: %w", url, err)
		}

		for _, entry := range page {
			names = append(names, entry.Name)
		}

		url = nextLink(resp.Header.Get("Link"))
	}

	return names, nil
}

func nextLink(header string) string {
	if header == "" {
		return ""
	}

	match := linkNextRE.FindStringSubmatch(header)
	if match == nil {
		return ""
	}

	return match[1]
}

func truncate(body []byte) string {
	const maxLen = 256

	if len(body) <= maxLen {
		return string(body)
	}

	return string(body[:maxLen]) + "..."
}
