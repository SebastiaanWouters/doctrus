# Repository Guidelines

## Project Structure & Module Organization
- `main.go` wires the CLI entry point through `internal/cli`.
- `internal/` houses core packages: `cli` (Cobra commands), `config` (YAML parsing), `workspace` (workspace management), `deps` (dependency tracking), `cache` (SHA-based caching), and `docker` (compose integration).
- `examples/` holds sample `doctrus.yml` setups that illustrate workspace/task definitions.
- `.github/workflows/ci.yml` defines the Go build-and-test pipeline; update it alongside tooling changes.

## Build, Test, and Development Commands
- `go build -o doctrus .` compiles the CLI for local use; target Go ≥1.24.5 as declared in `go.mod`.
- `go run . list` executes the CLI without installing, showing workspaces and tasks resolved from the current `doctrus.yml`.
- `go test ./...` runs unit tests across packages; add `-race` for concurrency-sensitive updates.
- `./doctrus run <workspace:task>` runs repository tasks with caching and dependency resolution.

## Coding Style & Naming Conventions
- Format Go sources with `go fmt ./...`; stick to tabs for indentation and let imports be organized automatically.
- Keep package names short and lowercase (`config`, `workspace`); export identifiers in PascalCase with doc comments when part of the CLI surface.
- Place Cobra subcommands in `internal/cli`, naming files after command verbs (e.g., `run.go`, `list.go`).

## Testing Guidelines
- Colocate `_test.go` files with their packages and mirror the table-driven style used in `internal/workspace/manager_test.go`.
- Use `t.Run` subtests and `t.TempDir()` for filesystem scenarios; ensure deterministic fixture paths.
- Verify coverage for cache and workspace logic via `go test ./... -cover` before opening reviews.

## Commit & Pull Request Guidelines
- Follow the Conventional Commit prefixes evident in history (`feat:`, `fix:`, `remove:`); add scopes when it clarifies impact (e.g., `feat(cli):`).
- Draft PRs with context, runnable command transcripts, and screenshots when CLI UX changes.
- Link issues with `Fixes #<id>` and confirm CI status green before requesting review.

## Configuration & Cache Tips
- Keep the canonical `doctrus.yml` at the repo root and document new workspace patterns under `examples/` for discoverability.
- Workspace `path` is optional; leaving it blank executes commands from the directory containing `doctrus.yml` locally and from the container's default working directory. When set, Doctrus switches into it before task execution in both environments.
- Use `pre` entries for repository-wide setup (e.g., `mkdir -p cache`) that must happen before any task executes; they run once per CLI invocation from the repo root.
- Tasks accept `verbose: false` to suppress stdout/stderr in routine runs; `doctrus run --verbose` forces output when debugging.
- Compound tasks (only `depends_on`) may set `parallel: true` to execute their dependencies concurrently; otherwise dependencies run sequentially.
- Developers can call tasks without explicitly using the `run` subcommand (e.g., `doctrus lint`). Built-in commands still win conflicts, so a user-defined task named `validate` must be invoked with `doctrus run validate`.
- Repository cache resides at `.doctrus/cache`; use `doctrus cache clear` or `doctrus run --force` after changing task inputs or outputs.
