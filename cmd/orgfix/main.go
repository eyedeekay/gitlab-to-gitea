package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Repository represents a Gitea repository record
type Repository struct {
	ID        int64
	Name      string
	OwnerID   int64
	OwnerName string
	IsFork    bool
	ForkID    sql.NullInt64
}

// User represents a Gitea user or organization
type User struct {
	ID   int64
	Name string
	Type int // 1 for individual, 2 for organization
}

const (
	UserTypeIndividual   = 0
	UserTypeOrganization = 1
)

func main() {
	// Define command line flags
	dbPath := flag.String("db", "", "Path to gitea.db (required)")
	orgName := flag.String("org", "", "Name of the organization to match against (required)")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without making changes")
	backup := flag.Bool("backup", true, "Create a backup of the database before making changes")
	verbose := flag.Bool("verbose", false, "Show detailed output")
	help := flag.Bool("help", false, "Show help")
	listOrgs := flag.Bool("list-orgs", false, "List all organizations in the database")

	flag.Parse()

	if *listOrgs {
		if *dbPath == "" {
			log.Fatal("Database path (-db) is required")
		}
		db, err := sql.Open("sqlite3", *dbPath)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		// Verify database connection
		if err := db.Ping(); err != nil {
			log.Fatalf("Failed to connect to database (possibly locked or invalid): %v", err)
		}

		listOrganizations(db)
		return
	}

	// Display help if requested or if no arguments provided
	if *help || flag.NFlag() == 0 {
		printUsage()
		return
	}

	// Validate required arguments
	if *dbPath == "" {
		log.Fatal("Database path (-db) is required")
	}

	if *orgName == "" {
		log.Fatal("Organization name (-org) is required")
	}

	// Create backup if requested and not in dry run mode
	if *backup && !*dryRun {
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

	// Get organization ID
	orgID, err := getOrganizationID(db, *orgName)
	if err != nil {
		log.Fatalf("Failed to find organization '%s': %v", *orgName, err)
	}

	// Get repositories owned by the organization
	orgRepos, err := getOrganizationRepos(db, orgID)
	if err != nil {
		log.Fatalf("Failed to get organization repositories: %v", err)
	}

	if len(orgRepos) == 0 {
		fmt.Printf("No repositories found for organization '%s'\n", *orgName)
		return
	}

	fmt.Printf("Found %d repositories in organization '%s'\n", len(orgRepos), *orgName)

	// Build a map of organization repositories by name for quick lookup
	orgRepoMap := make(map[string]*Repository)
	for i, repo := range orgRepos {
		orgRepoMap[repo.Name] = &orgRepos[i]
		if *verbose {
			fmt.Printf("Org repo: %s (ID: %d)\n", repo.Name, repo.ID)
		}
	}

	// Get all user repositories
	userRepos, err := getUserRepos(db)
	if err != nil {
		log.Fatalf("Failed to get user repositories: %v", err)
	}

	fmt.Printf("Found %d repositories in user namespaces\n", len(userRepos))

	// Find matches and setup fork relationships
	matchCount := 0
	updateCount := 0

	for _, userRepo := range userRepos {
		// Skip if the repo is already a fork
		if userRepo.IsFork && userRepo.ForkID.Valid {
			continue
		}

		// Check if there's a matching org repo with the same name
		if orgRepo, exists := orgRepoMap[userRepo.Name]; exists {
			matchCount++

			if *verbose || *dryRun {
				fmt.Printf("Match found: %s/%s (ID: %d) -> %s/%s (ID: %d)\n",
					userRepo.OwnerName, userRepo.Name, userRepo.ID,
					*orgName, orgRepo.Name, orgRepo.ID)
			}

			// Update the fork relationship if not in dry run mode
			if !*dryRun {
				err := updateForkRelationship(db, userRepo.ID, orgRepo.ID)
				if err != nil {
					log.Printf("Error updating fork relationship for %s/%s: %v",
						userRepo.OwnerName, userRepo.Name, err)
				} else {
					updateCount++
				}
			}
		}
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("- Matching repositories found: %d\n", matchCount)

	if *dryRun {
		fmt.Printf("- Dry run mode: no changes made\n")
	} else {
		fmt.Printf("- Fork relationships updated: %d\n", updateCount)
	}

	if !*dryRun {
		fmt.Printf("\nNOTE: You should restart your Gitea instance for changes to take effect.\n")
	}
}

// printUsage displays the help information
func printUsage() {
	fmt.Println("Gitea Organization Fork Matcher")
	fmt.Println("\nThis tool identifies repositories with the same name in both organization and user")
	fmt.Println("namespaces, then establishes proper fork relationships in Gitea's SQLite database.")
	fmt.Println("\nUsage:")
	fmt.Println("  gitea-org-fork-matcher -db /path/to/gitea.db -org YourOrgName")
	fmt.Println("\nOptions:")
	flag.PrintDefaults()
	fmt.Println("\nNOTE: Always restart Gitea after making changes for them to take effect.")
}

// backupDatabase creates a backup of the database file
func backupDatabase(dbPath string) string {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := dbPath + ".backup-" + timestamp

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

// getOrganizationID retrieves the ID of an organization by name
func getOrganizationID(db *sql.DB, orgName string) (int64, error) {
	var id int64
	query := `SELECT id FROM user WHERE name = ? AND type = ?`
	err := db.QueryRow(query, orgName, UserTypeOrganization).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("organization '%s' not found", orgName)
		}
		return 0, err
	}
	return id, nil
}

// getOrganizationRepos retrieves all repositories owned by the given organization
func getOrganizationRepos(db *sql.DB, orgID int64) ([]Repository, error) {
	query := `
		SELECT r.id, r.name, r.owner_id, u.name AS owner_name, r.is_fork, r.fork_id
		FROM repository r
		JOIN user u ON r.owner_id = u.id
		WHERE r.owner_id = ?
		ORDER BY r.name
	`

	rows, err := db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.OwnerID, &repo.OwnerName, &repo.IsFork, &repo.ForkID); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return repos, nil
}

