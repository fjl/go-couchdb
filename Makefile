.PHONY: test
test: # Run tests
	go test -race ./...

.PHONY: fmt
fmt: # Fix code formatting
	go fmt ./...
