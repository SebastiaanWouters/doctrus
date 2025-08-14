package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspaces and tasks",
		Long: `List all available workspaces and their tasks.

Examples:
  doctrus list                # List all workspaces and tasks
  doctrus list frontend       # List tasks in frontend workspace`,
		Args: cobra.MaximumNArgs(1),
		RunE: listWorkspaces,
	}

	return cmd
}

func listWorkspaces(cmd *cobra.Command, args []string) error {
	cli, err := newCLI()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		return cli.listWorkspaceTasks(args[0])
	}

	return cli.listAllWorkspaces()
}

func (c *CLI) listAllWorkspaces() error {
	workspaces := c.workspace.GetWorkspaces()
	
	if len(workspaces) == 0 {
		fmt.Println("No workspaces found")
		return nil
	}

	fmt.Printf("Available workspaces (%d):\n\n", len(workspaces))

	for _, workspaceName := range workspaces {
		workspace, _ := c.config.GetWorkspace(workspaceName)
		fmt.Printf("📁 %s", workspaceName)
		if workspace.Path != "" {
			fmt.Printf(" (%s)", workspace.Path)
		}
		if workspace.Container != "" {
			fmt.Printf(" [%s]", workspace.Container)
		}
		fmt.Println()

		tasks, _ := c.workspace.GetTasks(workspaceName)
		if len(tasks) > 0 {
			for _, taskName := range tasks {
				task, _ := c.config.GetTask(workspaceName, taskName)
				fmt.Printf("  ├─ %s", taskName)
				if task.Description != "" {
					fmt.Printf(": %s", task.Description)
				}
				if len(task.DependsOn) > 0 {
					fmt.Printf(" (depends: %s)", strings.Join(task.DependsOn, ", "))
				}
				fmt.Println()
			}
		}
		fmt.Println()
	}

	return nil
}

func (c *CLI) listWorkspaceTasks(workspaceName string) error {
	workspace, exists := c.config.GetWorkspace(workspaceName)
	if !exists {
		return fmt.Errorf("workspace %s not found", workspaceName)
	}

	tasks, err := c.workspace.GetTasks(workspaceName)
	if err != nil {
		return err
	}

	fmt.Printf("Workspace: %s", workspaceName)
	if workspace.Path != "" {
		fmt.Printf(" (%s)", workspace.Path)
	}
	if workspace.Container != "" {
		fmt.Printf(" [%s]", workspace.Container)
	}
	fmt.Println()

	if len(tasks) == 0 {
		fmt.Println("  No tasks found")
		return nil
	}

	fmt.Printf("\nTasks (%d):\n", len(tasks))
	for _, taskName := range tasks {
		task, _ := c.config.GetTask(workspaceName, taskName)
		fmt.Printf("  %s", taskName)
		if task.Description != "" {
			fmt.Printf(": %s", task.Description)
		}
		fmt.Println()

		if verbose {
			fmt.Printf("    Command: %s\n", strings.Join(task.Command, " "))
			if len(task.DependsOn) > 0 {
				fmt.Printf("    Depends on: %s\n", strings.Join(task.DependsOn, ", "))
			}
			if len(task.Inputs) > 0 {
				fmt.Printf("    Inputs: %s\n", strings.Join(task.Inputs, ", "))
			}
			if len(task.Outputs) > 0 {
				fmt.Printf("    Outputs: %s\n", strings.Join(task.Outputs, ", "))
			}
			if task.Cache {
				fmt.Printf("    Cache: enabled\n")
			}
			fmt.Println()
		}
	}

	return nil
}