# Doctrus

A powerful monorepo task runner with Docker Compose integration, intelligent caching, and dependency tracking.

## Features

- 🏗️ **Monorepo Support**: Manage multiple workspaces from a single configuration
- 🐳 **Docker Integration**: Run tasks inside Docker containers via Docker Compose
- 📦 **Smart Caching**: Skip tasks when inputs haven't changed
- 🔄 **Dependency Tracking**: Automatic dependency resolution and execution ordering  
- ⚡ **Performance**: Built in Go for speed and reliability
- 🎯 **Flexible**: Works with npm, composer, and any command-line tools

## Quick Start

### Installation

**Linux/WSL:**
```bash
curl -sSL https://raw.githubusercontent.com/SebastiaanWouters/doctrus/main/install.sh | bash
```

**Manual Installation:**
```bash
# Download the binary for your platform
wget https://github.com/SebastiaanWouters/doctrus/releases/latest/download/doctrus-linux-amd64
chmod +x doctrus-linux-amd64
sudo mv doctrus-linux-amd64 /usr/local/bin/doctrus
```

### Initialize a Project

```bash
cd your-monorepo
doctrus init
```

This creates a sample `doctrus.yml` configuration file.

### Basic Usage

```bash
# List all workspaces and tasks
doctrus list

# Run a specific task
doctrus run frontend:build

# Run task in any workspace (if unique)
doctrus run test

# Validate configuration
doctrus validate

# Force rebuild (ignore cache)
doctrus run --force backend:build

# Show what would run without executing
doctrus run --dry-run frontend:build
```

## Configuration

Create a `doctrus.yml` file in your project root:

```yaml
version: "1.0"

workspaces:
  frontend:
    path: ./frontend
    container: frontend  # Optional: Docker container name
    env:
      NODE_ENV: development
    tasks:
      install:
        command: ["npm", "install"]
        description: "Install dependencies"
        inputs: ["package.json", "package-lock.json"]
        outputs: ["node_modules/**/*"]
        cache: true
      
      build:
        command: ["npm", "run", "build"]
        description: "Build application"
        depends_on: ["install"]
        inputs: ["src/**/*", "public/**/*"]
        outputs: ["dist/**/*"]
        cache: true
      
      test:
        command: ["npm", "test"]
        depends_on: ["install"]
        inputs: ["src/**/*", "test/**/*"]

  backend:
    path: ./backend
    container: backend
    tasks:
      install:
        command: ["composer", "install"]
        inputs: ["composer.json", "composer.lock"]
        outputs: ["vendor/**/*"]
        cache: true
      
      test:
        command: ["./vendor/bin/phpunit"]
        depends_on: ["install"]
        
      deploy:
        command: ["php", "artisan", "deploy"]
        depends_on: ["install", "frontend:build"]

# Optional Docker configuration
docker:
  compose_file: docker-compose.yml
```

## Configuration Reference

### Workspace Configuration

- **path**: Directory path (relative or absolute)
- **container**: Docker container name from docker-compose.yml
- **env**: Environment variables for all tasks in workspace
- **tasks**: Map of task definitions

### Task Configuration

- **command**: Command to execute (array of strings)
- **description**: Human-readable description
- **depends_on**: Array of task dependencies
  - `"task"` - task in same workspace
  - `"workspace:task"` - task in different workspace
- **inputs**: File patterns to watch for changes (supports advanced globs including `**/*`)
- **outputs**: File patterns produced by task (supports advanced globs including `**/*`)
- **cache**: Enable/disable caching (default: false)
- **env**: Task-specific environment variables

### Docker Configuration

- **compose_file**: Path to docker-compose.yml

## Examples

### Frontend + Backend Monorepo

```yaml
version: "1.0"

workspaces:
  web:
    path: ./packages/web
    container: web
    tasks:
      deps:
        command: ["npm", "ci"]
        inputs: ["package*.json"]
        outputs: ["node_modules"]
        cache: true
      
      build:
        command: ["npm", "run", "build"]
        depends_on: ["deps"]
        inputs: ["src/**/*", "public/**/*"]
        outputs: ["dist/**/*"]
        cache: true
      
      test:
        command: ["npm", "test"]
        depends_on: ["deps"]

  api:
    path: ./packages/api
    container: api
    tasks:
      deps:
        command: ["go", "mod", "download"]
        inputs: ["go.mod", "go.sum"]
        cache: true
      
      build:
        command: ["go", "build", "-o", "bin/api", "."]
        depends_on: ["deps"]
        inputs: ["**/*.go"]
        outputs: ["bin/api"]
        cache: true
      
      test:
        command: ["go", "test", "./..."]
        depends_on: ["deps"]

docker:
  compose_file: docker-compose.dev.yml
```

### PHP + JavaScript Project

