# Doctrus Binary Releases

This document explains how to download and use pre-built Doctrus binaries.

## 📦 Available Downloads

Doctrus binaries are available for the following platforms:

| Platform | Architecture | Filename | Size |
|----------|-------------|----------|------|
| Linux | x86_64 | `doctrus-linux-amd64` | ~3.4MB |
| Linux | ARM64 | `doctrus-linux-arm64` | ~3.4MB |
| macOS | x86_64 | `doctrus-darwin-amd64` | ~3.4MB |
| macOS | ARM64 (Apple Silicon) | `doctrus-darwin-arm64` | ~3.3MB |
| Windows | x86_64 | `doctrus-windows-amd64.exe` | ~3.6MB |
| Windows | ARM | `doctrus-windows-arm.exe` | ~3.4MB |

Each binary includes:
- ✅ SHA256 checksum file for verification
- ✅ Statically linked (no external dependencies)
- ✅ Optimized binary size with `-s -w` flags

## 🚀 Quick Start

### 1. Download the appropriate binary for your platform

**Linux (x86_64):**
```bash
wget https://github.com/SebastiaanWouters/doctrus/releases/download/v0.1.0/doctrus-linux-amd64
chmod +x doctrus-linux-amd64
sudo mv doctrus-linux-amd64 /usr/local/bin/doctrus
```

**macOS (Intel):**
```bash
curl -L https://github.com/SebastiaanWouters/doctrus/releases/download/v0.1.0/doctrus-darwin-amd64 -o doctrus
chmod +x doctrus
sudo mv doctrus /usr/local/bin/
```

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/SebastiaanWouters/doctrus/releases/download/v0.1.0/doctrus-darwin-arm64 -o doctrus
chmod +x doctrus
sudo mv doctrus /usr/local/bin/
```

**Windows:**
Download `doctrus-windows-amd64.exe` and add it to your PATH.

### 2. Verify the download (optional but recommended)

```bash
# Download checksum
wget https://github.com/SebastiaanWouters/doctrus/releases/download/v0.1.0/doctrus-linux-amd64.sha256

# Verify
sha256sum -c doctrus-linux-amd64.sha256
```

### 3. Test the installation

```bash
doctrus --help
doctrus --version
```

## 📋 System Requirements

- **Linux**: Kernel 2.6.32+ (most modern distributions)
- **macOS**: 10.12+ (Sierra or later)
- **Windows**: Windows 7+ (64-bit recommended)
- **Memory**: ~10MB RAM minimum
- **Disk**: ~4MB for binary only

## 🛠️ Building from Source (Alternative)

If you prefer to build from source:

```bash
# Clone the repository
git clone https://github.com/SebastiaanWouters/doctrus.git
cd doctrus

# Build for your platform
go build -o doctrus .

# Or use the build script for all platforms
./build-release.sh
```

## 🔒 Security

- Binaries are built with Go's security features enabled
- SHA256 checksums provided for integrity verification
- No external dependencies required
- Open source code available for audit

## 🐛 Troubleshooting

### Binary won't run
```bash
# Check if binary is executable
ls -la doctrus

# Make executable if needed
chmod +x doctrus

# Check system compatibility
uname -a
```

### Permission denied
```bash
# Install to user directory instead
mkdir -p ~/bin
mv doctrus ~/bin/
export PATH="$HOME/bin:$PATH"
```

### Architecture mismatch
- Ensure you downloaded the correct binary for your CPU architecture
- Use `uname -m` to check your system's architecture

## 📞 Support

- **Issues**: https://github.com/SebastiaanWouters/doctrus/issues
- **Discussions**: https://github.com/SebastiaanWouters/doctrus/discussions
- **Documentation**: https://github.com/SebastiaanWouters/doctrus#readme

## 📈 Performance

- **Startup time**: <100ms
- **Memory usage**: ~5-15MB depending on project size
- **CPU usage**: Minimal (mostly I/O bound for caching operations)

---

**Happy coding with Doctrus! 🎉**
