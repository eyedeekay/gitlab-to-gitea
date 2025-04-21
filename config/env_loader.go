// env_loader.go

// Package config handles application configuration through environment variables
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from .env file if present
func LoadEnv() error {
	// Check if .env file exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		// No .env file, that's fine - we'll use environment variables directly
		return nil
	}

	// Load from .env file
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	return nil
}

// MustLoadEnv loads environment variables and exits on error
func MustLoadEnv() {
	if err := LoadEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load environment: %v\n", err)
		os.Exit(1)
	}
}

// SetEnvWithDefault sets an environment variable if it's not already set
func SetEnvWithDefault(key, defaultValue string) {
	if _, exists := os.LookupEnv(key); !exists {
		os.Setenv(key, defaultValue)
	}
}

// SetEnvDefaults sets default values for environment variables
// that are not already set
func SetEnvDefaults() {
	defaults := map[string]string{
		"MIGRATION_STATE_FILE": "migration_state.json",
		"RESUME_MIGRATION":     "true",
	}

	for key, value := range defaults {
		SetEnvWithDefault(key, value)
	}
}
