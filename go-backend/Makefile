.PHONY: build test lint clean cross-compile run setup-fonts

BINARY_NAME=training-eval-system
BUILD_DIR=./bin
MAIN_PKG=./cmd/server

# Default build (native)
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)

# Cross-compile for LoongArch (龙芯)
cross-compile:
	CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-loong64 $(MAIN_PKG)

# Run locally
run:
	go run $(MAIN_PKG)

# Run all tests
test:
	go test ./... -count=1 -race

# Run tests with coverage
test-cover:
	go test ./... -count=1 -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Lint with go vet + staticcheck
lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

# Format code
fmt:
	gofmt -s -w .

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

# Verify no CGO dependencies
verify-static:
	CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-verify $(MAIN_PKG)
	file $(BUILD_DIR)/$(BINARY_NAME)-verify
	@echo "Build successful - static binary verified"

# Download Chinese font for PDF rendering (replaces empty fonts/ directory)
setup-fonts:
	@mkdir -p fonts
	@echo "Downloading Noto Sans SC Chinese font..."
	@if command -v curl &> /dev/null; then \
		curl -sL -o fonts/NotoSansSC-Regular.ttf "https://github.com/googlefonts/noto-cjk/raw/main/Sans/OTF/SimplifiedChinese/NotoSansSC-Regular.otf" && \
		echo "Font downloaded to fonts/NotoSansSC-Regular.otf"; \
	elif command -v wget &> /dev/null; then \
		wget -q -O fonts/NotoSansSC-Regular.otf "https://github.com/googlefonts/noto-cjk/raw/main/Sans/OTF/SimplifiedChinese/NotoSansSC-Regular.otf" && \
		echo "Font downloaded to fonts/NotoSansSC-Regular.otf"; \
	else \
		echo "ERROR: Need curl or wget to download font"; \
		exit 1; \
	fi
	@echo "Done. Set TES_PDF_FONT_PATH=./fonts/NotoSansSC-Regular.otf or run with default detection"
