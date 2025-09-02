#!/bin/bash

# Create GitHub release with binaries
# Usage: ./create-release.sh [version]

VERSION="${1:-v0.1.0}"

echo "Creating GitHub release ${VERSION} with binaries..."
echo ""

# Check if binaries exist
if [ ! -d "dist" ]; then
    echo "Error: dist/ directory not found. Run ./build-release.sh first."
    exit 1
fi

# Create release with GitHub CLI
gh release create "${VERSION}" \
    --title "Doctrus ${VERSION} (Beta)" \
    --notes-file release_notes.md \
    --prerelease \
    dist/*

echo ""
echo "✅ Release ${VERSION} created successfully!"
echo "📦 Binaries uploaded:"
ls -lh dist/*.exe dist/*amd64* dist/*arm64* 2>/dev/null || ls -lh dist/*
