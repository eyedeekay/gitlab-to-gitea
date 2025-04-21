// mentions.go

// Package utils provides utility functions used throughout the application
package utils

import (
	"regexp"
	"strings"
)

// mentionPattern matches @username mentions in text
var mentionPattern = regexp.MustCompile(`@([a-zA-Z0-9_\.-]+(?:\s+[a-zA-Z0-9_\.-]+)*)`)

// ExtractUserMentions extracts all @username mentions from text
func ExtractUserMentions(text string) []string {
	if text == "" {
		return []string{}
	}

	// Find all matches
	matches := mentionPattern.FindAllStringSubmatch(text, -1)

	// Extract the username part of each match
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			result = append(result, match[1])
		}
	}

	return result
}

// NormalizeMentions replaces all @mentions in text with their normalized versions
func NormalizeMentions(text string) string {
	if text == "" {
		return text
	}

	// Extract all mentions
	mentions := ExtractUserMentions(text)

	// Replace each mention with its normalized version
	result := text
	for _, mention := range mentions {
		normalized := NormalizeUsername(mention)
		result = strings.ReplaceAll(result, "@"+mention, "@"+normalized)
	}

	return result
}
