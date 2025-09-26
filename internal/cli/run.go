package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"doctrus/internal/config"
	"doctrus/internal/deps"
	"doctrus/internal/workspace"
)

const (
	// ANSI color reset sequence
	colorReset = "\033[0m"
)

var (
	forceBuild bool
	skipCache  bool
	parallel   int
	showDiff   bool
)

// TaskError represents an error from a failed task with its exit code
type TaskError struct {
	ExitCode int
	Message  string
}

func (e *TaskError) Error() string {
	return e.Message
}

// GetExitCode extracts the exit code from an error, returns 0 if not a TaskError
func GetExitCode(err error) int {
	var taskErr *TaskError
	if errors.As(err, &taskErr) {
		return taskErr.ExitCode
	}
	return 0
}

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [workspace:]task",
		Short: "Run a task in a workspace",
		Long: `Run a task in a workspace. If workspace is not specified, 
it will try to find the task in all workspaces.

Examples:
  doctrus run build                    # Run 'build' task in any workspace
  doctrus run frontend:build           # Run 'build' task in 'frontend' workspace  
  doctrus run frontend:test backend:test # Run multiple tasks`,
		Args: cobra.MinimumNArgs(1),
		RunE: runTask,
	}

	cmd.Flags().BoolVarP(&forceBuild, "force", "f", false, "Force rebuild, ignore cache")
	cmd.Flags().BoolVar(&skipCache, "skip-cache", false, "Skip cache completely")
	cmd.Flags().IntVarP(&parallel, "parallel", "p", 1, "Number of tasks to run in parallel")
	cmd.Flags().BoolVar(&showDiff, "show-diff", false, "Show what files changed since last run")

	return cmd
}

func runTask(cmd *cobra.Command, args []string) error {
	cli, err := newCLI()
	if err != nil {
		return err
	}

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		// Ensure terminal is in a clean state
		cli.cleanup()
	}()

	if err := cli.ensurePreRunCommands(ctx); err != nil {
		return err
	}

	runner := newTaskRunner(cli)

	for _, taskSpec := range args {
		if err := cli.runSingleTask(ctx, runner, taskSpec); err != nil {
			// Cancel context to ensure cleanup
			cancel()
			return fmt.Errorf("failed to run task %s: %w", taskSpec, err)
		}
	}

	return nil
}

func (c *CLI) runSingleTask(ctx context.Context, runner *taskRunner, taskSpec string) error {
	workspaceName, taskName := parseTaskSpec(taskSpec)

	if workspaceName == "" {
		found, err := c.findTaskInWorkspaces(taskName)
		if err != nil {
			return err
		}
		if len(found) == 0 {
			return fmt.Errorf("task %s not found in any workspace", taskName)
		}

		for _, ws := range found {
			if err := c.runTaskInWorkspace(ctx, runner, ws, taskName); err != nil {
				return err
			}
		}
		return nil
	}

	return c.runTaskInWorkspace(ctx, runner, workspaceName, taskName)
}

func (c *CLI) runTaskInWorkspace(ctx context.Context, runner *taskRunner, workspaceName, taskName string) error {
	executions, err := c.workspace.ResolveDependencies(workspaceName, taskName)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	if verbose {
		c.printf("Resolved execution order:\n")
		for i, exec := range executions {
			c.printf("  %d. %s:%s\n", i+1, exec.WorkspaceName, exec.TaskName)
		}
		c.printf("\n")
	}

	return runner.RunTask(ctx, workspaceName, taskName, false)
}

