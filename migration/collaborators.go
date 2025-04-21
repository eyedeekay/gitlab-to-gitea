// collaborators.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// collaboratorAddRequest represents the data needed to add a collaborator to a repository
type collaboratorAddRequest struct {
	Permission string `json:"permission"`
}

// importProjectCollaborators imports project collaborators to Gitea
func (m *Manager) importProjectCollaborators(
	collaborators []*gitlab.ProjectMember,
	project *gitlab.Project,
) error {
	ownerInfo, err := m.getOwner(project)
	if err != nil {
		return fmt.Errorf("failed to get owner info: %w", err)
	}

	ownerUsername := ownerInfo["username"].(string)
	ownerType := ownerInfo["type"].(string)
	repoName := utils.CleanName(project.Name)

	for _, collaborator := range collaborators {
		cleanUsername := utils.NormalizeUsername(collaborator.Username)

		// Skip if the collaborator is the owner
		if ownerType == "user" && ownerUsername == cleanUsername {
			continue
		}

		// Map GitLab access levels to Gitea permissions
		permission := "read"
		if collaborator.AccessLevel >= 30 { // Developer+
			permission = "write"
		}
		if collaborator.AccessLevel >= 40 { // Maintainer+
			permission = "admin"
		}

		// Check if collaborator already exists
		exists, err := m.collaboratorExists(ownerUsername, repoName, cleanUsername)
		if err != nil {
			utils.PrintWarning(fmt.Sprintf("Error checking if collaborator %s exists: %v", cleanUsername, err))
			continue
		}

		if exists {
			utils.PrintWarning(fmt.Sprintf("Collaborator %s already exists for repo %s, skipping!", cleanUsername, repoName))
			continue
		}

		// Add collaborator
		colReq := collaboratorAddRequest{
			Permission: permission,
		}

		err = m.giteaClient.Put(
			fmt.Sprintf("/repos/%s/%s/collaborators/%s", ownerUsername, repoName, cleanUsername),
			colReq,
			nil,
		)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Failed to add collaborator %s: %v", cleanUsername, err))
			continue
		}

		utils.PrintInfo(fmt.Sprintf("Collaborator %s added to %s as %s!", collaborator.Username, repoName, permission))
	}

	return nil
}

// collaboratorExists checks if a user is a collaborator on a repository
func (m *Manager) collaboratorExists(owner, repo, username string) (bool, error) {
	err := m.giteaClient.Get(fmt.Sprintf("/repos/%s/%s/collaborators/%s", owner, repo, username), nil)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking if collaborator exists: %w", err)
	}
	return true, nil
}
