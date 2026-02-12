# envgraph - Environment Variable Analyzer & Graph Tool
# Makefile for building and distributing
# Builds for: Linux x64, Windows x64, macOS Silicon, macOS Intel

BINARY_NAME=envgraph
VERSION?=1.0.0
BUILD_DIR=dist
LDFLAGS=-ldflags "-s -w -X github.com/stackgen-cli/envgraph/cmd.version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Platforms: Linux x64, Windows x64, macOS Silicon (arm64), macOS Intel (amd64)
PLATFORMS=linux/amd64 windows/amd64 darwin/arm64 darwin/amd64

.PHONY: all build clean test deps release package help install uninstall

all: clean deps test build

## Build for current platform
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

## Build for all platforms
build-all: clean
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		output=$(BUILD_DIR)/$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then output=$$output.exe; fi; \
		echo "Building $$output..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $$output .; \
	done

## Run tests
test:
	$(GOTEST) -v ./...

## Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

## Create release packages with checksums
release: build-all
	@echo "Creating release packages..."
	@cd $(BUILD_DIR) && \
	for file in *; do \
		if [ -f "$$file" ]; then \
			platform=$$(echo $$file | sed 's/$(BINARY_NAME)-//' | sed 's/\.exe//'); \
			mkdir -p $(BINARY_NAME)-$(VERSION)-$$platform; \
			cp $$file $(BINARY_NAME)-$(VERSION)-$$platform/; \
			if [ "$${platform##*-}" != "windows" ]; then \
				cp $$file $(BINARY_NAME)-$(VERSION)-$$platform/$(BINARY_NAME); \
			fi; \
			cp ../README.md ../LICENSE ../DISCLAIMER.md $(BINARY_NAME)-$(VERSION)-$$platform/ 2>/dev/null || true; \
			zip -r $(BINARY_NAME)-$(VERSION)-$$platform.zip $(BINARY_NAME)-$(VERSION)-$$platform; \
			rm -rf $(BINARY_NAME)-$(VERSION)-$$platform; \
			rm $$file; \
		fi; \
	done
	@echo "Generating checksums..."
	@cd $(BUILD_DIR) && shasum -a 256 *.zip > checksums.txt
	@echo "Release packages created in $(BUILD_DIR)/"

## Install locally
install: build
	mv $(BINARY_NAME) /usr/local/bin/

## Uninstall
uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

## Show help
help:
	@echo "envgraph Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build for current platform"
	@echo "  build-all     Build for all platforms (Linux, Windows, macOS)"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  deps          Download and tidy dependencies"
	@echo "  clean         Remove build artifacts"
	@echo "  release       Create release packages with checksums"
	@echo "  install       Install to /usr/local/bin"
	@echo "  uninstall     Remove from /usr/local/bin"
	@echo "  help          Show this help message"
