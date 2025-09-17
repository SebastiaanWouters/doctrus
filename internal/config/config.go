package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version    string               `yaml:"version"`
	Workspaces map[string]Workspace `yaml:"workspaces"`
	Docker     DockerConfig         `yaml:"docker,omitempty"`
	Pre        []PreCommand         `yaml:"pre,omitempty"`
}

type Workspace struct {
	Path      string            `yaml:"path"`
	Container string            `yaml:"container,omitempty"`
	Tasks     map[string]Task   `yaml:"tasks"`
	Env       map[string]string `yaml:"env,omitempty"`
}

type Task struct {
	Command     []string          `yaml:"command"`
	Description string            `yaml:"description,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	Inputs      []string          `yaml:"inputs,omitempty"`
	Outputs     []string          `yaml:"outputs,omitempty"`
	Cache       bool              `yaml:"cache,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	Container   *string           `yaml:"container,omitempty"`
	Docker      *TaskDockerConfig `yaml:"docker,omitempty"`
	Verbose     *bool             `yaml:"verbose,omitempty"`
}

type PreCommand struct {
	Command     []string          `yaml:"command"`
	Description string            `yaml:"description,omitempty"`
	Dir         string            `yaml:"dir,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	Verbose     *bool             `yaml:"verbose,omitempty"`
}

type DockerConfig struct {
	ComposeFile string `yaml:"compose_file,omitempty"`
}

type TaskDockerConfig struct {
	ComposeFile string `yaml:"compose_file,omitempty"`
	Disable     bool   `yaml:"disable,omitempty"`
}

func Load(configPath string) (*Config, string, error) {
	if configPath == "" {
		configPath = "doctrus.yml"
	}

	// If config path is not absolute, search for it in parent directories
	var absPath string
	var configDir string

	if filepath.IsAbs(configPath) {
		absPath = configPath
		configDir = filepath.Dir(absPath)
	} else {
		// Search for config file in current and parent directories
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("failed to get working directory: %w", err)
		}

		foundPath, foundDir := findConfigInParents(currentDir, configPath)
		if foundPath == "" {
			// If not found, try the original path relative to cwd
			absPath = filepath.Join(currentDir, configPath)
			configDir = currentDir
		} else {
			absPath = foundPath
			configDir = foundDir
		}
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read config file %s: %w", absPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, "", fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, "", fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, configDir, nil
}

// findConfigInParents searches for a config file in the current and parent directories
func findConfigInParents(startDir, configName string) (string, string) {
	currentDir := startDir

	for {
		configPath := filepath.Join(currentDir, configName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, currentDir
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached root directory
			break
		}
		currentDir = parentDir
	}

	return "", ""
}

func (c *Config) validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	if len(c.Workspaces) == 0 {
		return fmt.Errorf("at least one workspace is required")
	}

	for i, pre := range c.Pre {
		if len(pre.Command) == 0 {
			return fmt.Errorf("pre[%d]: command is required", i)
		}
	}

	for name, workspace := range c.Workspaces {
		if len(workspace.Tasks) == 0 {
			return fmt.Errorf("workspace %s: at least one task is required", name)
		}

		for taskName, task := range workspace.Tasks {
			if len(task.Command) == 0 && len(task.DependsOn) == 0 {
				return fmt.Errorf("workspace %s, task %s: command is required unless task has dependencies (compound task)", name, taskName)
			}
		}
	}

	return nil
}

func (c *Config) GetWorkspace(name string) (*Workspace, bool) {
	workspace, exists := c.Workspaces[name]
	return &workspace, exists
}

func (c *Config) GetTask(workspaceName, taskName string) (*Task, bool) {
	workspace, exists := c.Workspaces[workspaceName]
	if !exists {
		return nil, false
	}

	task, exists := workspace.Tasks[taskName]
	return &task, exists
}

// GetEffectiveContainer returns the effective container name for a task,
// considering task-level overrides and workspace defaults
func (c *Config) GetEffectiveContainer(workspaceName, taskName string) string {
	workspace, exists := c.Workspaces[workspaceName]
	if !exists {
		return ""
	}

	task, exists := workspace.Tasks[taskName]
	if !exists {
		return ""
	}

	// Check if Docker is explicitly disabled at task level
	if task.Docker != nil && task.Docker.Disable {
		return ""
	}

	// Task-level container override takes precedence
	if task.Container != nil {
		return *task.Container
	}

	// Fall back to workspace container
	return workspace.Container
}

// GetEffectiveDockerConfig returns the effective Docker configuration for a task,
// considering task-level overrides and workspace/global defaults
func (c *Config) GetEffectiveDockerConfig(workspaceName, taskName string) DockerConfig {
	workspace, exists := c.Workspaces[workspaceName]
	if !exists {
		return c.Docker
	}

	task, exists := workspace.Tasks[taskName]
	if !exists {
		return c.Docker
	}

	// Start with global Docker config
	config := c.Docker

	// Override with task-specific Docker config if present
	if task.Docker != nil && task.Docker.ComposeFile != "" {
		config.ComposeFile = task.Docker.ComposeFile
	}

	return config
}
