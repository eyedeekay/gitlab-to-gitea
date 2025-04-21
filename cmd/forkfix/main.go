package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// Repository represents a Gitea repository record
type Repository struct {
	ID        int64
	Name      string
	OwnerName string
	IsFork    bool
	ForkID    sql.NullInt64
}

func main() {
	// Define command line flags
	dbPath := flag.String("db", "", "Path to gitea.db (required)")
	forkID := flag.Int64("fork", 0, "ID of the repository to mark as fork (required)")
	originalID := flag.Int64("original", 0, "ID of the original repository (required)")
	unsetFork := flag.Bool("unset", false, "Unset fork relationship instead of setting it")
	listRepos := flag.Bool("list", false, "List all repositories in the database")
	backup := flag.Bool("backup", true, "Create a backup of the database before making changes")
	help := flag.Bool("help", false, "Show help")

	flag.Parse()

	// Display help if requested or if no arguments provided
	if *help || flag.NFlag() == 0 {
		printUsage()
		return
	}

	// List repositories if requested
	if *listRepos {
		if *dbPath == "" {
			log.Fatal("Database path (-db) is required")
		}
		listRepositories(*dbPath)
		return
	}

	// Validate required arguments
	if *dbPath == "" {
		log.Fatal("Database path (-db) is required")
	}

	if !*unsetFork && (*forkID == 0 || *originalID == 0) {
		log.Fatal("Both fork ID (-fork) and original repository ID (-original) are required")
	}

	if *unsetFork && *forkID == 0 {
		log.Fatal("Fork ID (-fork) is required to unset a fork relationship")
	}

	// Create backup if requested
	if *backup {
		backupFile := backupDatabase(*dbPath)
		fmt.Printf("Created backup at: %s\n", backupFile)
	}

	// Open database connection
	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify the database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Verify repositories exist before making changes
	if !*unsetFork {
		verifyRepository(db, *forkID, "fork")
		verifyRepository(db, *originalID, "original")
	} else {
		verifyRepository(db, *forkID, "fork")
	}

	// Execute the operation
	if !*unsetFork {
		// Set fork relationship
		updateForkRelationship(db, *forkID, *originalID)
		fmt.Printf("Repository ID %d is now marked as a fork of repository ID %d\n", *forkID, *originalID)
	} else {
		// Unset fork relationship
		unsetForkRelationship(db, *forkID)
		fmt.Printf("Repository ID %d is no longer marked as a fork\n", *forkID)
	}
}

// printUsage displays the help information
func printUsage() {
	fmt.Println("Gitea Repository Fork Relationship Fixer")
	fmt.Println("\nThis tool helps set, update, or remove fork relationships between repositories in a Gitea instance using SQLite.")
	fmt.Println("\nUsage:")
	fmt.Println("  Set a fork relationship:")
	fmt.Println("    gitea-fork-fixer -db /path/to/gitea.db -fork 123 -original 456")
	fmt.Println("\n  Remove a fork relationship:")
	fmt.Println("    gitea-fork-fixer -db /path/to/gitea.db -fork 123 -unset")
	fmt.Println("\n  List all repositories:")
	fmt.Println("    gitea-fork-fixer -db /path/to/gitea.db -list")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
	fmt.Println("\nNOTE: Always restart Gitea after making changes for them to take effect.")
}

// backupDatabase creates a backup of the database file
func backupDatabase(dbPath string) string {
	backupPath := dbPath + ".backup-" + fmt.Sprintf("%d", os.Getpid())

	// Read the original database file
	data, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("Failed to read database for backup: %v", err)
	}

	// Write to the backup file
	err = os.WriteFile(backupPath, data, 0o644)
	if err != nil {
		log.Fatalf("Failed to create backup: %v", err)
	}

	return backupPath
}

// verifyRepository checks if a repository exists in the database
func verifyRepository(db *sql.DB, repoID int64, repoType string) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM repository WHERE id = ?", repoID).Scan(&count)
	if err != nil {
		log.Fatalf("Failed to verify %s repository: %v", repoType, err)
	}

	if count == 0 {
		log.Fatalf("%s repository with ID %d does not exist", repoType, repoID)
	}
}

// updateForkRelationship sets a repository as a fork of another
func updateForkRelationship(db *sql.DB, forkID, originalID int64) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}

	_, err = tx.Exec("UPDATE repository SET fork_id = ?, is_fork = 1 WHERE id = ?", originalID, forkID)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to update fork relationship: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit changes: %v", err)
	}
}

// unsetForkRelationship removes a fork relationship
func unsetForkRelationship(db *sql.DB, forkID int64) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}

	_, err = tx.Exec("UPDATE repository SET fork_id = NULL, is_fork = 0 WHERE id = ?", forkID)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to unset fork relationship: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit changes: %v", err)
	}
}

// listRepositories displays all repositories in the database
func listRepositories(dbPath string) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT r.id, r.name, u.name as owner_name, r.is_fork, r.fork_id 
		FROM repository r
		JOIN user u ON r.owner_id = u.id
		ORDER BY r.id
	`)
	if err != nil {
		log.Fatalf("Failed to query repositories: %v", err)
	}
	defer rows.Close()

	fmt.Println("ID\tOwner/Name\tIs Fork\tFork of ID")
	fmt.Println("--\t----------\t-------\t----------")

	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.OwnerName, &repo.IsFork, &repo.ForkID); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}

		forkID := "NULL"
		if repo.ForkID.Valid {
			forkID = fmt.Sprintf("%d", repo.ForkID.Int64)
		}

		fmt.Printf("%d\t%s/%s\t%t\t%s\n",
			repo.ID, repo.OwnerName, repo.Name, repo.IsFork, forkID)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating through results: %v", err)
	}
}
