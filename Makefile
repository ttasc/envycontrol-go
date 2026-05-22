APP_NAME = envycontrol

PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin

# Build variables for Go
GOOS ?= linux
ARCHS ?= amd64 arm64 386
DIST_DIR = dist
LDFLAGS = -s -w -extldflags '-static'

.PHONY: all build build-all clean install uninstall

all: build

build:
	@echo "Building static and lightweight binary '$(APP_NAME)' for native architecture..."
	@CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(APP_NAME) .
	@echo "Build successful!"

build-all:
	@echo "Building for multiple architectures: $(ARCHS)..."
	@mkdir -p $(DIST_DIR)
	@for arch in $(ARCHS); do \
		echo "  -> Building for linux/$$arch..."; \
		GOOS=$(GOOS) GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME)-$(GOOS)-$$arch .; \
	done
	@echo "All builds completed in $(DIST_DIR)/"

clean:
	@echo "Cleaning up..."
	@rm -f $(APP_NAME)
	@rm -rf $(DIST_DIR)

install: build
	@echo "Installing $(APP_NAME) to $(BINDIR)..."
	@install -d $(BINDIR)
	@install -m 755 $(APP_NAME) $(BINDIR)/
	@echo "Install completed!"

uninstall:
	@echo "Step 1: Reverting system configurations and rebuilding initramfs..."
	@if [ -f $(BINDIR)/$(APP_NAME) ]; then \
		$(BINDIR)/$(APP_NAME) --reset; \
	else \
		echo "Warning: $(APP_NAME) not found in $(BINDIR). Skipping system reset."; \
	fi
	@echo "Step 2: Removing application binary and leftover data..."
	@rm -f $(BINDIR)/$(APP_NAME)
	@rm -rf /var/lib/envycontrol
	@echo "Uninstall completed successfully!"
