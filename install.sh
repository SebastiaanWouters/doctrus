#!/bin/bash

set -e

# Doctrus Installation Script
# This script downloads and installs the latest version of doctrus

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TEMP_DIR="$(mktemp -d)"
BINARY_NAME="doctrus"
GITHUB_REPO="SebastiaanWouters/doctrus"
VERSION="${VERSION:-latest}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

cleanup() {
    rm -rf "$TEMP_DIR"
}

trap cleanup EXIT

detect_os_arch() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"
    
    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l)
            ARCH="arm"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac
    
    case "$OS" in
        linux)
            PLATFORM="linux"
            ;;
        darwin)
            PLATFORM="darwin"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac
    
    info "Detected platform: ${PLATFORM}-${ARCH}"
}

check_dependencies() {
    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        error "curl or wget is required for installation"
    fi
    
    if ! command -v tar >/dev/null 2>&1; then
        error "tar is required for installation"
    fi
}

get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        info "Fetching latest version..."
        if command -v curl >/dev/null 2>&1; then
            VERSION=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        elif command -v wget >/dev/null 2>&1; then
            VERSION=$(wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        fi
    fi
    
    if [ -z "$VERSION" ]; then
        warn "Could not determine latest version, using development build"
        VERSION="main"
    fi
    
    info "Installing version: $VERSION"
}

download_binary() {
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY_NAME}-${PLATFORM}-${ARCH}"
    if [ "$VERSION" = "main" ]; then
        # For development, we'll build from source
        build_from_source
        return
    fi
    
    info "Downloading from: $DOWNLOAD_URL"
    
    cd "$TEMP_DIR"
    if command -v curl >/dev/null 2>&1; then
        if ! curl -L -o "$BINARY_NAME" "$DOWNLOAD_URL"; then
            error "Failed to download binary"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -O "$BINARY_NAME" "$DOWNLOAD_URL"; then
            error "Failed to download binary"
        fi
    fi
    
    chmod +x "$BINARY_NAME"
}

build_from_source() {
    info "Building from source..."
    
    if ! command -v go >/dev/null 2>&1; then
        error "Go is required to build from source. Please install Go or use a pre-built binary."
    fi
    
    cd "$TEMP_DIR"
    
    git_clone_failed=false
    if command -v git >/dev/null 2>&1; then
        git clone "git@github.com:${GITHUB_REPO}.git" . 2>/dev/null || {
            warn "Git clone failed, falling back to zip download"
            git_clone_failed=true
        }
    else
        git_clone_failed=true
    fi
    
    if [ "$git_clone_failed" = "true" ]; then
        if command -v curl >/dev/null 2>&1; then
            curl -L "https://github.com/${GITHUB_REPO}/archive/main.zip" -o source.zip
        elif command -v wget >/dev/null 2>&1; then
            wget "https://github.com/${GITHUB_REPO}/archive/main.zip" -O source.zip
        fi
        unzip source.zip
        cd "${GITHUB_REPO##*/}-main" || cd "doctrus-main"
    fi
    
    info "Compiling..."
    go build -o "$BINARY_NAME" .
    
    if [ ! -f "$BINARY_NAME" ]; then
        error "Build failed - binary not found"
    fi
}

install_binary() {
    info "Installing to $INSTALL_DIR"
    
    if [ ! -w "$INSTALL_DIR" ]; then
        info "Installing with sudo (requires admin privileges)"
        sudo cp "$TEMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        cp "$TEMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi
}

verify_installation() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        VERSION_OUTPUT=$("$BINARY_NAME" --help 2>&1 | head -1 || true)
        info "✅ Installation successful!"
        info "Installed: $VERSION_OUTPUT"
        info ""
        info "Get started:"
        info "  1. cd your-monorepo-project"
        info "  2. doctrus init"
        info "  3. Edit doctrus.yml to match your project"
        info "  4. doctrus validate"
        info "  5. doctrus list"
        info "  6. doctrus run <task>"
        info ""
        info "For more information, visit: https://github.com/${GITHUB_REPO}"
    else
        error "Installation verification failed. Binary not found in PATH."
    fi
}

main() {
    info "🚀 Installing Doctrus - Monorepo Task Runner"
    info ""
    
    detect_os_arch
    check_dependencies
    get_latest_version
    download_binary
    install_binary
    verify_installation
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --help|-h)
            echo "Doctrus Installation Script"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --install-dir DIR    Install directory (default: /usr/local/bin)"
            echo "  --version VERSION    Version to install (default: latest)"
            echo "  --help, -h          Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  INSTALL_DIR         Install directory"
            echo "  VERSION             Version to install"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Install latest version"
            echo "  $0 --install-dir ~/.local/bin        # Install to user directory"
            echo "  $0 --version v1.0.0                  # Install specific version"
            echo ""
            exit 0
            ;;
        *)
            error "Unknown option: $1. Use --help for usage information."
            ;;
    esac
done

main