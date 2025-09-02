# 🚀 Doctrus v0.1.0 (Beta) - First Release!

**Monorepo Task Runner with Docker Integration**

## ✨ Features

### Core Functionality
- 🏗️ **Monorepo Support**: Manage multiple workspaces from a single configuration
- 🐳 **Docker Integration**: Run tasks inside Docker containers via Docker Compose
- 🔄 **Dependency Resolution**: Automatic dependency resolution with diamond dependency support
- 📦 **Smart Caching**: Skip tasks when inputs haven't changed using SHA256 hashing
- ⚡ **Performance**: Built in Go for speed and reliability

### Configuration & Flexibility
- 📝 **YAML Configuration**: Simple, readable configuration files
- 🔗 **Cross-workspace Dependencies**: Tasks can depend on tasks in other workspaces
- 🎯 **Flexible Tool Support**: Works with npm, composer, Go, and any command-line tools
- 🌍 **Environment Variables**: Support for workspace and task-specific environment variables

### Developer Experience
- 📊 **Verbose Output**: Detailed execution information with `--verbose`
- 🔍 **Dry Run Mode**: Preview execution plan with `--dry-run`
- ⚡ **Force Rebuild**: Override cache with `--force`
- 📋 **Task Listing**: List all available workspaces and tasks
- ✅ **Configuration Validation**: Validate your doctrus.yml setup

## 📖 Usage Examples

### Basic Configuration
```yaml
version: "1.0"

workspaces:
  frontend:
    path: ./frontend
    container: frontend
    tasks:
      install:
        command: ["npm", "install"]
        inputs: ["package.json", "package-lock.json"]
        outputs: ["node_modules/**/*"]
        cache: true
      
      build:
        command: ["npm", "run", "build"]
        depends_on: ["install"]
        inputs: ["src/**/*", "public/**/*"]
        outputs: ["dist/**/*"]
        cache: true

  backend:
    path: ./backend
    container: backend
    tasks:
      compile:
        command: ["go", "build"]
        inputs: ["**/*.go", "go.mod"]
        outputs: ["bin/app"]
        cache: true
```

### Command Examples
```bash
# Install Doctrus
curl -sSL https://raw.githubusercontent.com/SebastiaanWouters/doctrus/main/install.sh | bash

# Initialize project
doctrus init

# List workspaces and tasks
doctrus list

# Run specific task
doctrus run frontend:build

# Run task in all workspaces
doctrus run test

# Force rebuild (ignore cache)
doctrus run --force build

# Preview execution plan
doctrus run --dry-run deploy
```

## 🔧 Technical Features

### Intelligent Caching
- SHA256-based change detection for input files
- Automatic cache invalidation when dependencies change
- Support for complex glob patterns (`**/*`, `{a,b}/**/*.go`)
- Cache storage in project root (`.doctrus/cache`)

### Dependency Resolution
- Topological sorting using Kahn's algorithm
- Diamond dependency support (shared dependencies execute once)
- Circular dependency detection with clear error messages
- Cross-workspace dependency chains

### Docker Integration
- Automatic container status checking
- Clear error messages when containers aren't running
- Environment variable passing to containers
- Support for existing docker-compose.yml configurations

## 📋 Requirements

- Go 1.21+ (for building from source)
- Docker & Docker Compose (for containerized tasks)
- Linux/macOS/Windows (cross-platform support)

## 🐛 Known Issues

This is a beta release. Please report any issues on GitHub.

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines and submit pull requests.

## 📄 License

MIT License - see LICENSE file for details.

---

**Installation**: Download the binary for your platform from the releases section, or build from source with `go build .`

**Documentation**: Full documentation available at https://github.com/SebastiaanWouters/doctrus