func (c *CLI) runExecution(ctx context.Context, execution *workspace.TaskExecution, showTaskPrefix bool) error {
	taskKey := fmt.Sprintf("%s:%s", execution.WorkspaceName, execution.TaskName)

	task := execution.Task
	taskVerbose := isTaskVerbose(task)
	detailedLogging := verbose || taskVerbose

	if len(task.Command) == 0 {
		c.printCompoundTask(execution, detailedLogging, isTaskParallel(task))
		return nil
	}

	header := fmt.Sprintf("▶ Running %s", taskKey)
	if detailedLogging {
		header += fmt.Sprintf(" in %s", execution.AbsPath)
	}
	c.printf("%s\n", header)

	var previousState *deps.TaskState
	if !skipCache && task.Cache {
		var err error
		previousState, err = c.cache.Get(taskKey)
		if err != nil && detailedLogging {
			c.printf("  Warning: failed to load cache: %v\n", err)
		} else if previousState != nil && detailedLogging {
			c.printf("  Cache found, checking for changes...\n")
		}
	}

	shouldRun := forceBuild || skipCache
	if !shouldRun {
		var err error
		shouldRun, err = c.tracker.ShouldRunTask(execution, previousState)
		if err != nil {
			return fmt.Errorf("failed to check if task should run: %w", err)
		}
	}

	if !shouldRun {
		c.printf("  ✓ Cached (no changes detected)\n")
		return nil
	}

	if showDiff && previousState != nil {
		changes, err := c.tracker.GetChangedInputs(execution, previousState)
		if err == nil && len(changes) > 0 {
			c.printf("  Changed inputs: %s\n", strings.Join(changes, ", "))
		}
	}

	if dryRun {
		c.printf("  Would run: %s\n", strings.Join(task.Command, " "))
		return nil
	}

	var stdoutWriter, stderrWriter io.Writer
	var stdoutFlusher, stderrFlusher interface{ Flush() error }
	if detailedLogging {
		stdoutWriter = &colorResetWriter{dest: newTaskLogWriter(c, taskKey, "stdout", showTaskPrefix)}
		stderrWriter = &colorResetWriter{dest: newTaskLogWriter(c, taskKey, "stderr", showTaskPrefix)}
		stdoutFlusher = stdoutWriter.(*colorResetWriter)
		stderrFlusher = stderrWriter.(*colorResetWriter)
	}

	startTime := time.Now()
	result := c.executor.Execute(ctx, execution, stdoutWriter, stderrWriter)
	duration := time.Since(startTime)

	// Ensure colors are reset after command execution
	if detailedLogging {
		// Flush the writers to reset colors properly
		if err := stdoutFlusher.Flush(); err != nil {
			c.printf("Warning: failed to flush stdout colors: %v\n", err)
		}
		if err := stderrFlusher.Flush(); err != nil {
			c.printf("Warning: failed to flush stderr colors: %v\n", err)
		}
	}

	if result.Error != nil && result.ExitCode == 0 {
		return fmt.Errorf("execution error: %w", result.Error)
	}

	success := result.ExitCode == 0

	if !success {
		if !detailedLogging && result.Stdout != "" {
			c.printBufferedOutput(taskKey, "stdout", result.Stdout, showTaskPrefix)
		}
		if !detailedLogging && result.Stderr != "" {
			c.printBufferedOutput(taskKey, "stderr", result.Stderr, showTaskPrefix)
		}
	}

	if success {
		c.printf("  ✓ Executed successfully in %v\n", duration.Round(time.Millisecond))
	} else {
		c.printf("  ✗ Failed with exit code %d in %v\n", result.ExitCode, duration.Round(time.Millisecond))
		return &TaskError{
			ExitCode: result.ExitCode,
			Message:  fmt.Sprintf("task failed with exit code %d", result.ExitCode),
		}
	}

	if task.Cache {
		taskState, err := c.tracker.ComputeTaskState(execution, success)
		if err != nil {
			if detailedLogging {
				c.printf("  Warning: failed to compute task state: %v\n", err)
			}
		} else {
			if err := c.cache.Set(taskKey, taskState, 0); err != nil {
				if detailedLogging {
					c.printf("  Warning: failed to cache task state: %v\n", err)
				}
			} else if detailedLogging {
				c.printf("  Cache updated for future runs\n")
			}
		}
	}

	return nil
}

func (c *CLI) printCompoundTask(execution *workspace.TaskExecution, detailed bool, isParallel bool) {
	taskKey := fmt.Sprintf("%s:%s", execution.WorkspaceName, execution.TaskName)
	mode := "dependencies only"
	if isParallel {
		mode = "parallel dependencies"
	}

	message := fmt.Sprintf("▶ Compound task %s (%s)", taskKey, mode)
	if detailed {
		message += fmt.Sprintf(" in %s", execution.AbsPath)
	}
	c.printf("%s\n", message)
	c.printf("  ✓ Dependencies completed\n")
}

func isTaskVerbose(task *config.Task) bool {
	if task == nil || task.Verbose == nil {
		return true
	}
	return *task.Verbose
}

func isTaskParallel(task *config.Task) bool {
	if task == nil || task.Parallel == nil {
		return false
	}
	return *task.Parallel
}

