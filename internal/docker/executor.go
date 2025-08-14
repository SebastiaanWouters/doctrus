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
	if execution.Workspace.Container != "" {
		return e.executeInContainer(ctx, execution)
	}
	return e.executeLocal(ctx, execution)
}

func (e *Executor) executeInContainer(ctx context.Context, execution *workspace.TaskExecution) *ExecutionResult {
	composeFile := e.config.Docker.ComposeFile
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

	args := []string{
		"compose",
		"-f", composeFile,
		"exec",
		"-T",
	}

	workspaceRelPath, err := filepath.Rel(e.workingDir, execution.AbsPath)
	if err != nil {
		workspaceRelPath = execution.AbsPath
	}

	args = append(args, "-w", fmt.Sprintf("/workspace/%s", workspaceRelPath))

	env := e.buildEnvVars(execution)
	for key, value := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, execution.Workspace.Container)
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