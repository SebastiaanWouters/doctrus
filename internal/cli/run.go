package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	
	"doctrus/internal/deps"
	"doctrus/internal/workspace"
)

var (
	forceBuild     bool
	skipCache      bool
	parallel       int
	showDiff       bool
)

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

	ctx := context.Background()

	for _, taskSpec := range args {
		if err := cli.runSingleTask(ctx, taskSpec); err != nil {
			return fmt.Errorf("failed to run task %s: %w", taskSpec, err)
		}
	}

	return nil
}

func (c *CLI) runSingleTask(ctx context.Context, taskSpec string) error {
	workspaceName, taskName := parseTaskSpec(taskSpec)

	if workspaceName == "" {
		found, err := c.findTaskInWorkspaces(taskName)
		if err != nil {
			return err
		}
		if len(found) == 0 {
			return fmt.Errorf("task %s not found in any workspace", taskName)
		}
		
		// Run task in all workspaces where it's found
		for _, ws := range found {
			if err := c.runTaskInWorkspace(ctx, ws, taskName); err != nil {
				return err
			}
		}
		return nil
	}

	return c.runTaskInWorkspace(ctx, workspaceName, taskName)
}

func (c *CLI) runTaskInWorkspace(ctx context.Context, workspaceName, taskName string) error {
	executions, err := c.workspace.ResolveDependencies(workspaceName, taskName)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	if verbose {
		fmt.Printf("Resolved execution order:\n")
		for i, exec := range executions {
			fmt.Printf("  %d. %s:%s\n", i+1, exec.WorkspaceName, exec.TaskName)
		}
		fmt.Println()
	}

	for _, execution := range executions {
		if err := c.runExecution(ctx, execution); err != nil {
			return fmt.Errorf("failed to execute %s:%s: %w", execution.WorkspaceName, execution.TaskName, err)
		}
	}

	return nil
}

func (c *CLI) runExecution(ctx context.Context, execution *workspace.TaskExecution) error {
	taskKey := fmt.Sprintf("%s:%s", execution.WorkspaceName, execution.TaskName)
	
	fmt.Printf("▶ Running %s", taskKey)
	if verbose {
		fmt.Printf(" in %s", execution.AbsPath)
	}
	fmt.Println()

	var previousState *deps.TaskState
	if !skipCache && execution.Task.Cache {
		var err error
		previousState, err = c.cache.Get(taskKey)
		if err != nil && verbose {
			fmt.Printf("  Warning: failed to load cache: %v\n", err)
		} else if previousState != nil && verbose {
			fmt.Printf("  Cache found, checking for changes...\n")
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
		fmt.Printf("  ✓ Cached (no changes detected)\n")
		return nil
	}

	if showDiff && previousState != nil {
		changes, err := c.tracker.GetChangedInputs(execution, previousState)
		if err == nil && len(changes) > 0 {
			fmt.Printf("  Changed inputs: %s\n", strings.Join(changes, ", "))
		}
	}

	if dryRun {
		fmt.Printf("  Would run: %s\n", strings.Join(execution.Task.Command, " "))
		return nil
	}

	startTime := time.Now()
	result := c.executor.Execute(ctx, execution)
	duration := time.Since(startTime)

	if result.Error != nil && result.ExitCode == 0 {
		return fmt.Errorf("execution error: %w", result.Error)
	}

	success := result.ExitCode == 0

	if verbose || !success {
		if result.Stdout != "" {
			fmt.Printf("  stdout:\n%s\n", indentOutput(result.Stdout))
		}
		if result.Stderr != "" {
			fmt.Printf("  stderr:\n%s\n", indentOutput(result.Stderr))
		}
	}

	if success {
		fmt.Printf("  ✓ Executed successfully in %v\n", duration.Round(time.Millisecond))
	} else {
		fmt.Printf("  ✗ Failed with exit code %d in %v\n", result.ExitCode, duration.Round(time.Millisecond))
		return fmt.Errorf("task failed with exit code %d", result.ExitCode)
	}

	if execution.Task.Cache {
		taskState, err := c.tracker.ComputeTaskState(execution, success)
		if err != nil {
			if verbose {
				fmt.Printf("  Warning: failed to compute task state: %v\n", err)
			}
		} else {
			if err := c.cache.Set(taskKey, taskState, 0); err != nil {
				if verbose {
					fmt.Printf("  Warning: failed to cache task state: %v\n", err)
				}
			} else if verbose {
				fmt.Printf("  Cache updated for future runs\n")
			}
		}
	}

	return nil
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

func indentOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}