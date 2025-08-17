package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config",
			config:  Config{},
			wantErr: true,
			errMsg:  "version is required",
		},
		{
			name: "missing workspaces",
			config: Config{
				Version: "1.0",
			},
			wantErr: true,
			errMsg:  "at least one workspace is required",
		},
		{
			name: "workspace without path",
			config: Config{
				Version: "1.0",
				Workspaces: map[string]Workspace{
					"test": {
						Tasks: map[string]Task{
							"build": {Command: []string{"echo", "test"}},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "workspace test: path is required",
		},
		{
			name: "workspace without tasks",
			config: Config{
				Version: "1.0",
				Workspaces: map[string]Workspace{
					"test": {
						Path: "./test",
					},
				},
			},
			wantErr: true,
			errMsg:  "workspace test: at least one task is required",
		},
		{
			name: "task without command",
			config: Config{
				Version: "1.0",
				Workspaces: map[string]Workspace{
					"test": {
						Path: "./test",
						Tasks: map[string]Task{
							"build": {},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "workspace test, task build: command is required",
		},
		{
			name: "valid config",
			config: Config{
				Version: "1.0",
				Workspaces: map[string]Workspace{
					"frontend": {
						Path: "./frontend",
						Tasks: map[string]Task{
							"build": {
								Command:     []string{"npm", "run", "build"},
								Description: "Build frontend",
							},
							"test": {
								Command:   []string{"npm", "test"},
								DependsOn: []string{"build"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with docker",
			config: Config{
				Version: "1.0",
				Docker: DockerConfig{
					ComposeFile: "docker-compose.yml",
				},
				Workspaces: map[string]Workspace{
					"backend": {
						Path:      "./backend",
						Container: "backend-app",
						Tasks: map[string]Task{
							"start": {
								Command: []string{"go", "run", "main.go"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("Config.validate() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestConfigLoad(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		yamlContent string
		wantErr     bool
	}{
		{
			name: "valid yaml",
			yamlContent: `version: "1.0"
workspaces:
  frontend:
    path: ./frontend
    tasks:
      build:
        command: ["npm", "run", "build"]
        description: "Build the frontend"
      test:
        command: ["npm", "test"]
        depends_on: ["build"]`,
			wantErr: false,
		},
		{
			name: "invalid yaml",
			yamlContent: `version: "1.0"
workspaces:
  frontend:
    path: ./frontend
    tasks:
      build:
        command: [invalid yaml`,
			wantErr: true,
		},
		{
			name: "yaml with environment variables",
			yamlContent: `version: "1.0"
workspaces:
  backend:
    path: ./backend
    env:
      NODE_ENV: production
      PORT: "3000"
    tasks:
      start:
        command: ["node", "server.js"]
        env:
          DEBUG: "true"`,
			wantErr: false,
		},
		{
			name: "yaml with cache and io",
			yamlContent: `version: "1.0"
workspaces:
  app:
    path: ./app
    tasks:
      build:
        command: ["make", "build"]
        inputs: ["src/**/*.go", "go.mod"]
        outputs: ["bin/app"]
        cache: true`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(tempDir, tt.name+".yml")
			err := os.WriteFile(configFile, []byte(tt.yamlContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config file: %v", err)
			}

			config, err := Load(configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config == nil {
				t.Error("Load() returned nil config without error")
			}
		})
	}
}

func TestConfigLoadNonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/file.yml")
	if err == nil {
		t.Error("Load() should return error for non-existent file")
	}
}

func TestConfigGetWorkspace(t *testing.T) {
	config := &Config{
		Version: "1.0",
		Workspaces: map[string]Workspace{
			"frontend": {
				Path: "./frontend",
				Tasks: map[string]Task{
					"build": {Command: []string{"npm", "build"}},
				},
			},
			"backend": {
				Path: "./backend",
				Tasks: map[string]Task{
					"test": {Command: []string{"go", "test"}},
				},
			},
		},
	}

	tests := []struct {
		name          string
		workspaceName string
		wantExists    bool
	}{
		{
			name:          "existing workspace",
			workspaceName: "frontend",
			wantExists:    true,
		},
		{
			name:          "another existing workspace",
			workspaceName: "backend",
			wantExists:    true,
		},
		{
			name:          "non-existing workspace",
			workspaceName: "nonexistent",
			wantExists:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace, exists := config.GetWorkspace(tt.workspaceName)
			if exists != tt.wantExists {
				t.Errorf("GetWorkspace() exists = %v, want %v", exists, tt.wantExists)
			}
			if exists && workspace == nil {
				t.Error("GetWorkspace() returned nil workspace with exists=true")
			}
		})
	}
}

func TestConfigGetTask(t *testing.T) {
	config := &Config{
		Version: "1.0",
		Workspaces: map[string]Workspace{
			"frontend": {
				Path: "./frontend",
				Tasks: map[string]Task{
					"build": {
						Command:     []string{"npm", "build"},
						Description: "Build frontend",
					},
					"test": {
						Command:   []string{"npm", "test"},
						DependsOn: []string{"build"},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		workspaceName string
		taskName      string
		wantExists    bool
	}{
		{
			name:          "existing task",
			workspaceName: "frontend",
			taskName:      "build",
			wantExists:    true,
		},
		{
			name:          "another existing task",
			workspaceName: "frontend",
			taskName:      "test",
			wantExists:    true,
		},
		{
			name:          "non-existing task",
			workspaceName: "frontend",
			taskName:      "deploy",
			wantExists:    false,
		},
		{
			name:          "non-existing workspace",
			workspaceName: "backend",
			taskName:      "build",
			wantExists:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, exists := config.GetTask(tt.workspaceName, tt.taskName)
			if exists != tt.wantExists {
				t.Errorf("GetTask() exists = %v, want %v", exists, tt.wantExists)
			}
			if exists && task == nil {
				t.Error("GetTask() returned nil task with exists=true")
			}
		})
	}
}