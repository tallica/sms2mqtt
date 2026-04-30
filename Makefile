BINARY   := sms2mqtt
REMOTE   ?= user@your-linux-box
DEST     := /usr/local/bin/$(BINARY)

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) .

deploy: build-linux
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

.PHONY: build-linux deploy logs start stop restart status
