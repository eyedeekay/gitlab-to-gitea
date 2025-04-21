// users.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// userCreateRequest represents the data needed to create a user in Gitea
type userCreateRequest struct {
	Email      string `json:"email"`
	FullName   string `json:"full_name"`
	LoginName  string `json:"login_name"`
	Password   string `json:"password"`
	SendNotify bool   `json:"send_notify"`
	SourceID   int    `json:"source_id"`
	Username   string `json:"username"`
}

// ImportUser imports a single GitLab user to Gitea
func (m *Manager) ImportUser(user *gitlab.User, notify bool) error {
	// Normalize username
	cleanUsername := utils.NormalizeUsername(user.Username)

	// Check if user already exists
	if exists, err := m.userExists(cleanUsername); err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	} else if exists {
		utils.PrintWarning(fmt.Sprintf("User %s already exists as %s in Gitea, skipping!", user.Username, cleanUsername))
		return nil
	}

	// Generate temporary password
	tmpPassword := generateTempPassword()

	// Determine email (use placeholder if not available)
	email := fmt.Sprintf("%s@placeholder-migration.local", cleanUsername)
	if user.Email != "" {
		email = user.Email
	}

	// Create user request
	userReq := userCreateRequest{
		Email:      email,
		FullName:   user.Name,
		LoginName:  cleanUsername,
		Password:   tmpPassword,
		SendNotify: notify,
		SourceID:   0, // local user
		Username:   cleanUsername,
	}

	// Debug what endpoint we're calling and with what method
	utils.PrintInfo("Attempting to create user via: POST /admin/users\n")

	var result map[string]interface{}
	err := m.giteaClient.Post("/admin/users", userReq, &result)
	if err != nil {
		// Try the alternative user creation endpoint if the first one failed
		utils.PrintInfo("First attempt failed, trying alternative endpoint\n")
		err = m.giteaClient.Post("/api/v1/admin/users", userReq, &result)
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Username, err)
		}
	}

	utils.PrintInfo(fmt.Sprintf("User %s created as %s, temporary password: %s", user.Username, cleanUsername, tmpPassword))

	utils.PrintHeader("Importing SSH keys...")
	// Import user's SSH keys
	keys, err := m.gitlabClient.GetUserKeys(user.ID)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Failed to fetch keys for user %s: %v", user.Username, err))
	} else {
		utils.PrintInfo(fmt.Sprintf("Found %d keys for user %s", len(keys), user.Username))
		for _, key := range keys {
			utils.PrintInfo(fmt.Sprintf("Importing key %s for user %s", key.Title, cleanUsername))
			if err := m.importUserKey(cleanUsername, key); err != nil {
				utils.PrintWarning(fmt.Sprintf("Failed to import key for user %s: %v", user.Username, err))
			}
			utils.PrintInfo(fmt.Sprintf("Key %s imported for user %s", key.Title, cleanUsername))
		}
		utils.PrintSuccess(fmt.Sprintf("Imported %d keys for user %s", len(keys), cleanUsername))
	}

	return nil
}

// ImportPlaceholderUser creates a placeholder user when mentioned user doesn't exist
func (m *Manager) ImportPlaceholderUser(username string) error {
	cleanUsername := utils.NormalizeUsername(username)

	exists, err := m.userExists(cleanUsername)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if exists {
		utils.PrintWarning(fmt.Sprintf("User %s already exists as %s in Gitea, skipping placeholder creation", username, cleanUsername))
		return nil
	}

	tmpPassword := generateTempPassword()

	// Create user request
	userReq := userCreateRequest{
		Email:      fmt.Sprintf("%s@placeholder-migration.local", cleanUsername),
		FullName:   username, // Keep original name for display
		LoginName:  cleanUsername,
		Password:   tmpPassword,
		SendNotify: false,
		SourceID:   0, // local user
		Username:   cleanUsername,
	}

	var result map[string]interface{}
	err = m.giteaClient.Post("/admin/users", userReq, &result)
	if err != nil {
		return fmt.Errorf("failed to create placeholder user %s: %w", username, err)
	}

	utils.PrintInfo(fmt.Sprintf("Placeholder user %s created as %s", username, cleanUsername))
	return nil
}

// importUserKey imports a user's SSH key to Gitea
func (m *Manager) importUserKey(username string, key *gitlab.SSHKey) error {
	// Check if key already exists
	var existingKeys []map[string]interface{}
	err := m.giteaClient.Get(fmt.Sprintf("/users/%s/keys", username), &existingKeys)
	if err != nil {
		return fmt.Errorf("failed to get existing keys: %w", err)
	}

	// Check if key with same title already exists
	for _, existingKey := range existingKeys {
		if existingKey["title"] == key.Title {
			utils.PrintWarning(fmt.Sprintf("Key %s already exists for user %s, skipping", key.Title, username))
			return nil
		}
	}

	// Create key request
	keyReq := map[string]string{
		"key":   key.Key,
		"title": key.Title,
	}

	// Call Gitea API to create key
	var result map[string]interface{}
	err = m.giteaClient.Post(fmt.Sprintf("/admin/users/%s/keys", username), keyReq, &result)
	if err != nil {
		return fmt.Errorf("failed to create key %s: %w", key.Title, err)
	}

	utils.PrintInfo(fmt.Sprintf("Key %s imported for user %s", key.Title, username))
	return nil
}

// userExists checks if a user exists in Gitea
func (m *Manager) userExists(username string) (bool, error) {
	var user map[string]interface{}
	err := m.giteaClient.Get("/users/"+username, &user)
	if err != nil {
		// If we get an error, assume user doesn't exist
		// But only if the error contains "not found" or similar messages
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking if user exists: %w", err)
	}
	return true, nil
}

// generateTempPassword creates a random password for new users
func generateTempPassword() string {
	const (
		chars  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		pwdLen = 12
		prefix = "Tmp1!"
	)

	// Initialize random source
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate random part of password
	result := make([]byte, pwdLen)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}

	return prefix + string(result)
}

// isNotFoundError checks if an error is a 404 Not Found error
func isNotFoundError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "404") ||
		strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "does not exist"))
}
