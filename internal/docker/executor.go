package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

func (e *Executor) Execute(ctx context.Context, execution *workspace.TaskExecution, stdoutWriter, stderrWriter io.Writer) *ExecutionResult {
	effectiveContainer := e.config.GetEffectiveContainer(execution.WorkspaceName, execution.TaskName)
	if effectiveContainer != "" {
		return e.executeInContainer(ctx, execution, effectiveContainer, stdoutWriter, stderrWriter)
	}
	return e.executeLocal(ctx, execution, stdoutWriter, stderrWriter)
}

func (e *Executor) executeInContainer(ctx context.Context, execution *workspace.TaskExecution, containerName string, stdoutWriter, stderrWriter io.Writer) *ExecutionResult {
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

	// Check if container is running before attempting to exec
	if !e.isContainerRunning(composeFile, containerName) {
		return &ExecutionResult{
			ExitCode: 1,
			Error: fmt.Errorf("container '%s' is not running\n\nTo start containers, run:\n  docker compose -f %s up -d %s\n\nOr start all containers:\n  docker compose -f %s up -d",
				containerName, composeFile, containerName, composeFile),
		}
	}

	// Use exec for running containers
	args := []string{
		"compose",
		"-f", composeFile,
		"exec",
		"-T",
	}

	env := e.buildEnvVars(execution)
	for key, value := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	workDir, isAbsolute := e.containerWorkDir(execution)
	if workDir != "" && workDir != "." && isAbsolute {
		args = append(args, "--workdir", workDir)
	}

	args = append(args, containerName)

	commandArgs := execution.Task.Command
	if workDir != "" && workDir != "." && !isAbsolute {
		shellCommand := buildShellCommand(workDir, execution.Task.Command)
		commandArgs = []string{"sh", "-lc", shellCommand}
	}

	args = append(args, commandArgs...)

	return e.runCommand(ctx, "docker", args, execution.AbsPath, env, stdoutWriter, stderrWriter)
}

func (e *Executor) executeLocal(ctx context.Context, execution *workspace.TaskExecution, stdoutWriter, stderrWriter io.Writer) *ExecutionResult {
	if len(execution.Task.Command) == 0 {
		return &ExecutionResult{
			ExitCode: 1,
			Error:    fmt.Errorf("no command specified"),
		}
	}

	command := execution.Task.Command[0]
	args := execution.Task.Command[1:]
	env := e.buildEnvVars(execution)

	return e.runCommand(ctx, command, args, execution.AbsPath, env, stdoutWriter, stderrWriter)
}

func (e *Executor) runCommand(ctx context.Context, command string, args []string, workDir string, env map[string]string, stdoutWriter, stderrWriter io.Writer) *ExecutionResult {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir

	envList := os.Environ()
	for key, value := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = envList

	var stdout, stderr bytes.Buffer
	if stdoutWriter != nil {
		cmd.Stdout = io.MultiWriter(&stdout, stdoutWriter)
	} else {
		cmd.Stdout = &stdout
	}

	if stderrWriter != nil {
		cmd.Stderr = io.MultiWriter(&stderr, stderrWriter)
	} else {
		cmd.Stderr = &stderr
	}

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

func (e *Executor) containerWorkDir(execution *workspace.TaskExecution) (string, bool) {
	workspacePath := execution.Workspace.Path
	if workspacePath == "" {
		return "", false
	}

	if filepath.IsAbs(workspacePath) {
		return filepath.ToSlash(workspacePath), true
	}

	relPath, err := filepath.Rel(e.workingDir, execution.AbsPath)
	if err == nil {
		relPath = filepath.ToSlash(relPath)
		if relPath == "" {
			return ".", false
		}
		return relPath, false
	}

	clean := strings.TrimPrefix(filepath.ToSlash(workspacePath), "./")
	if clean == "" {
		return ".", false
	}
	return clean, false
}

func buildShellCommand(workDir string, command []string) string {
	target := workDir
	if target == "" {
		target = "."
	}

	return fmt.Sprintf("cd %s && %s", shellEscape(target), shellJoin(command))
}

func shellJoin(args []string) string {
	if len(args) == 0 {
		return ""
	}

	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellEscape(arg)
	}
	return strings.Join(quoted, " ")
}

func shellEscape(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
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

func (e *Executor) isContainerRunning(composeFile, containerName string) bool {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--format", "json", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Parse the JSON output to check if container is running
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return false
	}

	// Simple check: if we got output, assume container exists and is running
	// The docker compose ps command returns info for running containers
	return true
}
