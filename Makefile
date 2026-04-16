BINARY := req
INSTALL_DIR := /usr/local/bin

.PHONY: build install test clean

build:
	go build -o $(BINARY) .

install: build
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

test:
	go test ./... -v

clean:
	rm -f $(BINARY)
