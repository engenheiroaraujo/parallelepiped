# parallelepiped — common development tasks.
# Recipes avoid shell-specific syntax so they run under sh (Git Bash) and cmd.

GO       ?= go
NPM      ?= npm
WAILS    ?= $(shell $(GO) env GOPATH)/bin/wails
FRONTEND := frontend

.DEFAULT_GOAL := help

.PHONY: help
help: ## List the available targets
	@$(GO) run ./tools/makehelp Makefile

.PHONY: tools
tools: ## Install the Wails CLI used by dev/build
	$(GO) install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

.PHONY: deps
deps: ## Download Go modules and install frontend packages
	$(GO) mod download
	$(NPM) --prefix $(FRONTEND) install

.PHONY: dev
dev: ## Run the app with hot reload
	$(WAILS) dev

.PHONY: build
build: ## Build the production executable into build/bin
	$(WAILS) build -clean -trimpath

.PHONY: installer
installer: ## Build the executable plus an NSIS installer (needs makensis)
	$(WAILS) build -clean -trimpath -nsis

.PHONY: bindings
bindings: ## Regenerate the TypeScript bindings in frontend/wailsjs
	$(WAILS) generate module

.PHONY: fmt
fmt: ## Format Go code
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: test
test: ## Run Go tests
	$(GO) test ./...

.PHONY: typecheck
typecheck: ## Type-check the frontend
	$(NPM) --prefix $(FRONTEND) run typecheck

.PHONY: check
check: vet test typecheck ## Run every check (vet, tests, typecheck)

.PHONY: clean
clean: ## Remove build output
	$(GO) clean
	node -e "require('fs').rmSync('build/bin',{recursive:true,force:true})"
