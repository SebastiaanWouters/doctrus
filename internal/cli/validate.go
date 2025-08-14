package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		Long:  "Validate the doctrus configuration file and workspace setup",
		RunE:  validateConfig,
	}

	return cmd
}

func validateConfig(cmd *cobra.Command, args []string) error {
	cli, err := newCLI()
	if err != nil {
		return err
	}

	fmt.Println("✓ Configuration file is valid")

	workspaces := cli.workspace.GetWorkspaces()
	fmt.Printf("✓ Found %d workspace(s)\n", len(workspaces))

	for _, workspaceName := range workspaces {
		workspace, _ := cli.config.GetWorkspace(workspaceName)
		fmt.Printf("  📁 %s (%s)", workspaceName, workspace.Path)
		
		if workspace.Container != "" {
			fmt.Printf(" [%s]", workspace.Container)
			
			if !cli.executor.IsDockerComposeAvailable() {
				fmt.Printf(" ⚠️  Docker Compose not available")
			}
		}
		fmt.Println()

		tasks, _ := cli.workspace.GetTasks(workspaceName)
		fmt.Printf("    Tasks: %d\n", len(tasks))

		for _, taskName := range tasks {
			task, _ := cli.config.GetTask(workspaceName, taskName)
			if len(task.DependsOn) > 0 {
				for _, dep := range task.DependsOn {
					if err := cli.validateDependency(workspaceName, dep); err != nil {
						fmt.Printf("    ⚠️  %s dependency issue: %v\n", taskName, err)
					}
				}
			}
		}
	}

	if cli.executor.IsDockerComposeAvailable() {
		fmt.Println("✓ Docker Compose is available")
		
		containers, err := cli.executor.GetRunningContainers()
		if err != nil {
			fmt.Printf("⚠️  Could not check running containers: %v\n", err)
		} else if len(containers) > 0 {
			fmt.Printf("✓ Found %d running container(s)\n", len(containers))
		}
	} else {
		fmt.Println("⚠️  Docker Compose not available (tasks with containers will fail)")
	}

	stats, err := cli.cache.GetStats()
	if err == nil {
		fmt.Printf("✓ Cache directory: %v (%v entries)\n", stats["cache_dir"], stats["total_entries"])
	}

	fmt.Println("\n✅ Validation completed successfully!")
	
	return nil
}

func (c *CLI) validateDependency(currentWorkspace, dependency string) error {
	parts := splitDependency(dependency)
	workspaceName := parts[0]
	taskName := parts[1]
	
	if workspaceName == "" {
		workspaceName = currentWorkspace
	}

	if _, exists := c.config.GetWorkspace(workspaceName); !exists {
		return fmt.Errorf("workspace %s not found", workspaceName)
	}

	if _, exists := c.config.GetTask(workspaceName, taskName); !exists {
		return fmt.Errorf("task %s not found in workspace %s", taskName, workspaceName)
	}

	return nil
}

func splitDependency(dependency string) [2]string {
	if idx := strings.Index(dependency, ":"); idx != -1 {
		return [2]string{dependency[:idx], dependency[idx+1:]}
	}
	return [2]string{"", dependency}
}