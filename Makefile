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
	@echo "Running tests with coveralls"
	go test -tags "$(TAGS)" ./... -v -p 1 -covermode=atomic -coverprofile=$(COVERAGE_FILE) -count=1
	go tool cover -func=$(COVERAGE_FILE)

.PHONY: coveralls
coveralls:
	@echo "uploading coverage to coveralls"
	@goveralls -coverprofile=$(COVERAGE_FILE) -service=github

.PHONY: coveralls-deps
coveralls-deps:
	@echo "Installing coveralls"
	@go get github.com/mattn/goveralls
	@go install github.com/mattn/goveralls

.PHONY: deps
deps:
	@echo "Installing lint deps"
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
	@echo "Installing govulncheck"
	@go install golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: lint
lint:
	@echo "Running go-linter"
	@go mod tidy
	@go mod vendor
	@golangci-lint run --config=./.golangci.yml --modules-download-mode vendor

.PHONY: govulncheck
govulncheck:
	@echo "Running govulncheck"
	@govulncheck ./...
