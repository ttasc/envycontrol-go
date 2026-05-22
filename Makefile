APP_NAME = envycontrol

PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin

.PHONY: all build clean install uninstall

all: build

build:
	@echo "Building static and lightweight binary '$(APP_NAME)'..."
	@CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -extldflags '-static'" -o $(APP_NAME) .
	@echo "Build successful! Run 'ls -lh $(APP_NAME)' to see the file size."

clean:
	@echo "Cleaning up..."
	@rm -f $(APP_NAME)

install: build
	@echo "Installing $(APP_NAME) to $(BINDIR)..."
	@install -d $(BINDIR)
	@install -m 755 $(APP_NAME) $(BINDIR)/
	@echo "Install completed!"

uninstall:
	@echo "Removing $(APP_NAME) from $(BINDIR)..."
	@rm -f $(BINDIR)/$(APP_NAME)
	@echo "Uninstall completed!"
