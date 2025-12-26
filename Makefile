.PHONY: test server repl build clean

# Run all tests
test:
	@cat prologue.lisp tests.lisp > /tmp/boundedlisp-tests.lisp
	@go run main.go /tmp/boundedlisp-tests.lisp

# Start web server (default)
server:
	go run main.go

# Interactive REPL with prologue loaded
repl:
	@cat prologue.lisp | go run main.go -repl

# Build binary
build:
	go build -o boundedlisp main.go

# Clean
clean:
	rm -f boundedlisp /tmp/boundedlisp-tests.lisp
