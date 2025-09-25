package main

import (
	"fmt"
	"os"

	"doctrus/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		// Check if the error contains an exit code from a failed task
		if exitCode := cli.GetExitCode(err); exitCode != 0 {
			os.Exit(exitCode)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}