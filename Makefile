# ──────────────────────────────────────────────────────────────
# Nexio Makefile
# ──────────────────────────────────────────────────────────────

BINARY    := nexio
PKG       := ./cmd/nexio
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X main.version=$(VERSION)
TEST_DIR  := cmd/nexio

# Cross-compilation targets (must match release.yml matrix)
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64

# ── Default ──────────────────────────────────────────────────

.DEFAULT_GOAL := help

# ── Development ──────────────────────────────────────────────

.PHONY: build
build: ## Build the binary
	@go build -o $(BINARY) $(PKG)

.PHONY: build-release
build-release: ## Build a release-optimised binary (stripped, versioned)
	@go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(PKG)

.PHONY: install
install: ## Install nexio to $GOPATH/bin
	@go install -ldflags="$(LDFLAGS)" $(PKG)

.PHONY: run
run: build ## Build and run nexio (pass ARGS, e.g. make run ARGS="status")
	@./$(BINARY) $(ARGS)

# ── Quality ──────────────────────────────────────────────────

.PHONY: fmt
fmt: ## Format all Go source files
	@gofmt -w .

.PHONY: fmt-check
fmt-check: ## Check formatting (fails if files need changes)
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "Files need formatting:"; \
		gofmt -l .; \
		exit 1; \
	fi

.PHONY: vet
vet: ## Run go vet
	@go vet ./...

.PHONY: lint
lint: vet fmt-check ## Run all linters (vet + format check)

# ── Testing ──────────────────────────────────────────────────

.PHONY: test
test: ## Run tests with coverage (verbose)
	@cd $(TEST_DIR) && \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		NEXIO_ENV=test go test -cover -v ./...; \
		STATUS=$$?; \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		exit $$STATUS

.PHONY: test-short
test-short: ## Run tests (quiet, no verbose output)
	@cd $(TEST_DIR) && \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		NEXIO_ENV=test go test -cover ./...; \
		STATUS=$$?; \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		exit $$STATUS

.PHONY: test-run
test-run: ## Run a specific test (e.g. make test-run RUN=TestInit)
	@cd $(TEST_DIR) && \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		NEXIO_ENV=test go test -cover -v -run $(RUN) ./...; \
		STATUS=$$?; \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		exit $$STATUS

.PHONY: coverage
coverage: ## Generate and open an HTML coverage report
	@cd $(TEST_DIR) && \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		NEXIO_ENV=test go test -coverprofile=coverage.out ./...; \
		STATUS=$$?; \
		rm -rf .nexio __test__ *.txt subdir 2>/dev/null; \
		if [ $$STATUS -eq 0 ]; then \
			go tool cover -html=coverage.out -o coverage.html; \
			echo "Coverage report: $(TEST_DIR)/coverage.html"; \
		fi; \
		exit $$STATUS

# ── Dependencies ─────────────────────────────────────────────

.PHONY: deps
deps: ## Download module dependencies
	@go mod download

.PHONY: tidy
tidy: ## Tidy and verify module dependencies
	@go mod tidy

# ── Release / Cross-compilation ──────────────────────────────

.PHONY: build-all
build-all: ## Cross-compile for all release platforms
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(BINARY)-$$os-$$arch; \
		echo "Building $$output ..."; \
		GOOS=$$os GOARCH=$$arch go build -ldflags="$(LDFLAGS)" -o $$output $(PKG); \
	done

.PHONY: dist
dist: build-all ## Cross-compile and package tarballs into dist/
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		name=$(BINARY)-$$os-$$arch; \
		tar -czf dist/$$name.tar.gz $$name; \
		rm -f $$name; \
	done
	@echo "Tarballs written to dist/"

# ── Cleanup ──────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts and test leftovers
	@rm -f $(BINARY)
	@rm -f $(BINARY)-darwin-* $(BINARY)-linux-*
	@rm -rf dist
	@cd $(TEST_DIR) && rm -rf .nexio __test__ *.txt subdir coverage.out coverage.html 2>/dev/null
	@echo "Clean."

# ── Git Hooks ────────────────────────────────────────────────

.PHONY: install-hooks
install-hooks: ## Install the pre-commit Git hook
	@echo "> Installing Git hooks..."
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "> Pre-commit hook installed successfully!"
	@echo "\nINFO: The hook will run 'go vet' and 'gofmt' checks before each commit."

.PHONY: uninstall-hooks
uninstall-hooks: ## Uninstall the pre-commit Git hook
	@echo "> Uninstalling Git hooks..."
	@rm -f .git/hooks/pre-commit
	@echo "> Pre-commit hook uninstalled."

# ── CI (mirrors the GitHub Actions pipeline) ─────────────────

.PHONY: ci
ci: deps lint test build ## Run the full CI pipeline locally

# ── Setup ────────────────────────────────────────────────────

.PHONY: setup
setup: deps install-hooks ## First-time project setup (deps + hooks)

# ── Help ─────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@printf "\nUsage:\n  make \033[36m<target>\033[0m\n\n"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
