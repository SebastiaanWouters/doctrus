package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"doctrus/internal/config"
)

type Manager struct {
	config   *config.Config
	basePath string
}

type TaskExecution struct {
	WorkspaceName string
	TaskName      string
	Task          *config.Task
	Workspace     *config.Workspace
	AbsPath       string
}

func NewManager(cfg *config.Config, basePath string) *Manager {
	if basePath == "" {
		basePath, _ = os.Getwd()
	}
	return &Manager{
		config:   cfg,
		basePath: basePath,
	}
}

func (m *Manager) GetWorkspaces() []string {
	var names []string
	for name := range m.config.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *Manager) GetTasks(workspaceName string) ([]string, error) {
	workspace, exists := m.config.GetWorkspace(workspaceName)
	if !exists {
		return nil, fmt.Errorf("workspace %s not found", workspaceName)
	}

	var names []string
	for name := range workspace.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (m *Manager) GetAllTasks() map[string][]string {
	result := make(map[string][]string)
	for workspaceName := range m.config.Workspaces {
		tasks, _ := m.GetTasks(workspaceName)
		result[workspaceName] = tasks
	}
	return result
}

func (m *Manager) ResolveTaskExecution(workspaceName, taskName string) (*TaskExecution, error) {
	workspace, exists := m.config.GetWorkspace(workspaceName)
	if !exists {
		return nil, fmt.Errorf("workspace %s not found", workspaceName)
	}

	task, exists := m.config.GetTask(workspaceName, taskName)
	if !exists {
		return nil, fmt.Errorf("task %s not found in workspace %s", taskName, workspaceName)
	}

	absPath, err := m.resolveWorkspacePath(workspace.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workspace path: %w", err)
	}

	return &TaskExecution{
		WorkspaceName: workspaceName,
		TaskName:      taskName,
		Task:          task,
		Workspace:     workspace,
		AbsPath:       absPath,
	}, nil
}

func (m *Manager) ResolveDependencies(workspaceName, taskName string) ([]*TaskExecution, error) {
	_, exists := m.config.GetTask(workspaceName, taskName)
	if !exists {
		return nil, fmt.Errorf("task %s not found in workspace %s", taskName, workspaceName)
	}

	var executions []*TaskExecution
	visited := make(map[string]bool)
	processed := make(map[string]bool)
	
	if err := m.resolveDependenciesRecursive(workspaceName, taskName, &executions, visited, processed); err != nil {
		return nil, err
	}

	return executions, nil
}

func (m *Manager) resolveDependenciesRecursive(workspaceName, taskName string, executions *[]*TaskExecution, visited map[string]bool, processed map[string]bool) error {
	key := fmt.Sprintf("%s:%s", workspaceName, taskName)
	
	// If already processed, skip to avoid duplicates
	if processed[key] {
		return nil
	}
	
	// Check for circular dependencies
	if visited[key] {
		return fmt.Errorf("circular dependency detected: %s", key)
	}
	visited[key] = true
	defer delete(visited, key) // Clear after processing to allow diamond dependencies

	task, exists := m.config.GetTask(workspaceName, taskName)
	if !exists {
		return fmt.Errorf("task %s not found in workspace %s", taskName, workspaceName)
	}

	for _, dep := range task.DependsOn {
		parts := strings.Split(dep, ":")
		var depWorkspace, depTask string
		
		if len(parts) == 1 {
			depWorkspace = workspaceName
			depTask = parts[0]
		} else if len(parts) == 2 {
			depWorkspace = parts[0]
			depTask = parts[1]
		} else {
			return fmt.Errorf("invalid dependency format: %s", dep)
		}

		if err := m.resolveDependenciesRecursive(depWorkspace, depTask, executions, visited, processed); err != nil {
			return err
		}
	}

	execution, err := m.ResolveTaskExecution(workspaceName, taskName)
	if err != nil {
		return err
	}

	*executions = append(*executions, execution)
	processed[key] = true
	return nil
}

func (m *Manager) resolveWorkspacePath(workspacePath string) (string, error) {
	if filepath.IsAbs(workspacePath) {
		return workspacePath, nil
	}
	return filepath.Abs(filepath.Join(m.basePath, workspacePath))
}

func (m *Manager) ValidateWorkspaces() error {
	for name, workspace := range m.config.Workspaces {
		absPath, err := m.resolveWorkspacePath(workspace.Path)
		if err != nil {
			return fmt.Errorf("workspace %s: failed to resolve path: %w", name, err)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return fmt.Errorf("workspace %s: path does not exist: %s", name, absPath)
		}
	}
	return nil
}