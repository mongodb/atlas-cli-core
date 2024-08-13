TEST_CMD?=go test
UNIT_TAGS?=unit
COVERAGE=coverage.out


.PHONY: unit-test
unit-test: ## Run unit-tests
	@echo "==> Running unit tests..."
	$(TEST_CMD) --tags="$(UNIT_TAGS)" -race -cover -count=1 -coverprofile $(COVERAGE) ./...