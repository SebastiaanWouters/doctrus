package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new doctrus project",
		Long:  "Create a sample doctrus.yml configuration file in the current directory",
		RunE:  initProject,
	}

	return cmd
}

func initProject(cmd *cobra.Command, args []string) error {
	configPath := "doctrus.yml"
	
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("doctrus.yml already exists")
	}

	sampleConfig := `version: "1.0"

workspaces:
  frontend:
    path: ./frontend
    container: frontend  # Optional: run tasks in this Docker container
    env:
      NODE_ENV: development
    tasks:
      install:
        command: ["npm", "install"]
        description: "Install frontend dependencies"
        inputs: ["package.json", "package-lock.json"]
        outputs: ["node_modules/**/*"]
        cache: true
      
      build:
        command: ["npm", "run", "build"]
        description: "Build frontend application"
        depends_on: ["install"]
        inputs: ["src/**/*", "public/**/*", "package.json"]
        outputs: ["dist/**/*"]
        cache: true
      
      test:
        command: ["npm", "test"]
        description: "Run frontend tests"
        depends_on: ["install"]
        inputs: ["src/**/*", "test/**/*"]
        cache: true

  backend:
    path: ./backend
    container: backend  # Optional: run tasks in this Docker container
    env:
      COMPOSER_CACHE_DIR: /tmp/composer-cache
    tasks:
      install:
        command: ["composer", "install"]
        description: "Install backend dependencies"
        inputs: ["composer.json", "composer.lock"]
        outputs: ["vendor/**/*"]
        cache: true
      
      test:
        command: ["./vendor/bin/phpunit"]
        description: "Run backend tests"
        depends_on: ["install"]
        inputs: ["src/**/*", "tests/**/*"]
        cache: true
      
      build:
        command: ["php", "artisan", "optimize"]
        description: "Build and optimize backend"
        depends_on: ["install", "frontend:build"]
        inputs: ["src/**/*", "config/**/*"]
        cache: true

# Optional Docker configuration
docker:
  compose_file: docker-compose.yml
`

	if err := os.WriteFile(configPath, []byte(sampleConfig), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	fmt.Printf("✓ Created %s\n", configPath)
	fmt.Println("\nSample configuration created! You can now:")
	fmt.Println("  1. Edit doctrus.yml to match your project structure")
	fmt.Println("  2. Run 'doctrus validate' to check your configuration")
	fmt.Println("  3. Run 'doctrus list' to see available tasks")
	fmt.Println("  4. Run 'doctrus run <task>' to execute tasks")

	return nil
}