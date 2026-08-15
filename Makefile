SHELL := /bin/bash

# ── 24-bit Custom Palette ──
RESET        := \033[0m
BOLD         := \033[1m
DIM          := \033[2m
ITALIC       := \033[3m

# Blue-to-Purple Gradient Steps
B1           := \033[38;2;0;210;255m
B2           := \033[38;2;30;160;255m
B3           := \033[38;2;85;110;255m
B4           := \033[38;2;135;60;255m
B5           := \033[38;2;185;15;255m

# Custom Accent Colors
TAG_ORANGE   := \033[38;2;255;140;0m
CMD_PURPLE   := \033[38;2;175;80;255m
VINTAGE_GRN  := \033[38;2;138;171;143m

# Status Colors
SUCCESS      := \033[38;2;0;235;150m
WARNING      := \033[38;2;255;170;0m
ERROR        := \033[38;2;255;60;100m
MUTED        := \033[38;2;110;120;160m

# ── Configuration ──
BINARY_NAME          := webtoon-dl
BUILD_DIR            := bin
COMPLEXITY_THRESHOLD := 10

# ── Tooling Setup (Cross-Platform / CI Compliant) ──
GO       := go
GOPATH   := $(shell $(GO) env GOPATH)
GOBIN    := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
	GOBIN := $(GOPATH)/bin
endif

GOCYCLO  := $(GOBIN)/gocyclo
GOLANGCI := $(GOBIN)/golangci-lint

.PHONY: all build test coverage lint complexity static-analysis clean tools help doctortoon update fmt run

all: static-analysis test build

# ── Build & Test ──────────────────────────────────────────────────────────────

build: ## Build binary
	@printf "$(B1)▸$(RESET) Building $(BOLD)$(CMD_PURPLE)$(BINARY_NAME)$(RESET)…\n"
	@mkdir -p $(BUILD_DIR)
	@$(GO) build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/webtoon-dl \
		&& printf " $(SUCCESS)✔$(RESET) Build successful -> $(DIM)$(BUILD_DIR)/$(BINARY_NAME)$(RESET)\n"

run: ## Run application
	@printf "$(B2)▸$(RESET) Executing: ./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)\n"
	@./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

test: ## Run unit tests
	@printf "$(B2)▸$(RESET) Running unit tests…\n"
	@if $(GO) test -race -cover -v ./...; then \
		printf "  $(SUCCESS)✔$(RESET) All tests passed\n"; \
	else \
		printf "  $(ERROR)✘$(RESET) Tests failed\n"; \
		exit 1; \
	fi

coverage: ## Generate test coverage
	@printf "$(B2)▸$(RESET) Generating test coverage report…\n"
	@mkdir -p .build
	@$(GO) test -coverprofile=.build/coverage.out ./...
	@$(GO) tool cover -html=.build/coverage.out -o .build/coverage.html \
		&& printf " $(SUCCESS)✔$(RESET) Coverage report generated at $(DIM).build/coverage.html$(RESET)\n"

# ── Quality & Static Analysis ─────────────────────────────────────────────────

static-analysis: ## Run all checks
	@printf "$(B3)▸$(RESET) Running static analysis suite…\n"
	@$(MAKE) complexity lint

complexity: ## Check complexity
	@printf "$(B3)▸$(RESET) Checking cyclomatic complexity…\n"
	@if command -v gocyclo >/dev/null 2>&1; then \
		OVER_LIMIT=$$(gocyclo -over $(COMPLEXITY_THRESHOLD) .); \
		if [ -n "$$OVER_LIMIT" ]; then \
			printf "  $(ERROR)✘$(RESET) Complexity issues found:\n$$OVER_LIMIT\n"; \
			exit 1; \
		else \
			printf "  $(SUCCESS)✔$(RESET) Complexity check passed\n"; \
		fi; \
	elif [ -f "$(GOCYCLO)" ]; then \
		OVER_LIMIT=$$($(GOCYCLO) -over $(COMPLEXITY_THRESHOLD) .); \
		if [ -n "$$OVER_LIMIT" ]; then \
			printf "  $(ERROR)✘$(RESET) Complexity issues found:\n$$OVER_LIMIT\n"; \
			exit 1; \
		else \
			printf "  $(SUCCESS)✔$(RESET) Complexity check passed\n"; \
		fi; \
	else \
		printf "  $(WARNING)⚠$(RESET) gocyclo not found. Installing...\n"; \
		$(GO) install github.com/fzipp/gocyclo/cmd/gocyclo@latest; \
		$(GOCYCLO) -over $(COMPLEXITY_THRESHOLD) . && printf "  $(SUCCESS)✔$(RESET) Complexity check passed\n"; \
	fi

lint: ## Run linter
	@printf "$(B4)▸$(RESET) Running linter…\n"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		if golangci-lint run ./... --timeout=5m; then \
			printf "  $(SUCCESS)✔$(RESET) Linting passed\n"; \
		else \
			printf "  $(ERROR)✘$(RESET) Linting failed\n"; \
			exit 1; \
		fi; \
	elif [ -f "$(GOLANGCI)" ]; then \
		if $(GOLANGCI) run ./... --timeout=5m; then \
			printf "  $(SUCCESS)✔$(RESET) Linting passed\n"; \
		else \
			printf "  $(ERROR)✘$(RESET) Linting failed\n"; \
			exit 1; \
		fi; \
	else \
		printf "  $(WARNING)⚠$(RESET) golangci-lint is not installed. Run 'make tools' to install.\n"; \
	fi

fmt: ## Format code
	@printf "$(B2)▸$(RESET) Formatting source code…\n"
	@$(GO) fmt ./... \
		&& printf " $(SUCCESS)✔$(RESET) Code formatted\n"

# ── Utilities ─────────────────────────────────────────────────────────────────

clean: ## Clean workspace
	@printf "$(WARNING)▸$(RESET) Cleaning workspace…\n"
	@rm -rf $(BUILD_DIR) webtoon-dl *.cache *.tmp .build
	@$(GO) clean \
		&& printf " $(SUCCESS)✔$(RESET) Workspace clean\n"

doctortoon: build ## Run diagnostics
	@./$(BUILD_DIR)/$(BINARY_NAME) doctor

update: ## Update dependencies
	@printf "$(B4)▸$(RESET) Updating dependencies…\n"
	@$(GO) mod tidy \
		&& printf " $(SUCCESS)✔$(RESET) Dependencies updated\n"

tools: ## Install dev tools
	@printf "$(B1)▸$(RESET) Installing developer tools…\n"
	@$(GO) install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@printf " $(SUCCESS)✔$(RESET) Developer tools installed\n"

# ── Responsive Help Menu ──────────────────────────────────────────────────────

help: ## Show this help menu
	@COLS=$$(tput cols 2>/dev/null); \
	if [ -z "$$COLS" ] || [ "$$COLS" -eq 0 ]; then COLS=80; fi; \
	printf "\n"; \
	if [ $$COLS -ge 65 ]; then \
		printf "  $(B1)█   █ █████ ████  █████ █████ █████ ███╗   ██╗    ████╗  ██╗   $(RESET)\n"; \
		printf "  $(B2)█   █ █     █   █   █   █   █ █   █ ████╗  ██║    █   █  █     $(RESET)\n"; \
		printf "  $(B3)█ █ █ ███   ████    █   █   █ █   █ ██╔██╗ ██║    █   █  █     $(RESET)\n"; \
		printf "  $(B4)███║█ █     █   █   █   █   █ █   █ ██║╚██╗██║    █   █  █     $(RESET)\n"; \
		printf "  $(B5)█   █ █████ ████    █   █████ █████ ██║ ╚████║    ████╗  █████ $(RESET)\n"; \
		printf "  $(MUTED)─────────────────────────────────────────────────────────────$(RESET)\n"; \
	else \
		printf "  $(BOLD)$(B1)WEBTOON-DL$(RESET)\n"; \
		printf "  $(MUTED)─────────────────────────────────$(RESET)\n"; \
	fi; \
	printf "  $(BOLD)$(CMD_PURPLE)V E R S I O N  2 . 1 . 7$(RESET)\n"; \
	printf "  $(MUTED)─────────────────────────────────$(RESET)\n"; \
	printf "  $(DIM)$(VINTAGE_GRN)Powered by$(RESET) $(TAG_ORANGE)FJ™ - Cybertronic Systems$(RESET)\n\n"; \
	printf "  $(BOLD)$(CMD_PURPLE)COMMANDS$(RESET)\n"; \
	printf "  $(MUTED)─────────────────────────────────$(RESET)\n"; \
	grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  $(B4)%-16s$(RESET) $(MUTED)─$(RESET) $(B1)%s$(RESET)\n", $$1, $$2}' \
		| sort; \
	printf "\n  $(BOLD)$(VINTAGE_GRN)USAGE$(RESET)\n"; \
	printf "  $(MUTED)─────────────────────────────────$(RESET)\n"; \
	printf "  $(VINTAGE_GRN)make$(RESET) $(CMD_PURPLE)<command>$(RESET)  $(MUTED)|$(RESET)  $(TAG_ORANGE)make run ARGS=\"download 'Set It!'\"$(RESET)\n\n"