```yaml
version: "1.0"

workspaces:
  app:
    path: .
    container: php
    env:
      APP_ENV: development
    tasks:
      composer:
        command: ["composer", "install"]
        inputs: ["composer.json", "composer.lock"]
        outputs: ["vendor/**/*"]
        cache: true
        
      assets:
        command: ["npm", "run", "dev"]
        inputs: ["resources/**/*", "webpack.mix.js"]
        outputs: ["public/js/**/*", "public/css/**/*"]
        cache: true
        
      migrate:
        command: ["php", "artisan", "migrate"]
        depends_on: ["composer"]
        
      test:
        command: ["./vendor/bin/phpunit"]
        depends_on: ["composer", "assets"]

docker:
  compose_file: docker-compose.yml
```

## Command Reference

### `doctrus run [workspace:]task`

Run tasks with dependency resolution.

**Options:**
- `--force, -f`: Force rebuild (ignore cache)
- `--skip-cache`: Skip cache completely
- `--parallel, -p N`: Run N tasks in parallel
- `--show-diff`: Show changed files since last run
- `--dry-run`: Show execution plan without running

**Examples:**
```bash
doctrus run build                    # Run 'build' in any workspace
doctrus run frontend:build          # Run specific workspace task
doctrus run test --parallel 3       # Run with parallelism
doctrus run deploy --force          # Force rebuild
```

### `doctrus list [workspace]`

List workspaces and tasks.

```bash
doctrus list                # List all workspaces
doctrus list frontend       # List tasks in workspace
doctrus list -v             # Verbose output with details
```

### `doctrus cache`

Manage task cache.

```bash
doctrus cache clear         # Clear all cache
doctrus cache clear web     # Clear workspace cache
doctrus cache stats         # Show cache statistics
doctrus cache list          # List cached tasks
```

### `doctrus validate`

Validate configuration and environment.

```bash
doctrus validate           # Validate config and setup
doctrus validate -v        # Verbose validation output
```

## Docker Integration

Doctrus integrates with Docker Compose to run tasks in containers:

1. **Container Tasks**: Set `container` in workspace config
2. **File Mounting**: Workspace paths are mounted as `/workspace/{path}`
3. **Environment**: Environment variables are passed to containers
4. **Networking**: Uses Docker Compose networking

### Example docker-compose.yml

```yaml
version: '3.8'

services:
  frontend:
    image: node:18
    working_dir: /workspace
    volumes:
      - .:/workspace
    environment:
      - NODE_ENV=development

  backend:
    image: php:8.2-cli
    working_dir: /workspace  
    volumes:
      - .:/workspace
    environment:
      - APP_ENV=development
```

## Glob Patterns

Doctrus supports advanced glob patterns for inputs and outputs:

### Basic Patterns
- `*.js` - All JS files in current directory
- `src/*.ts` - All TypeScript files in src directory
- `test/**` - All files and directories under test (recursive)

### Advanced Patterns  
- `**/*.js` - All JS files recursively in any subdirectory
- `src/**/*.{ts,tsx}` - All TypeScript files in src and subdirectories
- `build/**/!(*.map)` - All files except source maps in build directory
- `{package.json,yarn.lock}` - Multiple specific files

### Examples
```yaml
tasks:
  build:
    inputs:
      - "src/**/*.{ts,tsx,js,jsx}"  # All source files
      - "public/**/*"               # All public assets
      - "package.json"              # Package config
      - "tsconfig.json"             # TypeScript config
    outputs:
      - "dist/**/*"                 # All build outputs
```

## Caching

Doctrus uses content-based caching to skip unnecessary task executions:

- **Input Tracking**: Monitors specified input files/patterns using SHA256 hashing
- **Output Verification**: Checks that outputs exist using glob patterns
- **Hash Comparison**: Uses SHA256 to detect changes in input files
- **Dependency Chain**: Invalidates dependents when inputs change

**Cache Storage**: `~/.doctrus/cache/` (configurable with `--cache-dir`)

## Best Practices

1. **Define Inputs/Outputs**: Accurate patterns improve cache effectiveness
2. **Granular Tasks**: Smaller tasks = better caching and parallelism  
3. **Use Dependencies**: Let Doctrus handle execution order
4. **Environment Variables**: Keep environment-specific config separate
5. **Docker Optimization**: Use .dockerignore to reduce context size

## Troubleshooting

### Cache Issues
```bash
doctrus cache clear          # Clear all cache
doctrus run --skip-cache     # Bypass cache temporarily
```

### Docker Problems
```bash
docker compose up -d         # Ensure containers are running
doctrus validate             # Check Docker integration
```

### Task Dependencies
```bash
doctrus list -v              # Show task details and dependencies
doctrus run --dry-run task   # Preview execution order
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.