func parseTaskSpec(taskSpec string) (string, string) {
	parts := strings.Split(taskSpec, ":")
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func (c *CLI) findTaskInWorkspaces(taskName string) ([]string, error) {
	var found []string

	for workspaceName := range c.config.Workspaces {
		if _, exists := c.config.GetTask(workspaceName, taskName); exists {
			found = append(found, workspaceName)
		}
	}

	return found, nil
}

func (c *CLI) ensurePreRunCommands(ctx context.Context) error {
	if c.preRunExecuted {
		return nil
	}

	if len(c.config.Pre) == 0 {
		c.preRunExecuted = true
		return nil
	}

	for idx, pre := range c.config.Pre {
		cmdDisplay := strings.Join(pre.Command, " ")
		if pre.Description != "" {
			cmdDisplay = pre.Description
		}

		preVerbose := true
		if pre.Verbose != nil {
			preVerbose = *pre.Verbose
		}
		detailedLogging := verbose || preVerbose

		workingDir := pre.Dir
		if workingDir == "" {
			workingDir = c.basePath
		} else if !filepath.IsAbs(workingDir) {
			workingDir = filepath.Join(c.basePath, workingDir)
		}

		headline := fmt.Sprintf("▶ Pre-run %d/%d: %s", idx+1, len(c.config.Pre), cmdDisplay)
		if detailedLogging {
			headline += fmt.Sprintf(" (dir %s)", workingDir)
		}
		c.printf("%s\n", headline)

		if len(pre.Command) == 0 {
			return fmt.Errorf("pre[%d]: command is required", idx)
		}

		execCmd := exec.CommandContext(ctx, pre.Command[0], pre.Command[1:]...)
		execCmd.Dir = workingDir

		envList := os.Environ()
		for key, value := range pre.Env {
			envList = append(envList, fmt.Sprintf("%s=%s", key, value))
		}
		execCmd.Env = envList

		var stdoutBuf, stderrBuf bytes.Buffer
		execCmd.Stdout = &stdoutBuf
		execCmd.Stderr = &stderrBuf

		start := time.Now()
		err := execCmd.Run()
		duration := time.Since(start)

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		stdout := stdoutBuf.String()
		stderr := stderrBuf.String()

		if detailedLogging || err != nil {
			if stdout != "" {
				c.printf("  stdout:\n%s\n", indentOutput(stdout))
			}
			if stderr != "" {
				c.printf("  stderr:\n%s\n", indentOutput(stderr))
			}
		}

		// Ensure colors are reset after pre-run command execution
		c.printf("%s", colorReset)

		if err != nil {
			c.printf("  ✗ Failed with exit code %d in %v\n", exitCode, duration.Round(time.Millisecond))
			return &TaskError{
				ExitCode: exitCode,
				Message:  fmt.Sprintf("pre-run command %d failed: %v", idx+1, err),
			}
		}

		c.printf("  ✓ Completed in %v\n", duration.Round(time.Millisecond))
	}

	c.preRunExecuted = true
	return nil
}

func (c *CLI) printf(format string, args ...interface{}) {
	c.outputMu.Lock()
	defer c.outputMu.Unlock()
	fmt.Printf(format, args...)
}

// cleanup ensures the terminal is in a clean state
func (c *CLI) cleanup() {
	c.outputMu.Lock()
	defer c.outputMu.Unlock()
	// Reset colors and ensure we're at the beginning of a new line
	fmt.Printf("%s\n", colorReset)
}

type dependencySpec struct {
	workspace string
	task      string
}

type taskRunner struct {
	cli    *CLI
	mu     sync.Mutex
	states map[string]*taskState
}

type taskState struct {
	cond    *sync.Cond
	running bool
	done    bool
	err     error
}

func newTaskRunner(cli *CLI) *taskRunner {
	return &taskRunner{
		cli:    cli,
		states: make(map[string]*taskState),
	}
}

func (r *taskRunner) RunTask(ctx context.Context, workspaceName, taskName string, triggeredByCompound bool) error {
	taskKey := fmt.Sprintf("%s:%s", workspaceName, taskName)

	r.mu.Lock()
	if state, exists := r.states[taskKey]; exists {
		for state.running {
			state.cond.Wait()
		}
		err := state.err
		r.mu.Unlock()
		return err
	}

	state := &taskState{}
	state.cond = sync.NewCond(&r.mu)
	state.running = true
	r.states[taskKey] = state
	r.mu.Unlock()

	err := r.execute(ctx, workspaceName, taskName, triggeredByCompound)

	r.mu.Lock()
	state.running = false
	state.done = true
	state.err = err
	state.cond.Broadcast()
	r.mu.Unlock()

	return err
}

func (r *taskRunner) execute(ctx context.Context, workspaceName, taskName string, triggeredByCompound bool) error {
	execution, err := r.cli.workspace.ResolveTaskExecution(workspaceName, taskName)
	if err != nil {
		return err
	}

	deps, err := r.cli.collectDependencies(workspaceName, execution.Task)
	if err != nil {
		return err
	}

	if len(deps) > 0 {
		if isParallelCompound(execution.Task) {
			if err := r.runDependenciesParallel(ctx, deps, triggeredByCompound || len(execution.Task.Command) == 0); err != nil {
				return err
			}
		} else {
			childCompoundContext := triggeredByCompound || len(execution.Task.Command) == 0
			for _, dep := range deps {
				if err := r.RunTask(ctx, dep.workspace, dep.task, childCompoundContext); err != nil {
					return err
				}
			}
		}
	}

	return r.cli.runExecution(ctx, execution, triggeredByCompound)
}

func (r *taskRunner) runDependenciesParallel(ctx context.Context, deps []dependencySpec, triggeredByCompound bool) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(deps))

	for _, dep := range deps {
		dep := dep
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.RunTask(ctx, dep.workspace, dep.task, triggeredByCompound); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (c *CLI) collectDependencies(currentWorkspace string, task *config.Task) ([]dependencySpec, error) {
	var deps []dependencySpec

	for _, dep := range task.DependsOn {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}

		workspaceName := currentWorkspace
		taskName := dep

		parts := strings.Split(dep, ":")
		if len(parts) == 2 {
			workspaceName = parts[0]
			taskName = parts[1]
		} else if len(parts) > 2 {
			return nil, fmt.Errorf("invalid dependency format: %s", dep)
		}

		deps = append(deps, dependencySpec{workspace: workspaceName, task: taskName})
	}

	return deps, nil
}

