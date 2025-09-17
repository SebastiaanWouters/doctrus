package cli

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"doctrus/internal/cache"
	"doctrus/internal/config"
	"doctrus/internal/deps"
	"doctrus/internal/docker"
	"doctrus/internal/workspace"
)

var (
	configPath string
	verbose    bool
	dryRun     bool
	cacheDir   string
)

type CLI struct {
	config         *config.Config
	workspace      *workspace.Manager
	executor       *docker.Executor
	tracker        *deps.Tracker
	cache          *cache.Manager
	basePath       string
	preRunExecuted bool
	outputMu       sync.Mutex
}

func newCLI() (*CLI, error) {
	cfg, configDir, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Use the directory containing doctrus.yml as the base path
	basePath := configDir
	if basePath == "" {
		basePath, err = filepath.Abs(".")
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	workspaceManager := workspace.NewManager(cfg, basePath)
	executor := docker.NewExecutor(cfg, basePath)
	tracker := deps.NewTracker(basePath)

	// Resolve cache directory
	if cacheDir == "" {
		cacheDir = filepath.Join(basePath, ".doctrus", "cache")
	}
	cacheManager := cache.NewManager(cacheDir)

	if err := workspaceManager.ValidateWorkspaces(); err != nil {
		return nil, fmt.Errorf("workspace validation failed: %w", err)
	}

	return &CLI{
		config:    cfg,
		workspace: workspaceManager,
		executor:  executor,
		tracker:   tracker,
		cache:     cacheManager,
		basePath:  basePath,
	}, nil
}

var rootCmd = &cobra.Command{
	Use:   "doctrus",
	Short: "A powerful monorepo task runner with Docker support",
	Long: `Doctrus is a monorepo management tool that helps you run tasks across
multiple workspaces with Docker Compose integration, intelligent caching,
and dependency tracking.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "doctrus.yml", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be executed without running it")
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", "", "Cache directory (default: ~/.doctrus/cache)")

	rootCmd.AddCommand(
		newRunCommand(),
		newListCommand(),
		newCacheCommand(),
		newValidateCommand(),
		newInitCommand(),
	)
}
