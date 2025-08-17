package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"doctrus/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{
		Version:    "1.0",
		Workspaces: make(map[string]config.Workspace),
	}

	tests := []struct {
		name     string
		basePath string
	}{
		{
			name:     "with base path",
			basePath: "/test/path",
		},
		{
			name:     "without base path",
			basePath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(cfg, tt.basePath)
			if manager == nil {
				t.Fatal("NewManager() returned nil")
			}
			if manager.config != cfg {
				t.Error("Manager config not set correctly")
			}
			if tt.basePath != "" && manager.basePath != tt.basePath {
				t.Errorf("Manager basePath = %v, want %v", manager.basePath, tt.basePath)
			}
			if tt.basePath == "" && manager.basePath == "" {
				t.Error("Manager basePath not set when empty string provided")
			}
		})
	}
}

func TestManagerGetWorkspaces(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"frontend": {Path: "./frontend"},
			"backend":  {Path: "./backend"},
			"database": {Path: "./database"},
		},
	}

	manager := NewManager(cfg, "/test")
	workspaces := manager.GetWorkspaces()

	expected := []string{"backend", "database", "frontend"}
	if !reflect.DeepEqual(workspaces, expected) {
		t.Errorf("GetWorkspaces() = %v, want %v", workspaces, expected)
	}
}

func TestManagerGetTasks(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"frontend": {
				Path: "./frontend",
				Tasks: map[string]config.Task{
					"build": {Command: []string{"npm", "build"}},
					"test":  {Command: []string{"npm", "test"}},
					"lint":  {Command: []string{"npm", "lint"}},
				},
			},
			"backend": {
				Path: "./backend",
				Tasks: map[string]config.Task{
					"compile": {Command: []string{"go", "build"}},
				},
			},
		},
	}

	manager := NewManager(cfg, "/test")

	tests := []struct {
		name          string
		workspaceName string
		wantTasks     []string
		wantErr       bool
	}{
		{
			name:          "frontend workspace",
			workspaceName: "frontend",
			wantTasks:     []string{"build", "lint", "test"},
			wantErr:       false,
		},
		{
			name:          "backend workspace",
			workspaceName: "backend",
			wantTasks:     []string{"compile"},
			wantErr:       false,
		},
		{
			name:          "non-existent workspace",
			workspaceName: "nonexistent",
			wantTasks:     nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := manager.GetTasks(tt.workspaceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTasks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tasks, tt.wantTasks) {
				t.Errorf("GetTasks() = %v, want %v", tasks, tt.wantTasks)
			}
		})
	}
}

func TestManagerGetAllTasks(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"frontend": {
				Path: "./frontend",
				Tasks: map[string]config.Task{
					"build": {Command: []string{"npm", "build"}},
					"test":  {Command: []string{"npm", "test"}},
				},
			},
			"backend": {
				Path: "./backend",
				Tasks: map[string]config.Task{
					"compile": {Command: []string{"go", "build"}},
					"test":    {Command: []string{"go", "test"}},
				},
			},
		},
	}

	manager := NewManager(cfg, "/test")
	allTasks := manager.GetAllTasks()

	if len(allTasks) != 2 {
		t.Errorf("GetAllTasks() returned %d workspaces, want 2", len(allTasks))
	}

	frontendTasks := allTasks["frontend"]
	sort.Strings(frontendTasks)
	expectedFrontend := []string{"build", "test"}
	if !reflect.DeepEqual(frontendTasks, expectedFrontend) {
		t.Errorf("Frontend tasks = %v, want %v", frontendTasks, expectedFrontend)
	}

	backendTasks := allTasks["backend"]
	sort.Strings(backendTasks)
	expectedBackend := []string{"compile", "test"}
	if !reflect.DeepEqual(backendTasks, expectedBackend) {
		t.Errorf("Backend tasks = %v, want %v", backendTasks, expectedBackend)
	}
}

