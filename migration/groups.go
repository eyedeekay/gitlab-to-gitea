// groups.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// organizationCreateRequest represents the data needed to create an organization in Gitea
type organizationCreateRequest struct {
	Description string `json:"description"`
	FullName    string `json:"full_name"`
	Location    string `json:"location"`
	Username    string `json:"username"`
	Website     string `json:"website"`
}

// ImportGroup imports a single GitLab group to Gitea as an organization
func (m *Manager) ImportGroup(group *gitlab.Group) error {
	cleanName := utils.CleanName(group.Name)

	utils.PrintInfo(fmt.Sprintf("Importing group %s...", cleanName))

	// Check if organization already exists
	if exists, err := m.organizationExists(cleanName); err != nil {
		return fmt.Errorf("failed to check if organization exists: %w", err)
	} else if exists {
		utils.PrintWarning(fmt.Sprintf("Group %s already exists in Gitea, skipping!", cleanName))
		return nil
	}

	// Get group members
	members, err := m.gitlabClient.GetGroupMembers(group.ID)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching members for group %s: %v", group.Name, err))
		members = []*gitlab.GroupMember{}
	}

	utils.PrintInfo(fmt.Sprintf("Found %d GitLab members for group %s", len(members), cleanName))

	// Create organization request
	orgReq := organizationCreateRequest{
		Description: group.Description,
		FullName:    group.FullName,
		Location:    "",
		Username:    cleanName,
		Website:     "",
	}

	// Call Gitea API to create organization
	var result map[string]interface{}
	err = m.giteaClient.Post("/orgs", orgReq, &result)
	if err != nil {
		return fmt.Errorf("failed to create organization %s: %w", cleanName, err)
	}

	utils.PrintInfo(fmt.Sprintf("Group %s imported!", cleanName))

	// Import group members
	if err := m.importGroupMembers(members, cleanName); err != nil {
		utils.PrintWarning(fmt.Sprintf("Error importing members for group %s: %v", cleanName, err))
	}

	return nil
}

// importGroupMembers imports group members to the first team in an organization
func (m *Manager) importGroupMembers(members []*gitlab.GroupMember, orgName string) error {
	// Get existing teams
	var teams []map[string]interface{}
	err := m.giteaClient.Get(fmt.Sprintf("/orgs/%s/teams", orgName), &teams)
	if err != nil {
		return fmt.Errorf("failed to get teams for organization %s: %w", orgName, err)
	}

	if len(teams) == 0 {
		return fmt.Errorf("no teams found for organization %s", orgName)
	}

	firstTeam := teams[0]
	teamID := int(firstTeam["id"].(float64))
	teamName := firstTeam["name"].(string)

	utils.PrintInfo(fmt.Sprintf("Organization teams fetched, importing users to first team: %s", teamName))

	// Add members to the team
	for _, member := range members {
		cleanUsername := utils.NormalizeUsername(member.Username)

		exists, err := m.memberExists(cleanUsername, teamID)
		if err != nil {
			utils.PrintWarning(fmt.Sprintf("Error checking if member %s exists: %v", cleanUsername, err))
			continue
		}

		if exists {
			utils.PrintWarning(fmt.Sprintf("Member %s already exists for team %s, skipping!", member.Username, teamName))
			continue
		}

		// Add member to team
		err = m.giteaClient.Put(fmt.Sprintf("/teams/%d/members/%s", teamID, cleanUsername), nil, nil)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Failed to add member %s to team %s: %v", member.Username, teamName, err))
			continue
		}

		utils.PrintInfo(fmt.Sprintf("Member %s added to team %s!", member.Username, teamName))
	}

	return nil
}

// organizationExists checks if an organization exists in Gitea
func (m *Manager) organizationExists(orgName string) (bool, error) {
	var org map[string]interface{}
	err := m.giteaClient.Get("/orgs/"+orgName, &org)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking if organization exists: %w", err)
	}
	return true, nil
}

// memberExists checks if a user is a member of a team
func (m *Manager) memberExists(username string, teamID int) (bool, error) {
	var members []map[string]interface{}
	err := m.giteaClient.Get(fmt.Sprintf("/teams/%d/members", teamID), &members)
	if err != nil {
		return false, fmt.Errorf("failed to get team members: %w", err)
	}

	for _, member := range members {
		if member["username"].(string) == username {
			return true, nil
		}
	}

	return false, nil
}
