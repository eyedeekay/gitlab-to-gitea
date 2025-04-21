// milestones.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// milestoneCreateRequest represents the data needed to create a milestone in Gitea
type milestoneCreateRequest struct {
	Description string `json:"description"`
	DueOn       string `json:"due_on,omitempty"`
	Title       string `json:"title"`
}

// milestoneUpdateRequest represents the data needed to update a milestone in Gitea
type milestoneUpdateRequest struct {
	Description string `json:"description"`
	DueOn       string `json:"due_on,omitempty"`
	State       string `json:"state"`
	Title       string `json:"title"`
}

// importProjectMilestones imports project milestones to Gitea
func (m *Manager) importProjectMilestones(milestones []*gitlab.Milestone, owner, repo string) error {
	for _, milestone := range milestones {
		// Check if milestone already exists
		exists, _, err := m.milestoneExists(owner, repo, milestone.Title)
		if err != nil {
			utils.PrintWarning(fmt.Sprintf("Error checking if milestone %s exists: %v", milestone.Title, err))
			continue
		}

		if exists {
			utils.PrintWarning(fmt.Sprintf("Milestone %s already exists in project %s, skipping!", milestone.Title, repo))
			continue
		}

		// Prepare due date
		var dueOn string
		if milestone.DueDate != nil {
			parsedDate, err := time.Parse("2006-01-02", milestone.DueDate.String())
			if err == nil {
				dueOn = parsedDate.Format(time.RFC3339)
			}
		}

		// Create milestone
		milestoneReq := milestoneCreateRequest{
			Description: milestone.Description,
			DueOn:       dueOn,
			Title:       milestone.Title,
		}

		var result map[string]interface{}
		err = m.giteaClient.Post(fmt.Sprintf("/repos/%s/%s/milestones", owner, repo), milestoneReq, &result)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Milestone %s import failed: %v", milestone.Title, err))
			continue
		}

		utils.PrintInfo(fmt.Sprintf("Milestone %s imported!", milestone.Title))

		// If the milestone is closed, update its state
		if milestone.State == "closed" && result != nil {
			milestoneID := int(result["id"].(float64))

			updateReq := milestoneUpdateRequest{
				Description: milestone.Description,
				DueOn:       dueOn,
				State:       "closed",
				Title:       milestone.Title,
			}

			err = m.giteaClient.Patch(
				fmt.Sprintf("/repos/%s/%s/milestones/%d", owner, repo, milestoneID),
				updateReq,
				nil,
			)
			if err != nil {
				utils.PrintWarning(fmt.Sprintf("Failed to update milestone state: %v", err))
			} else {
				utils.PrintInfo(fmt.Sprintf("Milestone %s state updated to closed", milestone.Title))
			}
		}
	}

	return nil
}

// milestoneExists checks if a milestone exists in a repository
func (m *Manager) milestoneExists(owner, repo, title string) (bool, map[string]interface{}, error) {
	var milestones []map[string]interface{}
	err := m.giteaClient.Get(fmt.Sprintf("/repos/%s/%s/milestones", owner, repo), &milestones)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get milestones: %w", err)
	}

	for _, milestone := range milestones {
		if milestone["title"].(string) == title {
			return true, milestone, nil
		}
	}

	return false, nil, nil
}