// getUserRepos retrieves all repositories owned by individual users (not organizations)
func getUserRepos(db *sql.DB) ([]Repository, error) {
	query := `
		SELECT r.id, r.name, r.owner_id, u.name AS owner_name, r.is_fork, r.fork_id
		FROM repository r
		JOIN user u ON r.owner_id = u.id
		WHERE u.type = ?
		ORDER BY r.name, u.name
	`

	rows, err := db.Query(query, UserTypeIndividual)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.OwnerID, &repo.OwnerName, &repo.IsFork, &repo.ForkID); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return repos, nil
}

// updateForkRelationship sets a repository as a fork of another
func updateForkRelationship(db *sql.DB, forkID, originalID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	_, err = tx.Exec("UPDATE repository SET fork_id = ?, is_fork = 1 WHERE id = ?", originalID, forkID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update fork relationship: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit changes: %v", err)
	}

	return nil
}

// Enhanced organization listing function
func listOrganizations(db *sql.DB) {
	// First, let's check if we can access the user table at all
	var tableExists int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='user'").Scan(&tableExists)
	if err != nil {
		log.Fatalf("Error checking for user table: %v", err)
	}

	if tableExists == 0 {
		log.Fatalf("The 'user' table does not exist in this database")
	}

	// Let's examine the schema of the user table
	fmt.Println("User table schema:")
	rows, err := db.Query("PRAGMA table_info(user)")
	if err != nil {
		log.Fatalf("Failed to query user table schema: %v", err)
	}

	columns := make(map[string]string)
	fmt.Println("Column\tType")
	fmt.Println("------\t----")
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			log.Fatalf("Failed to scan schema row: %v", err)
		}
		columns[name] = ctype
		fmt.Printf("%s\t%s\n", name, ctype)
	}
	rows.Close()

	// Check if the type column exists
	if _, hasType := columns["type"]; !hasType {
		// Check if it might be called something else, like "user_type"
		altTypeColumns := []string{"user_type", "kind", "organization_type"}
		for _, colName := range altTypeColumns {
			if _, has := columns[colName]; has {
				fmt.Printf("\nFound potential type column: '%s'\n", colName)
			}
		}
		log.Fatalf("The 'type' column does not exist in user table")
	}

	// Let's see what type values actually exist in the database
	fmt.Println("\nDistinct user types in database:")
	typeRows, err := db.Query("SELECT DISTINCT type, COUNT(*) FROM user GROUP BY type")
	if err != nil {
		log.Fatalf("Failed to query user types: %v", err)
	}

	fmt.Println("Type\tCount")
	fmt.Println("----\t-----")
	typeFound := false
	for typeRows.Next() {
		var userType, count int
		if err := typeRows.Scan(&userType, &count); err != nil {
			log.Fatalf("Failed to scan type row: %v", err)
		}
		typeFound = true
		fmt.Printf("%d\t%d\n", userType, count)
	}
	typeRows.Close()

	if !typeFound {
		fmt.Println("No user type data found - table might be empty or type field has NULL values")
	}

	// Now try to get the list of organizations regardless of type
	fmt.Println("\nAttempting to list all users (potential organizations):")
	userRows, err := db.Query("SELECT id, name, type FROM user ORDER BY name LIMIT 20")
	if err != nil {
		log.Fatalf("Failed to query users: %v", err)
	}

	fmt.Println("ID\tName\tType")
	fmt.Println("--\t----\t----")
	userFound := false
	for userRows.Next() {
		var id, userType int
		var name string
		if err := userRows.Scan(&id, &name, &userType); err != nil {
			log.Fatalf("Failed to scan user row: %v", err)
		}
		userFound = true
		fmt.Printf("%d\t%s\t%d\n", id, name, userType)
	}
	userRows.Close()

	if !userFound {
		fmt.Println("No users found in the database")
	}
}