func TestManagerResolveTaskExecution(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"frontend": {
				Path: "./frontend",
				Tasks: map[string]config.Task{
					"build": {
						Command:     []string{"npm", "build"},
						Description: "Build frontend",
					},
				},
			},
		},
	}

	manager := NewManager(cfg, "/test")

	tests := []struct {
		name          string
		workspaceName string
		taskName      string
		wantErr       bool
	}{
		{
			name:          "valid task",
			workspaceName: "frontend",
			taskName:      "build",
			wantErr:       false,
		},
		{
			name:          "non-existent workspace",
			workspaceName: "backend",
			taskName:      "build",
			wantErr:       true,
		},
		{
			name:          "non-existent task",
			workspaceName: "frontend",
			taskName:      "test",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execution, err := manager.ResolveTaskExecution(tt.workspaceName, tt.taskName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveTaskExecution() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if execution == nil {
					t.Error("ResolveTaskExecution() returned nil without error")
				} else {
					if execution.WorkspaceName != tt.workspaceName {
						t.Errorf("WorkspaceName = %v, want %v", execution.WorkspaceName, tt.workspaceName)
					}
					if execution.TaskName != tt.taskName {
						t.Errorf("TaskName = %v, want %v", execution.TaskName, tt.taskName)
					}
					if execution.Task == nil {
						t.Error("Task is nil")
					}
					if execution.Workspace == nil {
						t.Error("Workspace is nil")
					}
					if execution.AbsPath == "" {
						t.Error("AbsPath is empty")
					}
				}
			}
		})
	}
}

func TestManagerResolveDependencies(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"frontend": {
				Path: "./frontend",
				Tasks: map[string]config.Task{
					"clean": {
						Command: []string{"rm", "-rf", "dist"},
					},
					"build": {
						Command:   []string{"npm", "build"},
						DependsOn: []string{"clean"},
					},
					"test": {
						Command:   []string{"npm", "test"},
						DependsOn: []string{"build"},
					},
					"deploy": {
						Command:   []string{"npm", "deploy"},
						DependsOn: []string{"test"},
					},
				},
			},
			"backend": {
				Path: "./backend",
				Tasks: map[string]config.Task{
					"compile": {
						Command: []string{"go", "build"},
					},
					"test": {
						Command:   []string{"go", "test"},
						DependsOn: []string{"compile"},
					},
				},
			},
		},
	}

	manager := NewManager(cfg, "/test")

	tests := []struct {
		name              string
		workspaceName     string
		taskName          string
		expectedTaskOrder []string
		wantErr           bool
	}{
		{
			name:              "single task no dependencies",
			workspaceName:     "frontend",
			taskName:          "clean",
			expectedTaskOrder: []string{"frontend:clean"},
			wantErr:           false,
		},
		{
			name:              "task with one dependency",
			workspaceName:     "frontend",
			taskName:          "build",
			expectedTaskOrder: []string{"frontend:clean", "frontend:build"},
			wantErr:           false,
		},
		{
			name:              "task with chain of dependencies",
			workspaceName:     "frontend",
			taskName:          "deploy",
			expectedTaskOrder: []string{"frontend:clean", "frontend:build", "frontend:test", "frontend:deploy"},
			wantErr:           false,
		},
		{
			name:          "non-existent task",
			workspaceName: "frontend",
			taskName:      "nonexistent",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executions, err := manager.ResolveDependencies(tt.workspaceName, tt.taskName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(executions) != len(tt.expectedTaskOrder) {
					t.Errorf("ResolveDependencies() returned %d executions, want %d", len(executions), len(tt.expectedTaskOrder))
				}
				for i, execution := range executions {
					key := execution.WorkspaceName + ":" + execution.TaskName
					if key != tt.expectedTaskOrder[i] {
						t.Errorf("Execution[%d] = %s, want %s", i, key, tt.expectedTaskOrder[i])
					}
				}
			}
		})
	}
}

