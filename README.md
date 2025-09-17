# Doctrus

[![CI/CD](https://github.com/SebastiaanWouters/doctrus/actions/workflows/release.yml/badge.svg)](https://github.com/SebastiaanWouters/doctrus/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/SebastiaanWouters/doctrus)](https://goreportcard.com/report/github.com/SebastiaanWouters/doctrus)

A powerful monorepo task runner with Docker Compose integration, intelligent caching, and dependency tracking.

## Features

- 🏗️ **Monorepo Support**: Manage multiple workspaces from a single configuration
- 🐳 **Docker Integration**: Run tasks inside Docker containers via Docker Compose
- 📦 **Smart Caching**: Skip tasks when inputs haven't changed using SHA256 hashing
- 🔄 **Dependency Tracking**: Automatic dependency resolution with diamond dependency support
- ⚡ **Performance**: Built in Go for speed and reliability with efficient topological sorting
- 🎯 **Flexible**: Works with npm, composer, Go, and any command-line tools
- 🔗 **Cross-workspace Dependencies**: Tasks can depend on tasks in other workspaces
- 🔄 **Efficient Execution**: Each dependency executes only once, even in complex dependency graphs

## Quick Start

### Installation

#### Option 1: Automated Installation (Recommended)

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/SebastiaanWouters/doctrus/main/install.sh | bash
```

#### Option 2: Build from Source

```bash
git clone https://github.com/SebastiaanWouters/doctrus.git
cd doctrus
go build -o doctrus .
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

# Shorthand: run task without the `run` command
doctrus frontend:build

# Run task in all workspaces where it exists
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

pre:
  - command: ["mkdir", "-p", ".doctrus/cache"]
    description: "Ensure task cache directory exists"

workspaces:
  frontend:
    path: ./frontend                       # Optional: defaults to the doctrus.yml directory
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

Workspace options:
- `path` (optional): working directory for all tasks in the workspace. If omitted, Doctrus uses the directory containing `doctrus.yml` locally and the container's default working directory when running in Docker. When set, Doctrus changes into this directory before executing commands both locally and inside containers.

Key task options:
- `verbose` (default `true`): controls whether Doctrus prints the task's command stdout/stderr. Set it to `false` for especially noisy commands; use `doctrus run --verbose` to override at runtime.
- `pre`: optional commands that execute once before any tasks fire during `doctrus run`, useful for provisioning directories or dependencies.
- `parallel` (default `false`): available on compound tasks (those without a `command`). When set to `true`, Doctrus runs the task's immediate dependencies in parallel instead of sequentially.
- You can invoke tasks directly without `run` (for example, `doctrus build`). Built-in commands like `doctrus validate` continue to take precedence, so tasks that reuse those names still require `doctrus run`.

## Multi-Workspace Task Execution

When running a task without specifying a workspace, Doctrus will execute the task in **all workspaces** where it is defined. This behavior aligns with industry standards from tools like Turborepo, Nx, and Lerna.

### Examples

```yaml
version: "1.0"

workspaces:
  frontend:
    path: ./frontend
    tasks:
      build:
        command: ["npm", "run", "build"]
      test:
        command: ["npm", "test"]

  backend:
    path: ./backend
    tasks:
      build:
        command: ["go", "build", "."]
      lint:
        command: ["golangci-lint", "run"]

  shared:
    path: ./shared
    tasks:
      build:
        command: ["npm", "run", "build"]
```

**Task Execution Examples:**
```bash
# Runs build in frontend, backend, AND shared workspaces
doctrus run build

# Runs test only in frontend (only workspace that has it)
doctrus run test

# Runs lint only in backend (only workspace that has it)
doctrus run lint

# Run specific workspace task
doctrus run frontend:build
```

### Compound Tasks

Create tasks that orchestrate other tasks without executing commands themselves:

```yaml
workspaces:
  frontend:
    path: ./frontend
    tasks:
      install:
        command: ["npm", "ci"]
        cache: true

      build:
        command: ["npm", "run", "build"]
        depends_on: ["install"]
        cache: true

      test:
        command: ["npm", "test"]
        depends_on: ["install"]

      # Compound task - no command, only dependencies
      full-build:
        description: "Complete build and test"
        depends_on: ["build", "test"]

  root:
    path: .
    tasks:
      # Cross-workspace compound task
      build-all:
        description: "Build everything"
        depends_on: ["frontend:full-build", "backend:build"]

      # Sequential compound task
      deploy:
        description: "Deploy all components"
        depends_on: ["build-all"]
```

**Running Compound Tasks:**
```bash
# Runs install → build → test → full-build (compound)
doctrus run frontend:full-build

# Runs all build tasks across workspaces, then build-all (compound)
doctrus run root:build-all

# If multiple workspaces have compound tasks with the same name
doctrus run full-build  # Runs in all workspaces that have it
```

## Configuration Reference

### Workspace Configuration

- **path**: Directory path (relative or absolute)
- **container**: Docker container name from docker-compose.yml
- **env**: Environment variables for all tasks in workspace
- **tasks**: Map of task definitions

### Task Configuration

- **command**: Command to execute (array of strings, optional for compound tasks)
- **description**: Human-readable description
- **depends_on**: Array of task dependencies
  - `"task"` - task in same workspace
  - `"workspace:task"` - task in different workspace
- **inputs**: File patterns to watch for changes (supports advanced globs including `**/*`)
- **outputs**: File patterns produced by task (supports advanced globs including `**/*`)
- **cache**: Enable/disable caching (default: false)
- **env**: Task-specific environment variables

#### Input/Output Patterns & Caching

**Inputs** define files that the task depends on:
- Changes to input files trigger task re-execution
- Supports glob patterns: `src/**/*`, `package*.json`, etc.
- SHA256 hashes are computed for change detection

**Outputs** define files that the task produces:
- Used to verify task completion
- If output files are missing, task will re-run
- Supports same glob patterns as inputs

**Cache** enables intelligent task skipping:
- When `cache: true`, Doctrus tracks input changes
- If inputs haven't changed and outputs exist, task is skipped
- Dramatically speeds up development workflows
- Can be overridden with `--force` or `--skip-cache` flags

**Example with caching:**
```yaml
tasks:
  install:
    command: ["npm", "install"]
    inputs: ["package.json", "package-lock.json"]  # Watch for dependency changes
    outputs: ["node_modules/**/*"]                 # Verify installation
    cache: true                                    # Enable caching

  build:
    command: ["npm", "run", "build"]
    depends_on: ["install"]
    inputs: ["src/**/*", "public/**/*"]           # Watch source files
    outputs: ["dist/**/*"]                        # Verify build output
    cache: true                                   # Enable caching
```

#### Compound Tasks

Tasks without a command that only exist to orchestrate dependencies:

```yaml
tasks:
  full-build:
    description: "Build and test everything"
    depends_on: ["build", "test"]
    # No command - this is a compound task
```

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
doctrus run build                    # Run 'build' in all workspaces where it exists
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
2. **Working Directory**: Uses the container's default working directory from docker-compose.yml
3. **Environment**: Environment variables are passed to containers
4. **Networking**: Uses Docker Compose networking
5. **Running Containers Required**: Containers must be running before executing tasks in them

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

**Cache Storage**: `{project-root}/.doctrus/cache/` (where project-root contains doctrus.yml)

### Cache Architecture

Doctrus manages caching at the host level:

1. **Host-managed cache**: Doctrus runs on the host and manages all caching decisions
2. **Task execution agnostic**: Tasks don't directly interact with the cache files
3. **Cross-environment consistency**: Cache works the same whether tasks run locally or in containers
4. **Environment variable**: `DOCTRUS_CACHE_DIR` is set for informational purposes

**Note**: The cache is managed by Doctrus itself, not by the individual tasks.

## Dependency Resolution

Doctrus uses an efficient graph-based algorithm to resolve task dependencies:

### Algorithm Overview
1. **Graph Construction**: Builds a directed acyclic graph (DAG) of task dependencies
2. **Topological Sorting**: Uses Kahn's algorithm to determine execution order
3. **Deduplication**: Each task executes only once, even with multiple dependents
4. **Cycle Detection**: Prevents infinite loops by detecting circular dependencies

### Supported Patterns
- **Simple chains**: `A → B → C`
- **Diamond dependencies**: `A → B,C → D` (D executes only once)
- **Cross-workspace**: `frontend:build → backend:test`
- **Compound tasks**: Tasks with only dependencies (no commands)

### Execution Order
Tasks are executed in dependency order:
```bash
# For: frontend:build → backend:compile, backend:test → shared:setup
# Execution: shared:setup → backend:compile → backend:test → frontend:build
```

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
