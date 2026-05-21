APP_NAME = envycontrol
SRC = $(wildcard *.go)

.PHONY: all build clean install uninstall

all: build

build: $(SRC)
	@echo "Building $(APP_NAME)..."
	@go build -ldflags="-s -w" -o $(APP_NAME) .

clean:
	@echo "Cleaning up..."
	@rm -f $(APP_NAME)

install: build
	@echo "Installing to /usr/local/bin/..."
	@install -m 755 $(APP_NAME) /usr/local/bin/

uninstall:
	@echo "Removing from /usr/local/bin/..."
	@rm -f /usr/local/bin/$(APP_NAME)
