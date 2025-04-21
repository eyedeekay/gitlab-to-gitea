// config.go

// Package config handles application configuration through environment variables
package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds all configuration parameters for the migration
type Config struct {
	GitLabURL          string
	GitLabToken        string
	GitLabAdminUser    string
	GitLabAdminPass    string
	GiteaURL           string
	GiteaToken         string
	MigrationStateFile string
	ResumeMigration    bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	gitlabURL := os.Getenv("GITLAB_URL")
	if gitlabURL == "" {
		return nil, errors.New("GITLAB_URL environment variable is required")
	}

	gitlabToken := os.Getenv("GITLAB_TOKEN")
	if gitlabToken == "" {
		return nil, errors.New("GITLAB_TOKEN environment variable is required")
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		return nil, errors.New("GITEA_URL environment variable is required")
	}

	giteaToken := os.Getenv("GITEA_TOKEN")
	if giteaToken == "" {
		return nil, errors.New("GITEA_TOKEN environment variable is required")
	}

	// Optional values with defaults
	migrationStateFile := os.Getenv("MIGRATION_STATE_FILE")
	if migrationStateFile == "" {
		migrationStateFile = "migration_state.json"
	}

	resumeMigrationStr := os.Getenv("RESUME_MIGRATION")
	resumeMigration := true // default
	if resumeMigrationStr != "" {
		var err error
		resumeMigration, err = strconv.ParseBool(resumeMigrationStr)
		if err != nil {
			return nil, errors.New("RESUME_MIGRATION must be a boolean value")
		}
	}

	return &Config{
		GitLabURL:          gitlabURL,
		GitLabToken:        gitlabToken,
		GitLabAdminUser:    os.Getenv("GITLAB_ADMIN_USER"),
		GitLabAdminPass:    os.Getenv("GITLAB_ADMIN_PASS"),
		GiteaURL:           giteaURL,
		GiteaToken:         giteaToken,
		MigrationStateFile: migrationStateFile,
		ResumeMigration:    resumeMigration,
	}, nil
}
