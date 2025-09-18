package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"doctrus/internal/cache"
	"doctrus/internal/config"
	"doctrus/internal/deps"
	"doctrus/internal/docker"
	"doctrus/internal/workspace"
)

func TestIsTaskVerbose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task *config.Task
		want bool
	}{
		{
			name: "nil task",
			task: nil,
			want: true,
		},
		{
			name: "no verbose field defaults to true",
			task: &config.Task{},
			want: true,
		},
		{
			name: "explicit false",
			task: &config.Task{Verbose: boolPtr(false)},
			want: false,
		},
		{
			name: "explicit true",
			task: &config.Task{Verbose: boolPtr(true)},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isTaskVerbose(tt.task); got != tt.want {
				t.Fatalf("isTaskVerbose() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTaskParallel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task *config.Task
		want bool
	}{
		{
			name: "nil task",
			task: nil,
			want: false,
		},
		{
			name: "default false",
			task: &config.Task{},
			want: false,
		},
		{
			name: "explicit true",
			task: &config.Task{Parallel: boolPtr(true)},
			want: true,
		},
		{
			name: "explicit false",
			task: &config.Task{Parallel: boolPtr(false)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isTaskParallel(tt.task); got != tt.want {
				t.Fatalf("isTaskParallel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskLogWriterPrefixesOnlyWhenRequested(t *testing.T) {
	t.Parallel()

	cli := &CLI{}

	t.Run("no prefix retains raw output", func(t *testing.T) {
		var buf bytes.Buffer
		writer := newTaskLogWriter(cli, "app:lint", "stdout", false).(*taskLogWriter)
		writer.dest = &buf

		msg := "Regular output ✨\nSecond line"
		if _, err := writer.Write([]byte(msg)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		if got, want := buf.String(), msg; got != want {
			t.Fatalf("Write() got %q, want %q", got, want)
		}
	})

	t.Run("prefix applies per line for compound flows", func(t *testing.T) {
		var buf bytes.Buffer
		writer := newTaskLogWriter(cli, "web:build", "stderr", true).(*taskLogWriter)
		writer.dest = &buf

		msg := "line one\nsecond 🎉\nthird"
		if _, err := writer.Write([]byte(msg)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		want := "[web:build][stderr] line one\n[web:build][stderr] second 🎉\n[web:build][stderr] third"
		if got := buf.String(); got != want {
			t.Fatalf("Write() got %q, want %q", got, want)
		}
	})
}

func boolPtr(v bool) *bool {
	return &v
}

func TestEnsurePreRunCommands(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := &config.Config{
		Version: "1.0",
		Pre: []config.PreCommand{
			{Command: []string{"mkdir", "-p", "cache"}},
		},
		Workspaces: map[string]config.Workspace{
			"app": {
				Path:  "./app",
				Tasks: map[string]config.Task{"build": {Command: []string{"echo", "build"}}},
			},
		},
	}

	cli := &CLI{
		config:   cfg,
		basePath: tempDir,
	}

	ctx := context.Background()

	if err := cli.ensurePreRunCommands(ctx); err != nil {
		t.Fatalf("ensurePreRunCommands() error = %v", err)
	}

	cacheDir := filepath.Join(tempDir, "cache")
	if _, err := os.Stat(cacheDir); err != nil {
		t.Fatalf("expected cache dir to exist: %v", err)
	}

	if !cli.preRunExecuted {
		t.Fatalf("expected preRunExecuted to be true")
	}

	// Subsequent calls should be no-ops
	if err := cli.ensurePreRunCommands(ctx); err != nil {
		t.Fatalf("ensurePreRunCommands() second call error = %v", err)
	}
}

func TestParallelCompoundRunsDependenciesConcurrently(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell sleep command not available on Windows")
	}

	// Use serial execution to avoid interference with global flags.
	tempDir := t.TempDir()
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}

	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"app": {
				Path: tempDir,
				Tasks: map[string]config.Task{
					"slowA": {
						Command: []string{"sh", "-c", "sleep 0.3"},
					},
					"slowB": {
						Command: []string{"sh", "-c", "sleep 0.3"},
					},
					"bundle": {
						DependsOn: []string{"slowA", "slowB"},
						Parallel:  boolPtr(true),
					},
				},
			},
		},
	}

	workspaceManager := workspace.NewManager(cfg, tempDir)
	if err := workspaceManager.ValidateWorkspaces(); err != nil {
		t.Fatalf("ValidateWorkspaces() error = %v", err)
	}

	cli := &CLI{
		config:    cfg,
		workspace: workspaceManager,
		executor:  docker.NewExecutor(cfg, tempDir),
		tracker:   deps.NewTracker(tempDir),
		cache:     cache.NewManager(filepath.Join(tempDir, ".doctrus", "cache")),
		basePath:  tempDir,
	}

	ctx := context.Background()
	runner := newTaskRunner(cli)

	origForce := forceBuild
	origSkip := skipCache
	origDryRun := dryRun
	origShowDiff := showDiff
	origParallel := parallel
	t.Cleanup(func() {
		forceBuild = origForce
		skipCache = origSkip
		dryRun = origDryRun
		showDiff = origShowDiff
		parallel = origParallel
	})

	forceBuild = false
	skipCache = false
	dryRun = false
	showDiff = false
	parallel = 1

	start := time.Now()
	if err := cli.runTaskInWorkspace(ctx, runner, "app", "bundle"); err != nil {
		t.Fatalf("runTaskInWorkspace() error = %v", err)
	}
	duration := time.Since(start)

	if duration > 450*time.Millisecond {
		t.Fatalf("expected parallel execution to finish sooner, took %v", duration)
	}
}
