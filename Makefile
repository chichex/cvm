VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/chichex/cvm/cmd.Version=$(VERSION)"

.PHONY: build install test clean

build:
	go build $(LDFLAGS) -o bin/cvm .
	go build -o bin/cvm-mcp-kb ./cmd/mcp-kb/

install: build
	cp bin/cvm /usr/local/bin/cvm
	cp bin/cvm-mcp-kb /usr/local/bin/cvm-mcp-kb

uninstall:
	rm -f /usr/local/bin/cvm /usr/local/bin/cvm-mcp-kb

test:
	go test ./...

clean:
	rm -rf bin/

release:
	goreleaser release --clean
