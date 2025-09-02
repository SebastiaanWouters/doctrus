package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"doctrus/internal/config"
	"doctrus/internal/workspace"
)

type Executor struct {
	config     *config.Config
	workingDir string
}

type ExecutionResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Error    error
}

func NewExecutor(cfg *config.Config, workingDir string) *Executor {
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	return &Executor{
		config:     cfg,
		workingDir: workingDir,
	}
}

func (e *Executor) Execute(ctx context.Context, execution *workspace.TaskExecution) *ExecutionResult {
	effectiveContainer := e.config.GetEffectiveContainer(execution.WorkspaceName, execution.TaskName)
	if effectiveContainer != "" {
		return e.executeInContainer(ctx, execution, effectiveContainer)
	}
	return e.executeLocal(ctx, execution)
}

func (e *Executor) executeInContainer(ctx context.Context, execution *workspace.TaskExecution, containerName string) *ExecutionResult {
	dockerConfig := e.config.GetEffectiveDockerConfig(execution.WorkspaceName, execution.TaskName)
	composeFile := dockerConfig.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	if !filepath.IsAbs(composeFile) {
		composeFile = filepath.Join(e.workingDir, composeFile)
	}

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return &ExecutionResult{
			ExitCode: 1,
			Error:    fmt.Errorf("docker-compose file not found: %s", composeFile),
		}
	}

	// Always use exec for existing containers
	args := []string{
		"compose",
		"-f", composeFile,
		"exec",
		"-T",
	}

	// Mount cache directory if it exists
	cacheDir := e.getCacheDir()
	if _, err := os.Stat(cacheDir); err == nil {
		// Mount cache directory to same path inside container
		args = append(args, "-v", fmt.Sprintf("%s:%s", cacheDir, cacheDir))
	}

	env := e.buildEnvVars(execution)
	for key, value := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, containerName)
	args = append(args, execution.Task.Command...)

	return e.runCommand(ctx, "docker", args, execution.AbsPath, env)
}

func (e *Executor) executeLocal(ctx context.Context, execution *workspace.TaskExecution) *ExecutionResult {
	if len(execution.Task.Command) == 0 {
		return &ExecutionResult{
			ExitCode: 1,
			Error:    fmt.Errorf("no command specified"),
		}
	}

	command := execution.Task.Command[0]
	args := execution.Task.Command[1:]
	env := e.buildEnvVars(execution)

	return e.runCommand(ctx, command, args, execution.AbsPath, env)
}

func (e *Executor) runCommand(ctx context.Context, command string, args []string, workDir string, env map[string]string) *ExecutionResult {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir

	envList := os.Environ()
	for key, value := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = envList

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Error:    err,
	}
}

func (e *Executor) buildEnvVars(execution *workspace.TaskExecution) map[string]string {
	env := make(map[string]string)

	for key, value := range execution.Workspace.Env {
		env[key] = value
	}

	for key, value := range execution.Task.Env {
		env[key] = value
	}

	// Set cache directory relative to working directory
	effectiveContainer := e.config.GetEffectiveContainer(execution.WorkspaceName, execution.TaskName)
	if effectiveContainer != "" {
		cacheDir := e.getCacheDir()
		env["DOCTRUS_CACHE_DIR"] = cacheDir
	}

	return env
}

func (e *Executor) IsDockerComposeAvailable() bool {
	cmd := exec.Command("docker", "compose", "version")
	return cmd.Run() == nil
}

func (e *Executor) GetRunningContainers() ([]string, error) {
	composeFile := e.config.Docker.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	if !filepath.IsAbs(composeFile) {
		composeFile = filepath.Join(e.workingDir, composeFile)
	}

	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get running containers: %w", err)
	}

	var containers []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			containers = append(containers, line)
		}
	}

	return containers, nil
}

func (e *Executor) getCacheDir() string {
	// Cache directory is now relative to the working directory
	return filepath.Join(e.workingDir, ".doctrus", "cache")
}
