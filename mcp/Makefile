BINARY_NAME=prism-mcp
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
PLATFORMS=darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

.PHONY: build clean release

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

release: clean
	@mkdir -p bin
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output="bin/$(BINARY_NAME)-$${os}-$${arch}"; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $$output . || exit 1; \
	done
	@echo "Done. Binaries in bin/"

clean:
	find bin -type f -name '$(BINARY_NAME)-*' -delete 2>/dev/null || true
