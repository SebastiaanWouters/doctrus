package docker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"doctrus/internal/config"
	"doctrus/internal/workspace"
)

func TestExecutorContainerWorkDir(t *testing.T) {
	baseDir := t.TempDir()
	executor := &Executor{config: &config.Config{}, workingDir: baseDir}

	tests := []struct {
		name          string
		workspacePath string
		absPath       string
		wantPath      string
		wantAbsolute  bool
	}{
		{
			name:          "empty path",
			workspacePath: "",
			absPath:       baseDir,
			wantPath:      "",
			wantAbsolute:  false,
		},
		{
			name:          "relative path",
			workspacePath: "./frontend",
			absPath:       filepath.Join(baseDir, "frontend"),
			wantPath:      "frontend",
			wantAbsolute:  false,
		},
		{
			name:          "relative parent path",
			workspacePath: "../shared",
			absPath:       filepath.Join(baseDir, "../shared"),
			wantPath:      "../shared",
			wantAbsolute:  false,
		},
		{
			name:          "absolute path",
			workspacePath: filepath.Join(baseDir, "services", "api"),
			absPath:       filepath.Join(baseDir, "services", "api"),
			wantPath:      filepath.ToSlash(filepath.Join(baseDir, "services", "api")),
			wantAbsolute:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &workspace.TaskExecution{
				Workspace: &config.Workspace{Path: tt.workspacePath},
				AbsPath:   tt.absPath,
			}

			gotPath, gotAbsolute := executor.containerWorkDir(exec)
			if gotPath != tt.wantPath {
				t.Fatalf("containerWorkDir() path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotAbsolute != tt.wantAbsolute {
				t.Fatalf("containerWorkDir() absolute = %v, want %v", gotAbsolute, tt.wantAbsolute)
			}
		})
	}
}

func TestBuildShellCommand(t *testing.T) {
	got := buildShellCommand("frontend", []string{"npm", "run", "build"})
	want := "cd 'frontend' && 'npm' 'run' 'build'"
	if got != want {
		t.Fatalf("buildShellCommand() = %q, want %q", got, want)
	}

	gotRoot := buildShellCommand("", []string{"echo", "hello world"})
	wantRoot := "cd '.' && 'echo' 'hello world'"
	if gotRoot != wantRoot {
		t.Fatalf("buildShellCommand() root = %q, want %q", gotRoot, wantRoot)
	}
}

func TestExecuteLocalUsesWorkspacePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pwd command not available on Windows")
	}

	baseDir := t.TempDir()
	workspaceDir := filepath.Join(baseDir, "frontend")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}

	executor := NewExecutor(&config.Config{}, baseDir)
	execution := &workspace.TaskExecution{
		WorkspaceName: "app",
		TaskName:      "pwd",
		Task: &config.Task{
			Command: []string{"pwd"},
		},
		Workspace: &config.Workspace{Path: "./frontend"},
		AbsPath:   workspaceDir,
	}

	result := executor.executeLocal(context.Background(), execution, nil, nil)
	if result.Error != nil {
		t.Fatalf("executeLocal() error = %v", result.Error)
	}

	pwd := strings.TrimSpace(result.Stdout)
	if pwd != workspaceDir {
		t.Fatalf("executeLocal() ran in %q, want %q", pwd, workspaceDir)
	}
}
