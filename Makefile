.PHONY: build test run clean

build:
	go build -o philosopher main.go tools.go mcp_tools.go

test:
	go test -v

run: build
	./philosopher

server: build
	./philosopher

clean:
	rm -f philosopher files.zip

package: build test
	rm -f files.zip
	zip -r files.zip \
		main.go tools.go mcp_tools.go \
		datalog_test.go datalog-tests.lisp breadco.lisp \
		README-MCP.md ARCHITECTURE.md \
		go.mod prompts/ Makefile
