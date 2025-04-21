// labels.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// labelCreateRequest represents the data needed to create a label in Gitea
type labelCreateRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// importProjectLabels imports project labels to Gitea
func (m *Manager) importProjectLabels(labels []*gitlab.Label, owner, repo string) error {
	for _, label := range labels {
		// Check if label already exists
		exists, err := m.labelExists(owner, repo, label.Name)
		if err != nil {
			utils.PrintWarning(fmt.Sprintf("Error checking if label %s exists: %v", label.Name, err))
			continue
		}

		if exists {
			utils.PrintWarning(fmt.Sprintf("Label %s already exists in project %s, skipping!", label.Name, repo))
			continue
		}

		// Create label
		labelReq := labelCreateRequest{
			Name:        label.Name,
			Color:       label.Color,
			Description: label.Description,
		}

		var result map[string]interface{}
		err = m.giteaClient.Post(fmt.Sprintf("/repos/%s/%s/labels", owner, repo), labelReq, &result)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Label %s import failed: %v", label.Name, err))
			continue
		}

		utils.PrintInfo(fmt.Sprintf("Label %s imported!", label.Name))
	}

	return nil
}

// labelExists checks if a label exists in a repository
func (m *Manager) labelExists(owner, repo, labelName string) (bool, error) {
	var labels []map[string]interface{}
	err := m.giteaClient.Get(fmt.Sprintf("/repos/%s/%s/labels", owner, repo), &labels)
	if err != nil {
		return false, fmt.Errorf("failed to get labels: %w", err)
	}

	for _, label := range labels {
		if label["name"].(string) == labelName {
			return true, nil
		}
	}

	return false, nil
}
