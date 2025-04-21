// naming.go

// Package utils provides utility functions used throughout the application
package utils

import (
	"regexp"
	"strings"
	"sync"
)

var (
	// usernameMap caches normalized usernames to avoid repeated normalization
	usernameMap     = make(map[string]string)
	usernameMutex   sync.RWMutex
	specialCharsReg = regexp.MustCompile(`[^a-zA-Z0-9_\.-]`)
)

// NormalizeUsername converts usernames to Gitea-compatible format
// by replacing spaces with underscores and removing special characters
func NormalizeUsername(username string) string {
	if username == "" {
		return username
	}

	// Check cache first
	usernameMutex.RLock()
	if cleanName, exists := usernameMap[username]; exists {
		usernameMutex.RUnlock()
		return cleanName
	}
	usernameMutex.RUnlock()

	var cleanName string
	// Handle special cases (reserved names, etc)
	if strings.ToLower(username) == "ghost" {
		cleanName = "ghost_user"
	} else {
		// Replace spaces with underscores
		cleanName = strings.ReplaceAll(username, " ", "_")
		// Remove special characters
		cleanName = specialCharsReg.ReplaceAllString(cleanName, "_")
	}

	// Store in cache
	usernameMutex.Lock()
	usernameMap[username] = cleanName
	usernameMutex.Unlock()

	return cleanName
}

// CleanName sanitizes a name for use in Gitea by replacing spaces with underscores
// and removing special characters
func CleanName(name string) string {
	// Replace spaces with underscores
	newName := strings.ReplaceAll(name, " ", "_")
	// Replace special chars with hyphens
	newName = specialCharsReg.ReplaceAllString(newName, "-")

	// Handle special case for "plugins"
	if strings.ToLower(newName) == "plugins" {
		return newName + "-user"
	}

	return newName
}
