// issues.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// issueCreateRequest represents the data needed to create an issue in Gitea
type issueCreateRequest struct {
	Assignee  string   `json:"assignee,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
	Body      string   `json:"body"`
	Closed    bool     `json:"closed"`
	DueOn     string   `json:"due_on,omitempty"`
	Labels    []int    `json:"labels,omitempty"`
	Milestone int      `json:"milestone,omitempty"`
	Title     string   `json:"title"`
}

// importProjectIssues imports project issues to Gitea
func (m *Manager) importProjectIssues(issues []*gitlab.Issue, owner, repo string, projectID int) error {
	// Get existing milestones and labels for reference
	var existingMilestones []map[string]interface{}
	err := m.giteaClient.Get(fmt.Sprintf("/repos/%s/%s/milestones", owner, repo), &existingMilestones)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching milestones: %v", err))
	}

	var existingLabels []map[string]interface{}
	err = m.giteaClient.Get(fmt.Sprintf("/repos/%s/%s/labels", owner, repo), &existingLabels)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching labels: %v", err))
	}

	// Get existing issues to avoid duplicates
	var existingIssues []map[string]interface{}
	err = m.giteaClient.Get(fmt.Sprintf("/repos/%s/%s/issues?state=all&page=-1", owner, repo), &existingIssues)
	if err != nil {
		utils.PrintWarning(fmt.Sprintf("Error fetching existing issues: %v", err))
	}

	for _, issue := range issues {
		// Check if issue already exists
		exists, existingIssue := issueExists(existingIssues, issue.Title)
		if exists {
			utils.PrintWarning(fmt.Sprintf("Issue %s already exists in project %s, importing comments only", issue.Title, repo))

			// Import comments for existing issue
			if existingIssue != nil {
				issueNumber := int(existingIssue["number"].(float64))
				if err := m.importIssueComments(issue, owner, repo, issueNumber, projectID); err != nil {
					utils.PrintWarning(fmt.Sprintf("Error importing comments: %v", err))
				}
			}
			continue
		}

		// Prepare due date
		var dueOn string
		if issue.DueDate != nil {
			dueStr := issue.DueDate.String()
			parsedDate, err := time.Parse("2006-01-02", dueStr)
			if err == nil {
				dueOn = parsedDate.Format(time.RFC3339)
			}
		}

		// Process assignees
		var assignee string
		var assignees []string

		if issue.Assignee != nil {
			assignee = utils.NormalizeUsername(issue.Assignee.Username)
		}

		for _, a := range issue.Assignees {
			assignees = append(assignees, utils.NormalizeUsername(a.Username))
		}

		// Process milestone
		var milestoneID int
		if issue.Milestone != nil {
			for _, m := range existingMilestones {
				if m["title"].(string) == issue.Milestone.Title {
					milestoneID = int(m["id"].(float64))
					break
				}
			}
		}

		// Process labels
		var labelIDs []int
		for _, labelName := range issue.Labels {
			for _, l := range existingLabels {
				if l["name"].(string) == labelName {
					labelIDs = append(labelIDs, int(l["id"].(float64)))
					break
				}
			}
		}

		// Normalize mentions in the description
		description := utils.NormalizeMentions(issue.Description)

		// Create issue
		issueReq := issueCreateRequest{
			Assignee:  assignee,
			Assignees: assignees,
			Body:      description,
			Closed:    issue.State == "closed",
			DueOn:     dueOn,
			Labels:    labelIDs,
			Milestone: milestoneID,
			Title:     issue.Title,
		}

		var result map[string]interface{}
		err = m.giteaClient.Post(fmt.Sprintf("/repos/%s/%s/issues", owner, repo), issueReq, &result)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Issue %s import failed: %v", issue.Title, err))
			continue
		}

		utils.PrintInfo(fmt.Sprintf("Issue %s imported!", issue.Title))

		// Import comments for the new issue
		if result != nil {
			issueNumber := int(result["number"].(float64))
			if err := m.importIssueComments(issue, owner, repo, issueNumber, projectID); err != nil {
				utils.PrintWarning(fmt.Sprintf("Error importing comments: %v", err))
			}
		}
	}

	return nil
}

// issueExists checks if an issue already exists based on title
func issueExists(existingIssues []map[string]interface{}, title string) (bool, map[string]interface{}) {
	for _, issue := range existingIssues {
		if issue["title"].(string) == title {
			return true, issue
		}
	}
	return false, nil
}
