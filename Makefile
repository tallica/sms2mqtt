BINARY   := sms2mqtt
REMOTE   ?= user@your-linux-box
DEST     := /usr/local/bin/$(BINARY)
ARCH     ?= arm64   # amd64 | arm64 (Pi 4 64-bit) | arm (Pi 4 32-bit)
GOARM    ?= 7       # only used when ARCH=arm
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -ldflags "-X main.version=$(VERSION)"

build:
	GOOS=linux GOARCH=$(ARCH) GOARM=$(GOARM) go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

build-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY) .

build-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY) .

build-arm:
	GOOS=linux GOARCH=arm GOARM=$(GOARM) go build $(LDFLAGS) -o $(BINARY) .

deploy: build
	ssh $(REMOTE) sudo systemctl stop $(BINARY)
	scp $(BINARY) $(REMOTE):$(DEST)
	rm $(BINARY)

logs:
	ssh $(REMOTE) journalctl -fu $(BINARY)

start:
	ssh $(REMOTE) sudo systemctl start $(BINARY)

stop:
	ssh $(REMOTE) sudo systemctl stop $(BINARY)

restart:
	ssh $(REMOTE) sudo systemctl restart $(BINARY)

status:
	ssh $(REMOTE) sudo systemctl status $(BINARY)

.PHONY: build test build-amd64 build-arm64 build-arm deploy logs start stop restart status
