VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/ayrtonmarini/cvm/cmd.Version=$(VERSION)"

.PHONY: build install test clean

build:
	go build $(LDFLAGS) -o bin/cvm .

install: build
	cp bin/cvm /usr/local/bin/cvm

uninstall:
	rm -f /usr/local/bin/cvm

test:
	go test ./...

clean:
	rm -rf bin/

release:
	goreleaser release --clean
