package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRootCommandDelegatesToRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell commands not available on Windows test environment")
	}

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "doctrus.yml")
	configContent := `version: "1.0"
workspaces:
  app:
    path: .
    tasks:
      greet:
        command: ["sh", "-c", "echo hi > alias.txt"]
`

	if err := os.WriteFile(cfgPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	origConfigPath := configPath
	origCacheDir := cacheDir
	origForce := forceBuild
	origSkip := skipCache
	origDryRun := dryRun
	origShowDiff := showDiff
	origParallel := parallel

	cacheDir = ""
	forceBuild = false
	skipCache = false
	dryRun = false
	showDiff = false
	parallel = 1

	rootCmd.SetArgs([]string{"--config", cfgPath, "app:greet"})

	t.Cleanup(func() {
		cacheDir = origCacheDir
		configPath = origConfigPath
		forceBuild = origForce
		skipCache = origSkip
		dryRun = origDryRun
		showDiff = origShowDiff
		parallel = origParallel
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() error = %v", err)
	}

	aliasFile := filepath.Join(tempDir, "alias.txt")
	if _, err := os.Stat(aliasFile); err != nil {
		t.Fatalf("expected alias task to create file: %v", err)
	}
}

func TestRootCommandPrefersBuiltinCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell commands not available on Windows test environment")
	}

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "doctrus.yml")
	sentinel := filepath.Join(tempDir, "task-validate.txt")
	configContent := `version: "1.0"
workspaces:
  app:
    path: .
    tasks:
      validate:
        command: ["sh", "-c", "echo task > task-validate.txt"]
`

	if err := os.WriteFile(cfgPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	origConfigPath := configPath
	origCacheDir := cacheDir
	origForce := forceBuild
	origSkip := skipCache
	origDryRun := dryRun
	origShowDiff := showDiff
	origParallel := parallel

	cacheDir = ""
	forceBuild = false
	skipCache = false
	dryRun = false
	showDiff = false
	parallel = 1

	t.Cleanup(func() {
		cacheDir = origCacheDir
		configPath = origConfigPath
		forceBuild = origForce
		skipCache = origSkip
		dryRun = origDryRun
		showDiff = origShowDiff
		parallel = origParallel
		rootCmd.SetArgs(nil)
	})

	rootCmd.SetArgs([]string{"--config", cfgPath, "validate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() validate error = %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Fatalf("expected built-in validate command to run instead of task")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected sentinel error: %v", err)
	}

	// Explicit run should execute the task and create the sentinel file.
	rootCmd.SetArgs([]string{"--config", cfgPath, "run", "app:validate"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute() run app:validate error = %v", err)
	}

	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("expected task validate to create sentinel file: %v", err)
	}
}
