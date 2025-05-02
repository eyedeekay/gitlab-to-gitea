package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// getInitialCommit retrieves the hash of the initial commit by cloning the repository
func (nf *NameFixer) getInitialCommit(owner, repo string) (string, error) {
	// Create a temporary directory
	tempDir, err := ioutil.TempDir("", fmt.Sprintf("%s-%s", owner, repo))
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up when done

	// Get repository clone URL from Gitea API
	var repoInfo map[string]interface{}
	err = nf.client.Get(fmt.Sprintf("repos/%s/%s", owner, repo), &repoInfo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}

	// Try different URL formats that might be available
	var cloneURL string
	for _, urlKey := range []string{"clone_url", "html_url", "ssh_url"} {
		if url, ok := repoInfo[urlKey].(string); ok && url != "" {
			cloneURL = url
			break
		}
	}

	if cloneURL == "" {
		return "", fmt.Errorf("failed to get clone URL for %s/%s", owner, repo)
	}

	// If we have a token and it's an HTTPS URL, inject it for authentication
	if nf.client.GetToken() != "" && strings.HasPrefix(cloneURL, "http") {
		parsedURL, err := url.Parse(cloneURL)
		if err == nil {
			// Add token to URL for authentication
			parsedURL.User = url.UserPassword("oauth2", nf.client.GetToken())
			cloneURL = parsedURL.String()
		}
	}

	utils.PrintInfo(fmt.Sprintf("Cloning repository %s/%s...", owner, repo))

	// Clone with depth 1 first to speed things up if possible
	cmd := exec.Command("git", "clone", "--quiet", "--no-checkout", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	// Create a .git/config if it doesn't exist to avoid git errors
	configPath := filepath.Join(tempDir, ".git", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755)
		ioutil.WriteFile(configPath, []byte("[core]\n\trepositoryformatversion = 0\n"), 0o644)
	}

	// Find the initial commit(s) - the ones with no parents
	cmd = exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	cmd.Dir = tempDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		// Try with '--all' if HEAD doesn't work
		cmd = exec.Command("git", "rev-list", "--max-parents=0", "--all")
		cmd.Dir = tempDir
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git rev-list failed: %w\nOutput: %s", err, string(output))
		}
	}

	// Trim output and get first line (there might be multiple initial commits in case of merged histories)
	commits := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(commits) == 0 {
		return "", fmt.Errorf("no initial commit found")
	}

	// Return the first initial commit hash
	initialCommit := commits[0]
	utils.PrintInfo(fmt.Sprintf("Found initial commit for %s/%s by git clone: %s", owner, repo, initialCommit))

	return initialCommit, nil
}
