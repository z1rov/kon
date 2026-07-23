# Author: z1rov
BINARY     := z1
CMD        := ./cmd
DIST       := dist
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install clean dist linux arm

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

install: build
	@sudo mv $(BINARY) /usr/local/bin/$(BINARY)
	@echo "[+] installed -> /usr/local/bin/z1"

linux:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/z1-linux-amd64 $(CMD)
	@echo "[+] $(DIST)/z1-linux-amd64"

arm:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/z1-linux-arm64 $(CMD)
	@echo "[+] $(DIST)/z1-linux-arm64"

dist: linux arm

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
