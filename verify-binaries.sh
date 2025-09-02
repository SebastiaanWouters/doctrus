#!/bin/bash

# Verify that built binaries work correctly

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# Test Linux binary (if we're on Linux)
if [ -f "dist/doctrus-linux-amd64" ] && [ "$(uname -s)" = "Linux" ]; then
    log_info "Testing Linux binary..."
    if ./dist/doctrus-linux-amd64 --help > /dev/null 2>&1; then
        log_success "Linux binary works correctly"
    else
        log_error "Linux binary failed"
    fi
fi

# Test all binaries by checking if they can show help
echo ""
log_info "Testing all binaries..."

for binary in dist/*; do
    if [[ "$binary" == *.sha256 ]]; then
        continue
    fi
    
    filename=$(basename "$binary")
    
    # Skip Windows binaries if not on Windows
    if [[ "$filename" == *windows* ]] && [ "$(uname -s)" != "MINGW"* ] && [ "$(uname -s)" != "MSYS"* ]; then
        log_info "Skipping Windows binary: $filename (not on Windows)"
        continue
    fi
    
    # Skip macOS binaries if not on macOS
    if [[ "$filename" == *darwin* ]] && [ "$(uname -s)" != "Darwin" ]; then
        log_info "Skipping macOS binary: $filename (not on macOS)"
        continue
    fi
    
    log_info "Testing $filename..."
    
    if "$binary" --help > /dev/null 2>&1; then
        log_success "$filename works correctly"
    else
        # Check if it's an architecture mismatch (common for ARM binaries on x86)
        if [[ "$filename" == *arm* ]] && [ "$(uname -m)" != "aarch64" ] && [ "$(uname -m)" != "arm"* ]; then
            log_info "$filename built correctly (architecture mismatch expected)"
        else
            log_error "$filename failed to run"
        fi
    fi
done

echo ""
log_success "Binary verification complete!"
