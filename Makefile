TEST_CMD?=go test
E2E_TEST_PACKAGES?=./test/e2e/...
COVERAGE=coverage.out

ifeq ($(OS),Windows_NT)
	export PATH := .\bin;$(shell go env GOPATH)\bin;$(PATH)
else
	export PATH := ./bin:$(shell go env GOPATH)/bin:$(PATH)
endif
export TERM := linux-m
export GO111MODULE := on
export GOTOOLCHAIN := local

export MONGODB_ATLAS_ORG_ID?=a0123456789abcdef012345a
export MONGODB_ATLAS_PROJECT_ID?=b0123456789abcdef012345b
export MONGODB_ATLAS_PUBLIC_API_KEY?=ABCDEF01
export MONGODB_ATLAS_PRIVATE_API_KEY?=12345678-abcd-ef01-2345-6789abcdef01
export MONGODB_ATLAS_OPS_MANAGER_URL?=http://localhost:8080/

.PHONY: unit-test
unit-test: ## Run unit-tests
	@echo "==> Running unit tests..."
	$(TEST_CMD) -short -cover -count=1 -coverprofile $(COVERAGE) ./...

.PHONY: e2e-test
e2e-test: ## Run end-to-end tests
	@echo "==> Running e2e tests..."
	$(TEST_CMD) -v -p 1 ${E2E_TEST_PACKAGES} -race -count=1 ./test/e2e/...

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
