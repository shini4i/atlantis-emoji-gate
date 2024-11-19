.DEFAULT_GOAL := help

.PHONY: help
help: ## Print this help
	@echo "Usage: make [target]"
	@grep -E '^[a-z.A-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST)  | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: install-deps
install-deps: ## Install dependencies
	@echo "===> Installing dependencies"
	@go install github.com/jstemmer/go-junit-report@latest

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@go test -v -coverprofile=coverage.out ./... -count=1 2>&1 | tee /dev/stderr | go-junit-report -set-exit-code > report.xml

.PHONY: test
test: ## Run tests
	@go test -v ./... -count=1

.PHONY: build
build: ## Build the binary
	@GOOS=linux GOARCH=amd64 GO_ENABLED=0 go build -ldflags="-s -w" -o bin/atlantis-emoji-gate ./cmd/emoji-gate
