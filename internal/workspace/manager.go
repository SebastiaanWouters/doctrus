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

	// Build dependency graph
	graph, indegrees, err := m.buildDependencyGraph(workspaceName, taskName)
	if err != nil {
		return nil, err
	}

	// Perform topological sort using Kahn's algorithm
	executions, err := m.topologicalSort(graph, indegrees)
	if err != nil {
		return nil, err
	}

	return executions, nil
}

// buildDependencyGraph constructs a dependency graph for the given task.
// Uses BFS traversal to discover all dependencies and builds:
// - Adjacency list: maps each task to its dependents (tasks that depend on it)
// - Indegree map: counts how many dependencies each task has
// This enables efficient topological sorting with Kahn's algorithm.
func (m *Manager) buildDependencyGraph(workspaceName, taskName string) (map[string][]string, map[string]int, error) {
	graph := make(map[string][]string) // task -> list of tasks that depend on it
	indegrees := make(map[string]int)  // task -> number of dependencies
	visited := make(map[string]bool)   // to avoid processing the same task multiple times

	// Start with the target task
	queue := []string{fmt.Sprintf("%s:%s", workspaceName, taskName)}

	for len(queue) > 0 {
		currentKey := queue[0]
		queue = queue[1:]

		if visited[currentKey] {
			continue
		}
		visited[currentKey] = true

		// Parse the current task key
		parts := strings.Split(currentKey, ":")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid task key format: %s", currentKey)
		}
		currWorkspace, currTask := parts[0], parts[1]

		// Get the task definition
		task, exists := m.config.GetTask(currWorkspace, currTask)
		if !exists {
			return nil, nil, fmt.Errorf("task %s not found in workspace %s", currTask, currWorkspace)
		}

		// Initialize indegree for this task if not already done
		if _, exists := indegrees[currentKey]; !exists {
			indegrees[currentKey] = 0
		}

		// Process dependencies
		for _, dep := range task.DependsOn {
			var depWorkspace, depTask string

			// Parse dependency specification
			depParts := strings.Split(dep, ":")
			if len(depParts) == 1 {
				// Same workspace dependency
				depWorkspace = currWorkspace
				depTask = depParts[0]
			} else if len(depParts) == 2 {
				// Cross-workspace dependency
				depWorkspace = depParts[0]
				depTask = depParts[1]
			} else {
				return nil, nil, fmt.Errorf("invalid dependency format: %s", dep)
			}

			depKey := fmt.Sprintf("%s:%s", depWorkspace, depTask)

			// Verify dependency exists
			if _, exists := m.config.GetTask(depWorkspace, depTask); !exists {
				return nil, nil, fmt.Errorf("dependency %s not found", depKey)
			}

			// Add edge: dependency -> current task (dependency must run before current)
			graph[depKey] = append(graph[depKey], currentKey)

			// Increment indegree of current task
			indegrees[currentKey]++

			// Add dependency to queue if not visited
			if !visited[depKey] {
				queue = append(queue, depKey)
			}
		}
	}

	return graph, indegrees, nil
}

// topologicalSort performs topological sorting using Kahn's algorithm.
// This algorithm ensures:
// 1. Tasks are executed in dependency order (dependencies first)
// 2. Each task executes exactly once (handles diamond dependencies)
// 3. Circular dependencies are detected and reported as errors
//
// Algorithm:
// - Start with tasks that have no dependencies (indegree 0)
// - Process tasks in order, reducing indegree of their dependents
// - Tasks become available when their indegree reaches 0
// - If cycles exist, some tasks will never reach indegree 0
func (m *Manager) topologicalSort(graph map[string][]string, indegrees map[string]int) ([]*TaskExecution, error) {
	var result []*TaskExecution
	queue := make([]string, 0)

	// Find all tasks with no dependencies (indegree 0)
	for task, degree := range indegrees {
		if degree == 0 {
			queue = append(queue, task)
		}
	}

	// Process queue
	processedCount := 0
	totalTasks := len(indegrees)

	for len(queue) > 0 {
		// Sort queue for deterministic ordering
		sort.Strings(queue)

		// Dequeue first task
		currentKey := queue[0]
		queue = queue[1:]

		// Create task execution
		parts := strings.Split(currentKey, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid task key: %s", currentKey)
		}

		execution, err := m.ResolveTaskExecution(parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to resolve task execution for %s: %w", currentKey, err)
		}

		result = append(result, execution)
		processedCount++

		// Update indegrees of dependent tasks
		for _, dependent := range graph[currentKey] {
			indegrees[dependent]--
			if indegrees[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if processedCount != totalTasks {
		return nil, fmt.Errorf("circular dependency detected in dependency graph")
	}

	return result, nil
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
