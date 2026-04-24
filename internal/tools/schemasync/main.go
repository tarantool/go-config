// Package main is the schemasync CLI entry point.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// Config format is only stable starting with Tarantool 3.
const defaultMinMajor = 3

const defaultHTTPTimeout = 30 * time.Second

const schemaURLDefault = "https://download.tarantool.org/tarantool/schema/config.schema.%s.json"

func main() {
	var (
		repo        = flag.String("repo", "tarantool/tarantool", "GitHub repository in owner/name form.")
		schemasDir  = flag.String("schemas-dir", "tarantool/schemas", "Versioned-schema directory root.")
		schemaURL   = flag.String("schema-url", schemaURLDefault, "Schema URL template (%s = version).")
		minMajor    = flag.Int("min-major", defaultMinMajor, "Minimum major version to consider.")
		dryRun      = flag.Bool("dry-run", false, "Discover and validate without writing.")
		githubToken = flag.String("github-token", "", "GitHub API token; falls back to $GITHUB_TOKEN.")
	)

	flag.Parse()

	token := *githubToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg := Config{
		Repo:        *repo,
		SchemasDir:  *schemasDir,
		SchemaURL:   *schemaURL,
		GitHubToken: token,
		MinMajor:    *minMajor,
		DryRun:      *dryRun,
		HTTPClient:  &http.Client{Timeout: defaultHTTPTimeout}, //nolint:exhaustruct
		Logger:      logger,
	}

	err := Run(context.Background(), cfg)
	if err != nil {
		logger.Error("schemasync failed", "err", err)
		os.Exit(1)
	}
}
