// repositories.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// repositoryMigrateRequest represents the data needed to migrate a repository to Gitea
type repositoryMigrateRequest struct {
	AuthPassword string `json:"auth_password"`
	AuthUsername string `json:"auth_username"`
	CloneAddr    string `json:"clone_addr"`
	Description  string `json:"description"`
	Mirror       bool   `json:"mirror"`
	Private      bool   `json:"private"`
	RepoName     string `json:"repo_name"`
	UID          int    `json:"uid"`
}

// ImportProject imports a GitLab project to Gitea
// ImportProject imports a GitLab project to Gitea
func (m *Manager) ImportProject(project *gitlab.Project) error {
	cleanName := utils.CleanName(project.Name)

	utils.PrintInfo(fmt.Sprintf("Importing project %s from owner %s", cleanName, project.Namespace.Name))

	// Get the owner information first, so we use the correct name format
	ownerInfo, err := m.getOwner(project)
	if err != nil {
		return fmt.Errorf("failed to get project owner: %w", err)
	}

	// Get the correct owner username from the result
	owner, ok := ownerInfo["username"].(string)
	if !ok || owner == "" {
		return fmt.Errorf("failed to get valid username for project owner")
	}

	utils.PrintInfo(fmt.Sprintf("Using owner %s for project %s", owner, cleanName))

	// Check if repository already exists
	if exists, err := m.repoExists(owner, cleanName); err != nil {
		return fmt.Errorf("failed to check if repository exists: %w", err)
	} else if exists {
		utils.PrintWarning(fmt.Sprintf("Project %s already exists in Gitea, skipping repository creation!", cleanName))
	} else {
		// Prepare clone URL
		cloneURL := project.HTTPURLToRepo
		if m.config.GitLabAdminUser == "" && m.config.GitLabAdminPass == "" {
			cloneURL = project.SSHURLToRepo
		}

		// Determine visibility
		private := project.Visibility == "private" || project.Visibility == "internal"

		// Create migration request
		migrateReq := repositoryMigrateRequest{
			AuthPassword: m.config.GitLabAdminPass,
			AuthUsername: m.config.GitLabAdminUser,
			CloneAddr:    cloneURL,
			Description:  project.Description,
			Mirror:       false,
			Private:      private,
			RepoName:     cleanName,
			UID:          int(ownerInfo["id"].(float64)),
		}

		// Call Gitea API to migrate repository
		var result map[string]interface{}
		err = m.giteaClient.Post("/repos/migrate", migrateReq, &result)
		if err != nil {
			return fmt.Errorf("failed to migrate repository %s: %w", cleanName, err)
		}

		utils.PrintInfo(fmt.Sprintf("Project %s imported!", cleanName))
	}

	// Process collaborators
	collaborators, err := m.gitlabClient.GetProjectMembers(project.ID)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching collaborators for project %s: %v", project.Name, err))
	} else {
		utils.PrintInfo(fmt.Sprintf("Found %d collaborators for project %s", len(collaborators), cleanName))
		if err := m.importProjectCollaborators(collaborators, project); err != nil {
			utils.PrintWarning(fmt.Sprintf("Error importing collaborators: %v", err))
		}
	}

	// Process labels
	labels, err := m.gitlabClient.GetProjectLabels(project.ID)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching labels for project %s: %v", project.Name, err))
	} else {
		utils.PrintInfo(fmt.Sprintf("Found %d labels for project %s", len(labels), cleanName))
		if err := m.importProjectLabels(labels, owner, cleanName); err != nil {
			utils.PrintWarning(fmt.Sprintf("Error importing labels: %v", err))
		}
	}

	// Process milestones
	milestones, err := m.gitlabClient.GetProjectMilestones(project.ID)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching milestones for project %s: %v", project.Name, err))
	} else {
		utils.PrintInfo(fmt.Sprintf("Found %d milestones for project %s", len(milestones), cleanName))
		if err := m.importProjectMilestones(milestones, owner, cleanName); err != nil {
			utils.PrintWarning(fmt.Sprintf("Error importing milestones: %v", err))
		}
	}

	// Process issues
	issues, err := m.gitlabClient.GetProjectIssues(project.ID)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching issues for project %s: %v", project.Name, err))
	} else {
		utils.PrintInfo(fmt.Sprintf("Found %d issues for project %s", len(issues), cleanName))

		// Ensure all mentioned users exist in Gitea
		m.ensureMentionedUsersExist(issues)

		if err := m.importProjectIssues(issues, owner, cleanName, project.ID); err != nil {
			utils.PrintWarning(fmt.Sprintf("Error importing issues: %v", err))
		}
	}

	return nil
}

// getOwner retrieves the user or organization info for a project
func (m *Manager) getOwner(project *gitlab.Project) (map[string]interface{}, error) {
	namespacePath := utils.NormalizeUsername(project.Namespace.Path)

	// Try to get as a user first
	var result map[string]interface{}
	err := m.giteaClient.Get("/users/"+namespacePath, &result)
	if err == nil && result != nil {
		// Verify required fields exist
		if username, ok := result["username"].(string); ok && username != "" {
			return result, nil
		}
	}

	// Try to get as an organization
	orgName := utils.CleanName(project.Namespace.Name)
	err = m.giteaClient.Get("/orgs/"+orgName, &result)
	if err == nil && result != nil {
		// Verify required fields exist
		if username, ok := result["username"].(string); ok && username != "" {
			return result, nil
		}
	}

	// Create a placeholder user instead of failing
	utils.PrintWarning(fmt.Sprintf("Could not find owner for project %s, creating placeholder user", project.Name))
	if err := m.ImportPlaceholderUser(namespacePath); err != nil {
		return nil, fmt.Errorf("failed to create placeholder user: %w", err)
	}

	// Try to get the newly created user
	err = m.giteaClient.Get("/users/"+namespacePath, &result)
	if err == nil && result != nil {
		return result, nil
	}

	return nil, fmt.Errorf("failed to find or create owner for project: %s", project.Path)
}

// repoExists checks if a repository exists in Gitea
func (m *Manager) repoExists(owner, repo string) (bool, error) {
	var repository map[string]interface{}
	err := m.giteaClient.Get(fmt.Sprintf("/repos/%s/%s", owner, repo), &repository)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking if repository exists: %w", err)
	}
	return true, nil
}

// ensureMentionedUsersExist makes sure all users mentioned in issues exist in Gitea
func (m *Manager) ensureMentionedUsersExist(issues []*gitlab.Issue) {
	mentionedUsers := make(map[string]struct{})

	// Extract mentions from issues
	for _, issue := range issues {
		if issue.Description != "" {
			for _, mention := range utils.ExtractUserMentions(issue.Description) {
				mentionedUsers[mention] = struct{}{}
			}
		}
	}

	// Create placeholder users for any missing mentioned users
	for username := range mentionedUsers {
		exists, err := m.userExists(utils.NormalizeUsername(username))
		if err != nil {
			utils.PrintWarning(fmt.Sprintf("Error checking if user %s exists: %v", username, err))
			continue
		}

		if !exists {
			if err := m.ImportPlaceholderUser(username); err != nil {
				utils.PrintWarning(fmt.Sprintf("Failed to create placeholder user %s: %v", username, err))
			}
		}
	}
}
