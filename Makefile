BINARY := jsonlv
INSTALL_DIR := /usr/local/bin

.PHONY: build install

build:
	go build -o $(BINARY) .

install: build
	install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
