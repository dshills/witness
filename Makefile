BINARY := witness
PKG    := ./cmd/witness
BIN_DIR := bin

.DEFAULT_GOAL := help

.PHONY: help build install test race lint clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the witness binary into ./bin
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) $(PKG)

install: ## Install witness to $GOBIN / $GOPATH/bin
	go install $(PKG)

test: ## Run the full test suite
	go test ./...

race: ## Run the test suite under the race detector
	go test -race ./...

lint: ## Run golangci-lint across the module
	golangci-lint run ./...

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