func isParallelCompound(task *config.Task) bool {
	return len(task.Command) == 0 && isTaskParallel(task)
}

type taskLogWriter struct {
	cli         *CLI
	dest        io.Writer
	prefix      []byte
	showPrefix  bool
	atLineStart bool
}

// colorResetWriter ensures colors are reset after output
type colorResetWriter struct {
	dest io.Writer
}

func (w *colorResetWriter) Write(p []byte) (int, error) {
	n, err := w.dest.Write(p)
	// Don't reset colors after every write - this breaks colored output formatting
	// Colors will be reset when the writer is closed or flushed
	return n, err
}

// Flush ensures colors are reset and any buffered output is written
func (w *colorResetWriter) Flush() error {
	// Reset colors at the end of output
	_, err := w.dest.Write([]byte(colorReset))
	if err != nil {
		return fmt.Errorf("failed to write color reset sequence: %w", err)
	}
	return nil
}

// Close ensures colors are reset when the writer is closed
func (w *colorResetWriter) Close() error {
	// Reset colors when closing the writer
	_, err := w.dest.Write([]byte(colorReset))
	if err != nil {
		return fmt.Errorf("failed to write color reset sequence on close: %w", err)
	}
	return nil
}

func newTaskLogWriter(cli *CLI, taskKey, stream string, showPrefix bool) io.Writer {
	prefix := []byte(fmt.Sprintf("[%s][%s] ", taskKey, stream))
	return &taskLogWriter{
		cli:         cli,
		dest:        os.Stdout,
		prefix:      prefix,
		showPrefix:  showPrefix,
		atLineStart: true,
	}
}

func (w *taskLogWriter) Write(p []byte) (int, error) {
	w.cli.outputMu.Lock()
	defer w.cli.outputMu.Unlock()

	total := 0
	rest := p

	for len(rest) > 0 {
		if w.atLineStart && w.showPrefix {
			if _, err := w.dest.Write(w.prefix); err != nil {
				return total, err
			}
			w.atLineStart = false
		}

		newlineIndex := bytes.IndexByte(rest, '\n')
		if newlineIndex == -1 {
			written, err := w.dest.Write(rest)
			total += written
			return total, err
		}

		chunk := rest[:newlineIndex+1]
		written, err := w.dest.Write(chunk)
		total += written
		if err != nil {
			return total, err
		}
		w.atLineStart = true
		rest = rest[newlineIndex+1:]
	}

	return total, nil
}

func (c *CLI) printBufferedOutput(taskKey, stream, output string, showPrefix bool) {
	if strings.TrimSpace(output) == "" {
		return
	}
	writer := &colorResetWriter{dest: newTaskLogWriter(c, taskKey, stream, showPrefix)}
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	_, _ = writer.Write([]byte(output))
	// Flush to ensure colors are reset
	if err := writer.Flush(); err != nil {
		c.printf("Warning: failed to flush colors for %s: %v\n", stream, err)
	}
}

func indentOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}
