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
}

type DockerConfig struct {
	ComposeFile string `yaml:"compose_file,omitempty"`
}

func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "doctrus.yml"
	}

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", absPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required")
	}

	if len(c.Workspaces) == 0 {
		return fmt.Errorf("at least one workspace is required")
	}

	for name, workspace := range c.Workspaces {
		if workspace.Path == "" {
			return fmt.Errorf("workspace %s: path is required", name)
		}

		if len(workspace.Tasks) == 0 {
			return fmt.Errorf("workspace %s: at least one task is required", name)
		}

		for taskName, task := range workspace.Tasks {
			if len(task.Command) == 0 {
				return fmt.Errorf("workspace %s, task %s: command is required", name, taskName)
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