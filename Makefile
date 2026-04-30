BINARY   := sms2mqtt
REMOTE   ?= user@your-linux-box
DEST     := /usr/local/bin/$(BINARY)
ARCH     ?= arm64   # amd64 | arm64 (Pi 4 64-bit) | arm (Pi 4 32-bit)
GOARM    ?= 7       # only used when ARCH=arm

build:
	GOOS=linux GOARCH=$(ARCH) GOARM=$(GOARM) go build -o $(BINARY) .

build-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) .

build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BINARY) .

build-arm:
	GOOS=linux GOARCH=arm GOARM=$(GOARM) go build -o $(BINARY) .

deploy: build
	scp $(BINARY) $(REMOTE):$(DEST)
	rm $(BINARY)

logs:
	ssh $(REMOTE) journalctl -fu $(BINARY)

start:
	ssh $(REMOTE) systemctl start $(BINARY)

stop:
	ssh $(REMOTE) systemctl stop $(BINARY)

restart:
	ssh $(REMOTE) systemctl restart $(BINARY)

status:
	ssh $(REMOTE) systemctl status $(BINARY)

.PHONY: build build-amd64 build-arm64 build-arm deploy logs start stop restart status
