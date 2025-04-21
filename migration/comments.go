// comments.go

// Package migration handles the migration of data from GitLab to Gitea
package migration

import (
	"fmt"

	"github.com/xanzy/go-gitlab"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// commentCreateRequest represents the data needed to create a comment in Gitea
type commentCreateRequest struct {
	Body string `json:"body"`
}

// importIssueComments imports comments from a GitLab issue to a Gitea issue
func (m *Manager) importIssueComments(
	gitlabIssue *gitlab.Issue,
	owner, repo string,
	giteaIssueNumber, projectID int,
) error {
	// Get migration state for comment tracking
	commentKey := fmt.Sprintf("%s/%s/issues/%d", owner, repo, giteaIssueNumber)

	// Get existing comments to avoid duplicates
	var existingComments []map[string]interface{}
	err := m.giteaClient.Get(
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, giteaIssueNumber),
		&existingComments,
	)
	if err != nil {
		return fmt.Errorf("failed to get existing comments: %w", err)
	}

	// Get notes from GitLab
	notes, err := m.gitlabClient.GetIssueNotes(projectID, gitlabIssue.IID)
	if err != nil {
		return fmt.Errorf("failed to get issue notes: %w", err)
	}

	utils.PrintInfo(fmt.Sprintf("Found %d comments for issue #%d", len(notes), giteaIssueNumber))

	importedCount := 0
	for _, note := range notes {
		// Skip system notes
		if note.System {
			continue
		}

		// Skip if note was already imported
		noteID := fmt.Sprintf("%d", note.ID)
		if m.state.HasImportedComment(commentKey, noteID) {
			utils.PrintWarning(fmt.Sprintf("Comment %s already imported, skipping", noteID))
			continue
		}

		// Check for duplicate content
		body := note.Body
		isDuplicate := false
		for _, comment := range existingComments {
			if comment["body"].(string) == body {
				utils.PrintWarning("Comment content already exists, skipping")
				m.state.MarkCommentImported(commentKey, noteID)
				if err := m.state.Save(); err != nil {
					utils.PrintWarning(fmt.Sprintf("Failed to save migration state: %v", err))
				}
				isDuplicate = true
				break
			}
		}

		if isDuplicate {
			continue
		}

		// Normalize mentions in the body
		body = utils.NormalizeMentions(body)

		// Create comment
		commentReq := commentCreateRequest{
			Body: body,
		}

		var result map[string]interface{}
		err = m.giteaClient.Post(
			fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, giteaIssueNumber),
			commentReq,
			&result,
		)
		if err != nil {
			utils.PrintError(fmt.Sprintf("Comment import failed: %v", err))
			continue
		}

		utils.PrintInfo(fmt.Sprintf("Comment for issue #%d imported!", giteaIssueNumber))
		m.state.MarkCommentImported(commentKey, noteID)
		if err := m.state.Save(); err != nil {
			utils.PrintWarning(fmt.Sprintf("Failed to save migration state: %v", err))
		}
		importedCount++
	}

	utils.PrintInfo(fmt.Sprintf("Imported %d new comments for issue #%d", importedCount, giteaIssueNumber))
	return nil
}
