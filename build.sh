#!/bin/bash

# Build script for Burnmail (Linux/macOS only)
# Usage: ./build.sh [platform]

set -e

BINARY_NAME="burnmail"
VERSION="1.4.1"

echo "🔨 Building Burnmail v${VERSION}"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Download dependencies
echo -e "${BLUE}📦 Downloading dependencies...${NC}"
go mod download
go mod tidy

# Build based on platform argument
PLATFORM=${1:-current}

case $PLATFORM in
  "current")
    echo -e "${BLUE}🏗️  Building for current platform...${NC}"
    go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}
    echo -e "${GREEN}✓ Built: ${BINARY_NAME}${NC}"
    ;;

  "linux")
    echo -e "${BLUE}🐧 Building for Linux...${NC}"
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-linux-amd64
    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-linux-arm64
    echo -e "${GREEN}✓ Built: ${BINARY_NAME}-linux-amd64${NC}"
    echo -e "${GREEN}✓ Built: ${BINARY_NAME}-linux-arm64${NC}"
    ;;

  "macos")
    echo -e "${BLUE}🍎 Building for macOS...${NC}"
    GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-macos-amd64
    GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-macos-arm64
    echo -e "${GREEN}✓ Built: ${BINARY_NAME}-macos-amd64${NC}"
    echo -e "${GREEN}✓ Built: ${BINARY_NAME}-macos-arm64${NC}"
    ;;

  "all")
    echo -e "${BLUE}🌍 Building for Linux and macOS...${NC}"

    # Linux
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-linux-amd64
    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-linux-arm64

    # macOS
    GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-macos-amd64
    GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X burnmail/internal/config.Version=${VERSION}" -o ${BINARY_NAME}-macos-arm64

    echo -e "${GREEN}✓ Built all binaries${NC}"
    ;;

  *)
    echo "Unknown platform: $PLATFORM"
    echo "Usage: ./build.sh [current|linux|macos|all]"
    echo "Note: For Windows, use build.ps1 instead"
    exit 1
    ;;
esac

echo ""
echo -e "${GREEN}🎉 Build complete!${NC}"
echo ""
echo "To install globally:"
echo "  sudo mv ${BINARY_NAME} /usr/local/bin/"
echo ""
echo "To test:"
echo "  ./${BINARY_NAME} g"
echo "  ./${BINARY_NAME} m"
echo "  ./${BINARY_NAME} me"
echo "  ./${BINARY_NAME} d"