.PHONY: test
test: ## Run tests
	go test -race ./...

.PHONY: fmt
fmt: ## Fix code formatting
	go fmt ./...

.PHONY: help
help: ## Display help
	awk 'BEGIN { FS=": ##"; } /^[a-zA-Z_-]+: ##/ { printf("%-10s %s\n", $$1, $$2); }' $(MAKEFILE_LIST) | sort

