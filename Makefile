GOTEST := go test
TAGS :=
COVERAGE_FILE := coverage.out

.PHONY: codespell
codespell:
	@echo "Running codespell"
	@codespell

.PHONY: test
test:
	@echo "Running tests"
	@go test ./... -count=1

.PHONY: testrace
testrace:
	@echo "Running tests with race flag"
	@go test ./... -count=100 -race

.PHONY: coverage
coverage:
	@echo "Running tests with coverage (excluding internal/testutil)"
	go test -tags "$(TAGS)" ./... -v -p 1 -covermode=atomic -coverprofile=$(COVERAGE_FILE) -count=1
	@echo "Excluding internal/testutil from coverage report"
	@grep -v "internal/testutil" $(COVERAGE_FILE) > $(COVERAGE_FILE).tmp && mv $(COVERAGE_FILE).tmp $(COVERAGE_FILE)
	go tool cover -func=$(COVERAGE_FILE)

.PHONY: deps
deps:
	@echo "Installing lint deps"
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
	@echo "Installing govulncheck"
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Install goveralls"
	@go install github.com/mattn/goveralls@latest

.PHONY: coveralls
coveralls:
	@echo "uploading coverage to coveralls"
	@goveralls -coverprofile=$(COVERAGE_FILE) -service=github

.PHONY: lint
lint:
	@echo "Running go-linter"
	@golangci-lint run --config=./.golangci.yml ./...

.PHONY: govulncheck
govulncheck:
	@echo "Running govulncheck"
	@govulncheck ./...
