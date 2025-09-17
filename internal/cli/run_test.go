package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"doctrus/internal/config"
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
