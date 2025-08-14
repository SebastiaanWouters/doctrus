package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage task cache",
		Long:  "Manage the task execution cache",
	}

	cmd.AddCommand(
		newCacheClearCommand(),
		newCacheStatsCommand(),
		newCacheListCommand(),
	)

	return cmd
}

func newCacheClearCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear [workspace]",
		Short: "Clear cache",
		Long:  "Clear all cache or cache for a specific workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE:  clearCache,
	}

	return cmd
}

func newCacheStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show cache statistics",
		Long:  "Display cache usage statistics",
		RunE:  showCacheStats,
	}

	return cmd
}

func newCacheListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cached tasks",
		Long:  "List all cached task executions",
		RunE:  listCachedTasks,
	}

	return cmd
}

func clearCache(cmd *cobra.Command, args []string) error {
	cli, err := newCLI()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		workspaceName := args[0]
		if err := cli.cache.InvalidateWorkspace(workspaceName); err != nil {
			return fmt.Errorf("failed to clear workspace cache: %w", err)
		}
		fmt.Printf("✓ Cleared cache for workspace: %s\n", workspaceName)
	} else {
		if err := cli.cache.Clear(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Println("✓ Cleared all cache")
	}

	return nil
}

func showCacheStats(cmd *cobra.Command, args []string) error {
	cli, err := newCLI()
	if err != nil {
		return err
	}

	stats, err := cli.cache.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats: %w", err)
	}

	fmt.Println("Cache Statistics:")
	fmt.Printf("  Directory: %v\n", stats["cache_dir"])
	fmt.Printf("  Total entries: %v\n", stats["total_entries"])
	fmt.Printf("  Expired entries: %v\n", stats["expired_entries"])

	if size, ok := stats["cache_dir_size"]; ok {
		fmt.Printf("  Directory size: %d bytes\n", size)
	}

	return nil
}

func listCachedTasks(cmd *cobra.Command, args []string) error {
	cli, err := newCLI()
	if err != nil {
		return err
	}

	entries, err := cli.cache.List()
	if err != nil {
		return fmt.Errorf("failed to list cache entries: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No cached tasks found")
		return nil
	}

	fmt.Printf("Cached tasks (%d):\n\n", len(entries))

	for _, entry := range entries {
		fmt.Printf("Task: %s\n", entry.TaskKey)
		fmt.Printf("  Created: %s", entry.CreatedAt.Format(time.RFC3339))
		
		age := time.Since(entry.CreatedAt)
		fmt.Printf(" (%s ago)\n", formatDuration(age))
		
		if entry.TTL > 0 {
			remaining := entry.TTL - age
			if remaining > 0 {
				fmt.Printf("  Expires in: %s\n", formatDuration(remaining))
			} else {
				fmt.Printf("  Status: expired\n")
			}
		} else {
			fmt.Printf("  TTL: never expires\n")
		}
		
		if entry.State != nil {
			fmt.Printf("  Success: %t\n", entry.State.Success)
			fmt.Printf("  Inputs: %d files\n", len(entry.State.InputHashes))
			fmt.Printf("  Outputs: %d files\n", len(entry.State.Outputs))
		}
		
		fmt.Println()
	}

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}