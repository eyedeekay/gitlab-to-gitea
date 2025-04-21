// action.go

// Package gitea provides functionality for working with Gitea actions
package gitea

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// ImportCommitActions imports Git commits to Gitea action database
func ImportCommitActions(logFilePath string) error {
	// Get required environment variables
	userID := getEnvInt("USERID", 1)
	repoID := getEnvInt("REPOID", 1)
	branch := getEnvWithDefault("BRANCH", "master")

	// Database connection
	dbHost := getEnvWithDefault("DB_HOST", "localhost")
	dbUser := getEnvWithDefault("DB_USER", "user")
	dbPass := getEnvWithDefault("DB_PASS", "password")
	dbName := getEnvWithDefault("DB_NAME", "gitea")

	// Connect to database
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPass, dbHost, dbName))
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Prepare SQL statement
	stmt, err := db.Prepare("INSERT INTO action (user_id, op_type, act_user_id, repo_id, comment_id, ref_name, is_private, created_unix) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare SQL statement: %w", err)
	}
	defer stmt.Close()

	// Open commit log file
	file, err := os.Open(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to open commit log file: %w", err)
	}
	defer file.Close()

	// Process each line
	var importCount int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 3 {
			return fmt.Errorf("invalid line format: %s", line)
		}

		// Parse commit timestamp
		timestamp, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse timestamp '%s': %w", parts[1], err)
		}

		// Insert action record
		_, err = stmt.Exec(
			userID,    // user_id
			5,         // op_type - 5 means commit
			userID,    // act_user_id
			repoID,    // repo_id
			0,         // comment_id
			branch,    // ref_name
			1,         // is_private
			timestamp, // created_unix
		)
		if err != nil {
			return fmt.Errorf("failed to insert action record: %w", err)
		}
		importCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading commit log file: %w", err)
	}

	fmt.Printf("%d actions inserted.\n", importCount)
	return nil
}

// Helper functions for environment variables
func getEnvInt(key string, defaultValue int) int {
	if val, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvWithDefault(key, defaultValue string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultValue
}
