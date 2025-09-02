#!/bin/bash

# Doctrus Release Build Script
# Builds binaries for multiple platforms and architectures

set -e

# Configuration
PROJECT_NAME="doctrus"
VERSION="${VERSION:-v0.1.0}"
OUTPUT_DIR="${OUTPUT_DIR:-dist}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Build function
build_binary() {
    local os=$1
    local arch=$2
    local output_name="${PROJECT_NAME}-${os}-${arch}"
    
    if [ "$os" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    local output_path="${OUTPUT_DIR}/${output_name}"
    
    log_info "Building for ${os}/${arch}..."
    
    # Set GOOS and GOARCH
    export GOOS=$os
    export GOARCH=$arch
    
    # Build the binary
    if go build -ldflags "-X main.version=${VERSION} -s -w" -o "$output_path" .; then
        log_success "Built ${output_name}"
        
        # Calculate and display file size
        local size=$(du -h "$output_path" | cut -f1)
        log_info "Size: ${size}"
        
        # Generate SHA256 checksum
        local checksum_file="${output_path}.sha256"
        sha256sum "$output_path" | cut -d' ' -f1 > "$checksum_file"
        log_info "Checksum: $(cat $checksum_file)"
        
        echo ""
    else
        log_error "Failed to build for ${os}/${arch}"
        return 1
    fi
}

# Main build process
main() {
    log_info "Starting Doctrus release build for ${VERSION}"
    echo ""
    
    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    
    # Define target platforms
    # Format: "OS ARCH"
    platforms=(
        "linux amd64"
        "linux arm64"
        "darwin amd64"
        "darwin arm64"
        "windows amd64"
        "windows arm"
    )
    
    log_info "Building for ${#platforms[@]} platforms..."
    echo ""
    
    # Build for each platform
    for platform in "${platforms[@]}"; do
        os=$(echo $platform | cut -d' ' -f1)
        arch=$(echo $platform | cut -d' ' -f2)
        build_binary "$os" "$arch"
    done
    
    log_success "Build complete!"
    log_info "Binaries are in: ${OUTPUT_DIR}/"
    echo ""
    
    log_info "File listing:"
    ls -lh "${OUTPUT_DIR}/"
    echo ""
    
    log_info "To create a GitHub release with these binaries:"
    echo "  gh release create ${VERSION} ${OUTPUT_DIR}/* --title \"Doctrus ${VERSION}\" --notes-file release_notes.md --prerelease"
    echo ""
    
    log_info "Or upload manually to: https://github.com/SebastiaanWouters/doctrus/releases/new"
}

# Run main function
main "$@"