func TestManagerResolveDependenciesCrossWorkspace(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"frontend": {
				Path: "./frontend",
				Tasks: map[string]config.Task{
					"build": {
						Command:   []string{"npm", "build"},
						DependsOn: []string{"backend:build"},
					},
				},
			},
			"backend": {
				Path: "./backend",
				Tasks: map[string]config.Task{
					"build": {
						Command: []string{"go", "build"},
					},
				},
			},
		},
	}

	manager := NewManager(cfg, "/test")
	executions, err := manager.ResolveDependencies("frontend", "build")
	
	if err != nil {
		t.Fatalf("ResolveDependencies() error = %v", err)
	}

	expectedOrder := []string{"backend:build", "frontend:build"}
	if len(executions) != len(expectedOrder) {
		t.Errorf("ResolveDependencies() returned %d executions, want %d", len(executions), len(expectedOrder))
	}

	for i, execution := range executions {
		key := execution.WorkspaceName + ":" + execution.TaskName
		if key != expectedOrder[i] {
			t.Errorf("Execution[%d] = %s, want %s", i, key, expectedOrder[i])
		}
	}
}

func TestManagerResolveDependenciesCircular(t *testing.T) {
	cfg := &config.Config{
		Version: "1.0",
		Workspaces: map[string]config.Workspace{
			"app": {
				Path: "./app",
				Tasks: map[string]config.Task{
					"task1": {
						Command:   []string{"echo", "1"},
						DependsOn: []string{"task2"},
					},
					"task2": {
						Command:   []string{"echo", "2"},
						DependsOn: []string{"task3"},
					},
					"task3": {
						Command:   []string{"echo", "3"},
						DependsOn: []string{"task1"},
					},
				},
			},
		},
	}

	manager := NewManager(cfg, "/test")
	_, err := manager.ResolveDependencies("app", "task1")
	
	if err == nil {
		t.Error("ResolveDependencies() should detect circular dependency")
	}
	if err != nil && !contains(err.Error(), "circular") {
		t.Errorf("ResolveDependencies() error should mention circular dependency, got: %v", err)
	}
}

func TestManagerValidateWorkspaces(t *testing.T) {
	tempDir := t.TempDir()
	
	frontendDir := filepath.Join(tempDir, "frontend")
	os.MkdirAll(frontendDir, 0755)
	
	backendDir := filepath.Join(tempDir, "backend")
	os.MkdirAll(backendDir, 0755)

	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "all workspaces exist",
			config: &config.Config{
				Version: "1.0",
				Workspaces: map[string]config.Workspace{
					"frontend": {
						Path: filepath.Join(tempDir, "frontend"),
						Tasks: map[string]config.Task{
							"build": {Command: []string{"npm", "build"}},
						},
					},
					"backend": {
						Path: filepath.Join(tempDir, "backend"),
						Tasks: map[string]config.Task{
							"build": {Command: []string{"go", "build"}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "workspace does not exist",
			config: &config.Config{
				Version: "1.0",
				Workspaces: map[string]config.Workspace{
					"nonexistent": {
						Path: filepath.Join(tempDir, "nonexistent"),
						Tasks: map[string]config.Task{
							"build": {Command: []string{"make"}},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.config, "")
			err := manager.ValidateWorkspaces()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspaces() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveWorkspacePath(t *testing.T) {
	manager := &Manager{
		basePath: "/test/base",
	}

	tests := []struct {
		name         string
		workspacePath string
		wantPrefix   string
	}{
		{
			name:         "relative path",
			workspacePath: "./frontend",
			wantPrefix:   "/test/base/frontend",
		},
		{
			name:         "absolute path",
			workspacePath: "/absolute/path",
			wantPrefix:   "/absolute/path",
		},
		{
			name:         "relative parent path",
			workspacePath: "../sibling",
			wantPrefix:   "/test/sibling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.resolveWorkspacePath(tt.workspacePath)
			if err != nil {
				t.Errorf("resolveWorkspacePath() error = %v", err)
				return
			}
			if !contains(result, tt.wantPrefix) {
				t.Errorf("resolveWorkspacePath() = %v, want to contain %v", result, tt.wantPrefix)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}