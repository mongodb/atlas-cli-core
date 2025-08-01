TEST_CMD?=go test
UNIT_TAGS?=unit
COVERAGE=coverage.out

ifeq ($(OS),Windows_NT)
	export PATH := .\bin;$(shell go env GOPATH)\bin;$(PATH)
else
	export PATH := ./bin:$(shell go env GOPATH)/bin:$(PATH)
endif
export TERM := linux-m
export GO111MODULE := on
export GOTOOLCHAIN := local

.PHONY: unit-test
unit-test: ## Run unit-tests
	@echo "==> Running unit tests..."
	$(TEST_CMD) --tags="$(UNIT_TAGS)" -race -cover -count=1 -coverprofile $(COVERAGE) ./...

.PHONY: gen-mocks
gen-mocks: ## Generate mocks
	@echo "==> Generating mocks"
	rm -rf ./mocks
	go generate ./...

.PHONY: deps
deps:  ## Download go module dependencies
	@echo "==> Installing go.mod dependencies..."
	go mod download
	go mod tidy

.PHONY: lint
lint: ## Run linter
	golangci-lint run
	
.PHONY: fix-lint
fix-lint: ## Fix linting errors
	golangci-lint run --fix

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'