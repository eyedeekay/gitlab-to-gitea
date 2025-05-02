package main

import (
	"fmt"
	"time"

	"github.com/go-i2p/gitlab-to-gitea/gitea"
	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// Repository represents a Gitea repository with its owner and initial commit
type Repository struct {
	Owner             string `json:"owner"`
	Name              string `json:"name"`
	InitialCommitHash string `json:"initial_commit_hash"`
}

// Inconsistency represents a pair of repositories with naming inconsistencies
type Inconsistency struct {
	SourceRepo Repository `json:"source_repo"`
	TargetRepo Repository `json:"target_repo"`
}

// NameFixer handles the repository name inconsistency checking logic
type NameFixer struct {
	client      *gitea.Client
	sourceOwner string
	targetOwner string
}

// NewNameFixer creates a new NameFixer instance
func NewNameFixer(client *gitea.Client, sourceOwner, targetOwner string) *NameFixer {
	return &NameFixer{
		client:      client,
		sourceOwner: sourceOwner,
		targetOwner: targetOwner,
	}
}

// getOwnerRepositories fetches repositories for a specific owner with pagination
func (nf *NameFixer) getOwnerRepositories(owner string) ([]map[string]interface{}, error) {
	var allRepos []map[string]interface{}
	page := 1
	pageSize := 50 // Smaller page size to avoid timeout issues

	for {
		utils.PrintInfo(fmt.Sprintf("Fetching page %d of repositories for %s...", page, owner))

		// Try user repositories endpoint first
		var response interface{}
		endpoint := fmt.Sprintf("users/%s/repos?page=%d&limit=%d", owner, page, pageSize)
		err := nf.client.Get(endpoint, &response)
		// If that fails, try organization endpoint
		if err != nil {
			endpoint = fmt.Sprintf("orgs/%s/repos?page=%d&limit=%d", owner, page, pageSize)
			err = nf.client.Get(endpoint, &response)
			// If both fail, return error
			if err != nil {
				return nil, fmt.Errorf("failed to fetch repositories: %w", err)
			}
		}

		// Extract repository data
		var pageRepos []map[string]interface{}

		// Handle both array and object with data field response types
		if responseMap, ok := response.(map[string]interface{}); ok {
			if data, ok := responseMap["data"].([]interface{}); ok {
				// Handle data field containing repositories
				for _, repo := range data {
					if repoMap, ok := repo.(map[string]interface{}); ok {
						pageRepos = append(pageRepos, repoMap)
					}
				}
			}
		} else if repoArray, ok := response.([]map[string]interface{}); ok {
			// Direct array of repositories
			pageRepos = repoArray
		} else if repoIntfArray, ok := response.([]interface{}); ok {
			// Array of interfaces that should be maps
			for _, repo := range repoIntfArray {
				if repoMap, ok := repo.(map[string]interface{}); ok {
					pageRepos = append(pageRepos, repoMap)
				}
			}
		}

		// If no repos found, we've reached the end of pagination
		if len(pageRepos) == 0 {
			break
		}

		// Add repositories from this page
		allRepos = append(allRepos, pageRepos...)

		// If we got fewer repositories than page size, we've reached the end
		if len(pageRepos) < pageSize {
			break
		}

		// Move to next page
		page++

		// Add a small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	return allRepos, nil
}
