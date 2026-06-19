BINARY     := kon
CMD        := ./cmd
DIST       := dist
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install clean dist linux arm

## Build for current OS/arch
build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

## Install to /usr/local/bin
install: build
	@sudo mv $(BINARY) /usr/local/bin/$(BINARY)
	@echo "[+] installed → /usr/local/bin/kon"

## Build linux amd64
linux:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/kon-linux-amd64 $(CMD)
	@echo "[+] $(DIST)/kon-linux-amd64"

## Build linux arm64
arm:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/kon-linux-arm64 $(CMD)
	@echo "[+] $(DIST)/kon-linux-arm64"

## Build all targets
dist: linux arm

## Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
