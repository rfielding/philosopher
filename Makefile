.PHONY: build test test-go test-lisp run clean package prompt

# Build the binary
build:
	go build -o philosopher main.go tools.go mcp_tools.go

# Run all tests
test: test-go test-lisp

# Go unit tests
test-go:
	go test -v

# LISP integration tests
test-lisp: build
	@echo "=== Running LISP Tests ==="
	@echo ""
	@echo "--- datalog-tests.lisp ---"
	./philosopher datalog-tests.lisp
	@echo ""
	@echo "--- breadco.lisp ---"
	./philosopher breadco.lisp
	@echo ""
	@echo "=== LISP Tests Complete ==="

# Run web server
run: build
	./philosopher

server: build
	./philosopher

# Run MCP server
mcp: build
	./philosopher mcp

# Run a prompt through LLM (usage: make prompt P="your prompt here")
# Or: make prompt P=prompts/test-01-counter.md
prompt: build
	@if [ -z "$(P)" ]; then \
		echo "Usage: make prompt P=\"your prompt here\""; \
		echo "   or: make prompt P=prompts/test-01-counter.md"; \
		exit 1; \
	fi
	./philosopher -prompt "$(P)"

# Clean build artifacts
clean:
	rm -f philosopher files.zip

# Package for distribution
package: build test-go
	rm -f files.zip
	zip -r files.zip \
		main.go tools.go mcp_tools.go \
		datalog_test.go prompt_test.go \
		datalog-tests.lisp breadco.lisp \
		README-MCP.md ARCHITECTURE.md \
		go.mod prompts/ Makefile

# Quick build check
check:
	go build -o /dev/null main.go tools.go mcp_tools.go

# Run specific LISP file
%.lisp: build
	./philosopher $@
